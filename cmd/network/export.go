//go:build legacy_export
// +build legacy_export

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"sigs.k8s.io/yaml"
)

// NOTE: export 子命令为“简化版邻居表导出”实现（允许破坏性变更）：
// - 不做端口扫描、不做全网段探测
// - 仅从系统 ARP/neighbor 表导出（同 WiFi/同局域网里“已被系统学到”的邻居）
// - 名称优先使用 ARP 输出里自带的 host token，否则可选进行反向 DNS

type exportDevice struct {
	IP   string `json:"ip" yaml:"ip"`
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type exportResult struct {
	Mode      string   `json:"mode" yaml:"mode"` // neighbor-table
	CIDRs     []string `json:"cidrs" yaml:"cidrs"`
	ScannedAt string   `json:"scannedAt" yaml:"scannedAt"`
	Duration  string   `json:"duration" yaml:"duration"`
	Count     int      `json:"count" yaml:"count"`
	Devices   []exportDevice `json:"devices" yaml:"devices"`
}

type multiStringFlag []string

func (m *multiStringFlag) String() string { return strings.Join(*m, ",") }
func (m *multiStringFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	*m = append(*m, v)
	return nil
}

func runExport(args []string) int {
	fs := flag.NewFlagSet("network export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cidrs multiStringFlag
	fs.Var(&cidrs, "cidr", "指定过滤网段（可重复）。不指定则自动从网卡读取，例如: --cidr 192.168.1.0/24")

	timeout := fs.Duration("timeout", 15*time.Second, "导出超时时间（用于限制外部命令/反向解析）")
	resolveDNS := fs.Bool("resolve-dns", true, "是否进行反向解析以补充设备名称")
	dnsTimeout := fs.Duration("dns-timeout", 250*time.Millisecond, "反向解析超时时间（仅在 --resolve-dns=true 生效）")

	format := fs.String("format", "json", "输出格式：json|yaml")
	output := fs.String("output", "", "输出文件路径（默认输出到 stdout）")

	for _, a := range args {
		switch a {
		case "help", "-h", "--help":
			fmt.Fprintln(os.Stdout, "Usage: network export [flags]")
			fs.SetOutput(os.Stdout)
			fs.PrintDefaults()
			return 0
		}
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *timeout <= 0 {
		fmt.Fprintln(os.Stderr, "export: --timeout 必须 > 0")
		return 2
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	targetCIDRs, err := computeTargetCIDRs([]string(cidrs))
	if err != nil {
		fmt.Fprintf(os.Stderr, "export: 计算扫描网段失败: %v\n", err)
		return 1
	}
	if len(targetCIDRs) == 0 {
		fmt.Fprintln(os.Stderr, "export: 未找到可扫描的网段（可用 --cidr 指定）")
		return 1
	}

	neighbors, err := readNeighborTable(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "export: 读取邻居表失败: %v\n", err)
		return 1
	}

	devs := make([]exportDevice, 0, len(neighbors))
	for _, n := range neighbors {
		ip4 := net.ParseIP(n.IP).To4()
		if ip4 == nil {
			continue
		}
		if !ipInAnyCIDR(ip4, targetCIDRs) {
			continue
		}
		name := normalizeHostToken(n.Name)
		if name == "" && *resolveDNS {
			name = reverseLookup(ctx, ip4.String(), *dnsTimeout)
		}
		devs = append(devs, exportDevice{IP: ip4.String(), Name: name})
	}
	devs = dedupeDevices(devs)

	// sort by IP for stable output
	sort.Slice(devs, func(i, j int) bool { return ipLess(devs[i].IP, devs[j].IP) })

	cidrStrings := make([]string, 0, len(targetCIDRs))
	for _, c := range targetCIDRs {
		if c.Net != nil {
			cidrStrings = append(cidrStrings, c.Net.String())
		}
	}

	res := exportResult{
		Mode:      "neighbor-table",
		CIDRs:     cidrStrings,
		ScannedAt: start.Format(time.RFC3339Nano),
		Duration:  time.Since(start).String(),
		Count:     len(devs),
		Devices:   devs,
	}

	var b []byte
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json", "":
		b, err = json.MarshalIndent(res, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "export: json marshal failed: %v\n", err)
			return 1
		}
		b = append(b, '\n')
	case "yaml", "yml":
		b, err = yaml.Marshal(res)
		if err != nil {
			fmt.Fprintf(os.Stderr, "export: yaml marshal failed: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "export: unsupported --format=%q (use json|yaml)\n", *format)
		return 2
	}

	if strings.TrimSpace(*output) == "" {
		_, _ = os.Stdout.Write(b)
		return 0
	}
	if err := os.WriteFile(*output, b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "export: write output failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "export ok: %s (%d devices)\n", *output, len(devs))
	return 0
}

type cidrTarget struct {
	Net      *net.IPNet
	IfName   string
	IfIP     net.IP
	Source   string // auto|flag
	Priority int
}

func computeTargetCIDRs(userCIDRs []string) ([]cidrTarget, error) {
	if len(userCIDRs) > 0 {
		var out []cidrTarget
		for _, s := range userCIDRs {
			_, n, err := net.ParseCIDR(strings.TrimSpace(s))
			if err != nil {
				return nil, err
			}
			if n == nil || n.IP == nil || n.IP.To4() == nil {
				continue
			}
			out = append(out, cidrTarget{Net: n, Source: "flag"})
		}
		return dedupeCIDRs(out), nil
	}
	auto, err := localIPv4CIDRs()
	if err != nil {
		return nil, err
	}
	return dedupeCIDRs(auto), nil
}

func dedupeCIDRs(in []cidrTarget) []cidrTarget {
	seen := map[string]struct{}{}
	out := make([]cidrTarget, 0, len(in))
	for _, c := range in {
		if c.Net == nil {
			continue
		}
		k := c.Net.String()
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, c)
	}
	return out
}

func localIPv4CIDRs() ([]cidrTarget, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := []cidrTarget{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet == nil || ipnet.IP == nil {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}
			// Normalize network IP.
			nip := ip4.Mask(ipnet.Mask)
			n := &net.IPNet{IP: nip, Mask: ipnet.Mask}
			out = append(out, cidrTarget{
				Net:    n,
				IfName: iface.Name,
				IfIP:   ip4,
				Source: "auto",
			})
		}
	}
	return out, nil
}

type scanOpts struct {
	ProbePorts   []int
	ProbeTimeout time.Duration
	Concurrency  int
	Delay        time.Duration

	ResolveDNS bool
	DNSTimeout time.Duration
	ResolveMAC bool
}

func scanLAN(ctx context.Context, targets []cidrTarget, opt scanOpts) []exportDevice {
	type task struct {
		ip   net.IP
		cidr cidrTarget
	}

	tasks := make(chan task)
	results := make(chan exportDevice, 128)

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for t := range tasks {
			select {
			case <-ctx.Done():
				return
			default:
			}

			alive, prs := probeAliveTCP(t.ip, opt.ProbePorts, opt.ProbeTimeout)
			if !alive {
				continue
			}

			d := exportDevice{
				IP:         t.ip.String(),
				CIDR:       safeCIDRString(t.cidr.Net),
				Interface:  t.cidr.IfName,
				Alive:      true,
				ProbePorts: prs,
			}

			if opt.ResolveDNS {
				d.Hostname = reverseLookup(ctx, d.IP, opt.DNSTimeout)
			}
			if opt.ResolveMAC {
				d.MAC = lookupMAC(ctx, d.IP)
			}

			select {
			case results <- d:
			case <-ctx.Done():
				return
			}
		}
	}

	if opt.Concurrency <= 0 {
		opt.Concurrency = 1
	}
	wg.Add(opt.Concurrency)
	for i := 0; i < opt.Concurrency; i++ {
		go worker()
	}

	// Feed tasks (slow mode supported via opt.Delay).
	go func() {
		defer close(tasks)
		for _, c := range targets {
			iterIPv4Hosts(ctx, c.Net, func(ip net.IP) bool {
				select {
				case <-ctx.Done():
					return false
				default:
				}
				if opt.Delay > 0 {
					select {
					case <-time.After(opt.Delay):
					case <-ctx.Done():
						return false
					}
				}
				select {
				case tasks <- task{ip: ip, cidr: c}:
					return true
				case <-ctx.Done():
					return false
				}
			})
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	out := []exportDevice{}
	for d := range results {
		out = append(out, d)
	}
	return dedupeByIP(out)
}

func dedupeByIP(in []exportDevice) []exportDevice {
	seen := map[string]struct{}{}
	out := make([]exportDevice, 0, len(in))
	for _, d := range in {
		if d.IP == "" {
			continue
		}
		if _, ok := seen[d.IP]; ok {
			continue
		}
		seen[d.IP] = struct{}{}
		out = append(out, d)
	}
	return out
}

func fetchHTTPInfoLAN(ctx context.Context, devs []exportDevice, port int, timeout time.Duration) {
	if port <= 0 || port > 65535 {
		return
	}
	if timeout <= 0 {
		timeout = 700 * time.Millisecond
	}
	c := &http.Client{Timeout: timeout}

	for i := range devs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !devs[i].Alive || strings.TrimSpace(devs[i].IP) == "" {
			continue
		}
		u := "http://" + net.JoinHostPort(devs[i].IP, strconv.Itoa(port)) + "/info"
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		resp, err := c.Do(req)
		if err != nil {
			devs[i].FetchError = err.Error()
			continue
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				devs[i].FetchError = fmt.Sprintf("status=%d", resp.StatusCode)
				return
			}
			var m map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
				devs[i].FetchError = err.Error()
				return
			}
			devs[i].HTTPInfo = m
		}()
	}
}

