package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// export：简化版，不做端口扫描/探测。
// 仅从系统 ARP/neighbor 表导出（同 WiFi/同局域网里已被系统“学到”的邻居），并输出 IP + 设备名称（best-effort）。

type exportDevice struct {
	IP   string `json:"ip" yaml:"ip"`
	Name string `json:"name" yaml:"name"`
	MAC  string `json:"mac" yaml:"mac"`
}

type exportResult struct {
	Mode      string         `json:"mode" yaml:"mode"` // neighbor-table
	CIDRs     []string       `json:"cidrs" yaml:"cidrs"`
	ScannedAt string         `json:"scannedAt" yaml:"scannedAt"`
	Duration  string         `json:"duration" yaml:"duration"`
	Count     int            `json:"count" yaml:"count"`
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

	format := fs.String("format", "json", "输出格式：json|yaml|cmd")
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
		fmt.Fprintf(os.Stderr, "export: 计算网段失败: %v\n", err)
		return 1
	}
	if len(targetCIDRs) == 0 {
		fmt.Fprintln(os.Stderr, "export: 未找到可用网段（可用 --cidr 指定）")
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
		// Fallback: if we still don't have a human name, use MAC as a stable identifier.
		if strings.TrimSpace(name) == "" {
			if mac := strings.TrimSpace(n.MAC); mac != "" {
				name = mac
			}
		}
		devs = append(devs, exportDevice{IP: ip4.String(), Name: name, MAC: strings.TrimSpace(n.MAC)})
	}
	devs = dedupeDevices(devs)
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
	case "cmd", "command", "cli", "text":
		var sb strings.Builder
		for _, d := range devs {
			sb.WriteString(d.IP)
			sb.WriteByte(' ')
			sb.WriteString(formatCmdField(d.Name))
			sb.WriteByte(' ')
			sb.WriteString(formatCmdField(d.MAC))
			sb.WriteByte('\n')
		}
		b = []byte(sb.String())
	default:
		fmt.Fprintf(os.Stderr, "export: unsupported --format=%q (use json|yaml|cmd)\n", *format)
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
	Net    *net.IPNet
	IfName string
	IfIP   net.IP
	Source string // auto|flag
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

type neighborEntry struct {
	IP   string
	Name string // best-effort, may be "?"
	MAC  string // best-effort
}

var (
	arpDarwinRe = regexp.MustCompile(`^(\S+)\s+\((\d+\.\d+\.\d+\.\d+)\)\s+at\s+(.+?)\s+on\s+(\S+)`)
	ipNeighRe   = regexp.MustCompile(`^(\d+\.\d+\.\d+\.\d+)\s+dev\s+(\S+)\b`)
	macRe       = regexp.MustCompile(`(?i)([0-9a-f]{2}:){5}[0-9a-f]{2}`)
)

func readNeighborTable(ctx context.Context) ([]neighborEntry, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.CommandContext(ctx, "arp", "-a").Output()
		if err != nil {
			return nil, err
		}
		return parseDarwinARP(string(out)), nil
	case "linux":
		out, err := exec.CommandContext(ctx, "sh", "-c", "ip neigh show 2>/dev/null || arp -a 2>/dev/null").Output()
		if err != nil {
			return nil, err
		}
		return parseLinuxNeighbors(string(out)), nil
	default:
		out, err := exec.CommandContext(ctx, "arp", "-a").Output()
		if err != nil {
			return nil, err
		}
		return parseDarwinARP(string(out)), nil
	}
}

func parseDarwinARP(out string) []neighborEntry {
	lines := strings.Split(out, "\n")
	entries := make([]neighborEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := arpDarwinRe.FindStringSubmatch(line)
		if len(m) < 4 {
			continue
		}
		host := strings.TrimSpace(m[1])
		ip := strings.TrimSpace(m[2])
		hw := strings.ToLower(strings.TrimSpace(m[3]))
		// Skip unresolved neighbors like: "at (incomplete)".
		if strings.Contains(hw, "incomplete") {
			continue
		}
		mac := strings.ToLower(macRe.FindString(hw))
		if net.ParseIP(ip).To4() == nil {
			continue
		}
		entries = append(entries, neighborEntry{IP: ip, Name: host, MAC: mac})
	}
	return entries
}

func parseLinuxNeighbors(out string) []neighborEntry {
	lines := strings.Split(out, "\n")
	entries := make([]neighborEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.Contains(upper, " INCOMPLETE") || strings.Contains(upper, " FAILED") {
			continue
		}
		mac := strings.ToLower(macRe.FindString(line))
		if m := ipNeighRe.FindStringSubmatch(line); len(m) >= 2 {
			ip := strings.TrimSpace(m[1])
			if net.ParseIP(ip).To4() == nil {
				continue
			}
			entries = append(entries, neighborEntry{IP: ip, Name: "", MAC: mac})
			continue
		}
		if m := arpDarwinRe.FindStringSubmatch(line); len(m) >= 3 {
			ip := strings.TrimSpace(m[2])
			host := strings.TrimSpace(m[1])
			if net.ParseIP(ip).To4() == nil {
				continue
			}
			entries = append(entries, neighborEntry{IP: ip, Name: host, MAC: mac})
			continue
		}
	}
	return entries
}

func normalizeHostToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "?" {
		return ""
	}
	return strings.TrimSuffix(s, ".")
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
	return strings.TrimSuffix(strings.TrimSpace(names[0]), ".")
}

func dedupeDevices(in []exportDevice) []exportDevice {
	seen := map[string]exportDevice{}
	for _, d := range in {
		ip := strings.TrimSpace(d.IP)
		if ip == "" {
			continue
		}
		cur, ok := seen[ip]
		if !ok {
			seen[ip] = exportDevice{IP: ip, Name: strings.TrimSpace(d.Name), MAC: strings.TrimSpace(d.MAC)}
			continue
		}
		if strings.TrimSpace(cur.Name) == "" && strings.TrimSpace(d.Name) != "" {
			cur.Name = strings.TrimSpace(d.Name)
		}
		if strings.TrimSpace(cur.MAC) == "" && strings.TrimSpace(d.MAC) != "" {
			cur.MAC = strings.TrimSpace(d.MAC)
		}
		seen[ip] = cur
	}
	out := make([]exportDevice, 0, len(seen))
	for _, d := range seen {
		out = append(out, d)
	}
	return out
}

func ipInAnyCIDR(ip net.IP, cidrs []cidrTarget) bool {
	for _, c := range cidrs {
		if c.Net != nil && c.Net.Contains(ip) && !isNetworkOrBroadcastIPv4(ip, c.Net) {
			return true
		}
	}
	return false
}

func isNetworkOrBroadcastIPv4(ip net.IP, n *net.IPNet) bool {
	if n == nil || n.IP == nil || n.Mask == nil {
		return false
	}
	ip4 := ip.To4()
	net4 := n.IP.To4()
	if ip4 == nil || net4 == nil {
		return false
	}
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return false
	}
	// /31,/32 don't have meaningful broadcast semantics for our filtering.
	if ones >= 31 {
		return false
	}
	netU := binary.BigEndian.Uint32(net4.Mask(n.Mask))
	maskU := binary.BigEndian.Uint32(n.Mask)
	bcastU := netU | ^maskU
	u := binary.BigEndian.Uint32(ip4)
	return u == netU || u == bcastU
}

func formatCmdField(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	// If it contains whitespace, quote it so the output stays 3 fields.
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return strconv.Quote(s)
		}
	}
	return s
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