func parsePortsCSV(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		if n <= 0 || n > 65535 {
			return nil, fmt.Errorf("invalid port: %d", n)
		}
		out = append(out, n)
	}
	out = uniqueInts(out)
	sort.Ints(out)
	return out, nil
}

func uniqueInts(in []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, n := range in {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func probeAliveTCP(ip net.IP, ports []int, timeout time.Duration) (bool, []portResult) {
	ip4 := ip.To4()
	if ip4 == nil {
		return false, nil
	}
	if timeout <= 0 {
		timeout = 350 * time.Millisecond
	}

	alive := false
	results := make([]portResult, 0, len(ports))
	for _, port := range ports {
		start := time.Now()
		addr := net.JoinHostPort(ip4.String(), strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		dur := time.Since(start)
		if err == nil {
			_ = conn.Close()
			results = append(results, portResult{Port: port, Outcome: "open", Duration: dur.String()})
			alive = true
			continue
		}

		outcome := classifyDialError(err)
		results = append(results, portResult{Port: port, Outcome: outcome, Duration: dur.String()})
		if outcome == "refused" || outcome == "open" {
			alive = true
		}
	}
	return alive, results
}

func classifyDialError(err error) string {
	// Treat "connection refused" as alive.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "refused"
	}
	if errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH) {
		return "unreachable"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "timeout"
	}
	// Some platforms don't wrap syscall neatly.
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") {
		return "refused"
	}
	if strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "timeout") {
		return "timeout"
	}
	return "error"
}

func reverseLookup(ctx context.Context, ip string, timeout time.Duration) string {
	if strings.TrimSpace(ip) == "" {
		return ""
	}
	if timeout <= 0 {
		timeout = 250 * time.Millisecond
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r := net.Resolver{}
	names, err := r.LookupAddr(cctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}
	// pick first, trim trailing dot
	return strings.TrimSuffix(strings.TrimSpace(names[0]), ".")
}

var macRe = regexp.MustCompile(`(?i)([0-9a-f]{2}:){5}[0-9a-f]{2}`)

func lookupMAC(ctx context.Context, ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}

	// Best-effort, OS dependent.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "arp", "-n", ip)
	default:
		// Linux common: `ip neigh show <ip>`
		cmd = exec.CommandContext(ctx, "sh", "-c", "ip neigh show "+shellQuote(ip)+" 2>/dev/null || arp -n "+shellQuote(ip)+" 2>/dev/null")
	}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	m := macRe.FindString(string(out))
	return strings.ToLower(m)
}

func shellQuote(s string) string {
	// minimal single-quote escape for sh -c
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func safeCIDRString(n *net.IPNet) string {
	if n == nil {
		return ""
	}
	return n.String()
}

func estimateIPv4Hosts(n *net.IPNet) int {
	if n == nil || n.IP == nil || n.IP.To4() == nil {
		return 0
	}
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return 0
	}
	hostBits := 32 - ones
	if hostBits <= 0 {
		return 0
	}
	// rough guard: if /31 or /32 => 0 usable hosts in traditional sense
	if hostBits == 1 || hostBits == 0 {
		return 0
	}
	total := 1 << hostBits
	usable := total - 2
	if usable < 0 {
		return 0
	}
	return usable
}

func iterIPv4Hosts(ctx context.Context, n *net.IPNet, fn func(net.IP) bool) {
	if n == nil || n.IP == nil {
		return
	}
	ip4 := n.IP.To4()
	if ip4 == nil {
		return
	}
	ones, bits := n.Mask.Size()
	if bits != 32 || ones < 0 || ones > 32 {
		return
	}
	// skip tiny networks
	if ones >= 31 {
		return
	}
	netBase := binary.BigEndian.Uint32(ip4.Mask(n.Mask))
	hostCount := uint32(1) << uint32(32-ones)
	start := netBase + 1
	end := netBase + hostCount - 2
	for v := start; v <= end; v++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		if !fn(net.IP(b)) {
			return
		}
		if v == end {
			return
		}
	}
}

func ipLess(a, b string) bool {
	ipa := net.ParseIP(strings.TrimSpace(a)).To4()
	ipb := net.ParseIP(strings.TrimSpace(b)).To4()
	if ipa == nil && ipb == nil {
		return a < b
	}
	if ipa == nil {
		return false
	}
	if ipb == nil {
		return true
	}
	ua := binary.BigEndian.Uint32(ipa)
	ub := binary.BigEndian.Uint32(ipb)
	return ua < ub
}
