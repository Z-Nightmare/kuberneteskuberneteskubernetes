package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/internal/core/logprovider"
	"github.com/Z-Nightmare/kuberneteskuberneteskubernetes/pkg/storage"
	"github.com/grandcat/zeroconf"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type Settings struct {
	// ListenAddr is the TCP address for the local health server, e.g. ":7946".
	ListenAddr string
	// Service is the mDNS service name, e.g. "_k3._tcp".
	Service string
	// Domain is usually "local." for mDNS.
	Domain string
	// PeerTTL controls how long a peer stays Ready after last seen.
	PeerTTL time.Duration
	// ProbeInterval controls how often we probe known peers.
	ProbeInterval time.Duration
	// ProbeTimeout controls the per-peer TCP probe timeout.
	ProbeTimeout time.Duration
	// SelfHeartbeatInterval controls how often we refresh self node info (when RegisterSelf is true).
	SelfHeartbeatInterval time.Duration
	// NodeName is used as the mDNS instance name.
	NodeName string
	// RegisterSelf controls whether to also upsert current node into Store.
	RegisterSelf bool
}

type Service struct {
	store  storage.Store
	logger logprovider.Logger
	s      Settings

	mu       sync.Mutex
	peers    map[string]peerState
	cancelBg context.CancelFunc

	httpServer *http.Server
	mdnsServer *zeroconf.Server
}

type peerState struct {
	lastSeen time.Time
	addrs    []net.IP
	port     int
	txt      map[string]string
}

func NewService(store storage.Store, logger logprovider.Logger, s Settings) *Service {
	if strings.TrimSpace(s.ListenAddr) == "" {
		s.ListenAddr = ":7946"
	}
	if strings.TrimSpace(s.Service) == "" {
		s.Service = "_k3._tcp"
	}
	if strings.TrimSpace(s.Domain) == "" {
		s.Domain = "local."
	}
	if s.PeerTTL <= 0 {
		s.PeerTTL = 90 * time.Second
	}
	if s.ProbeInterval <= 0 {
		s.ProbeInterval = 15 * time.Second
	}
	if s.ProbeTimeout <= 0 {
		s.ProbeTimeout = 700 * time.Millisecond
	}
	if s.SelfHeartbeatInterval <= 0 {
		s.SelfHeartbeatInterval = 30 * time.Second
	}
	if strings.TrimSpace(s.NodeName) == "" {
		s.NodeName = defaultNodeName(logger)
	}

	return &Service{
		store:  store,
		logger: logger,
		s:      s,
		peers:  make(map[string]peerState),
	}
}

func (svc *Service) Start(ctx context.Context) error {
	bgCtx, cancel := context.WithCancel(context.Background())
	svc.cancelBg = cancel

	ln, port, err := listenTCP(svc.s.ListenAddr)
	if err != nil {
		return err
	}

	svc.httpServer = &http.Server{
		Handler: svc.healthMux(port),
	}
	go func() {
		if err := svc.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			svc.logger.Warnf("network: health server stopped: %v", err)
		}
	}()

	txt := []string{
		"node=" + svc.s.NodeName,
		"port=" + strconv.Itoa(port),
		"pid=" + strconv.Itoa(os.Getpid()),
	}
	mdns, err := zeroconf.Register(svc.s.NodeName, svc.s.Service, svc.s.Domain, port, txt, nil)
	if err != nil {
		_ = svc.httpServer.Shutdown(context.Background())
		return fmt.Errorf("network: mdns register failed: %w", err)
	}
	svc.mdnsServer = mdns

	if svc.s.RegisterSelf {
		addrs := localIPv4s()
		_ = svc.upsertManagedNode(svc.s.NodeName, addrs, port, map[string]string{
			"source": "self",
		}, true)
		go svc.selfHeartbeatLoop(bgCtx, port)
	}

	if err := svc.startBrowse(bgCtx); err != nil {
		svc.logger.Warnf("network: browse start failed: %v", err)
	}

	go svc.reapLoop(bgCtx)

	_ = ctx // fx OnStart ctx is short-lived; use bgCtx instead.
	svc.logger.Infof("network: started (listen=%s, mdns=%s/%s)", svc.s.ListenAddr, svc.s.Service, svc.s.Domain)
	return nil
}

func (svc *Service) Stop(ctx context.Context) error {
	if svc.cancelBg != nil {
		svc.cancelBg()
	}
	if svc.mdnsServer != nil {
		svc.mdnsServer.Shutdown()
	}
	if svc.httpServer != nil {
		_ = svc.httpServer.Shutdown(ctx)
	}
	svc.logger.Info("network: stopped")
	return nil
}

func (svc *Service) healthMux(port int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"node":   svc.s.NodeName,
			"port":   port,
			"pid":    os.Getpid(),
			"addrs":  localIPv4sAsStrings(),
			"ts":     time.Now().Format(time.RFC3339Nano),
			"mdns":   map[string]string{"service": svc.s.Service, "domain": svc.s.Domain},
			"selfUp": svc.s.RegisterSelf,
		})
	})
	return mux
}

func (svc *Service) startBrowse(ctx context.Context) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-entries:
				if !ok {
					return
				}
				svc.onEntry(e)
			}
		}
	}()

	return resolver.Browse(ctx, svc.s.Service, svc.s.Domain, entries)
}

func (svc *Service) onEntry(e *zeroconf.ServiceEntry) {
	if e == nil {
		return
	}

	txt := parseTXT(e.Text)
	name := strings.TrimSpace(txt["node"])
	if name == "" {
		name = strings.TrimSpace(e.Instance)
	}
	if name == "" || name == svc.s.NodeName {
		return
	}

	port := e.Port
	if pStr := strings.TrimSpace(txt["port"]); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			port = p
		}
	}

	addrs := make([]net.IP, 0, len(e.AddrIPv4)+len(e.AddrIPv6))
	addrs = append(addrs, e.AddrIPv4...)
	addrs = append(addrs, e.AddrIPv6...)

	if len(addrs) == 0 {
		return
	}

	svc.mu.Lock()
	svc.peers[name] = peerState{
		lastSeen: time.Now(),
		addrs:    addrs,
		port:     port,
		txt:      txt,
	}
	svc.mu.Unlock()

	// Only create/update nodes we "own" (managed=true). If a real controller has
	// already reported this node, we won't fight it.
	if err := svc.upsertManagedNode(name, addrs, port, txt, true); err != nil {
		svc.logger.Debugf("network: upsert peer node failed: %s: %v", name, err)
	}
}

func (svc *Service) reapLoop(ctx context.Context) {
	t := time.NewTicker(svc.s.ProbeInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			// Take a snapshot so we don't hold the lock during network I/O.
			peers := map[string]peerState{}
			svc.mu.Lock()
			for name, st := range svc.peers {
				peers[name] = st
			}
			svc.mu.Unlock()

			for name, st := range peers {
				alive := probePeer(st.addrs, st.port, svc.s.ProbeTimeout)
				if alive {
					svc.mu.Lock()
					cur := svc.peers[name]
					cur.lastSeen = now
					svc.peers[name] = cur
					svc.mu.Unlock()

					_ = svc.markManagedNodeReady(name, true, "PeerAlive", "tcp probe ok")
					continue
				}

				if now.Sub(st.lastSeen) > svc.s.PeerTTL {
					// Mark stale peers NotReady only if they are managed by this module.
					if err := svc.markManagedNodeReady(name, false, "PeerExpired", "peer ttl exceeded"); err != nil {
						svc.logger.Debugf("network: mark peer not ready failed: %s: %v", name, err)
					}
				}
			}
		}
	}
}

func (svc *Service) selfHeartbeatLoop(ctx context.Context, port int) {
	t := time.NewTicker(svc.s.SelfHeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			addrs := localIPv4s()
			_ = svc.upsertManagedNode(svc.s.NodeName, addrs, port, map[string]string{
				"source": "self",
			}, true)
		}
	}
}

func (svc *Service) upsertManagedNode(name string, addrs []net.IP, port int, txt map[string]string, ready bool) error {
	nodeGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}

	existingObj, err := svc.store.Get(nodeGVK, "", name)
	if err == nil {
		if existing, ok := existingObj.(*corev1.Node); ok {
			// If it's not managed by us, don't touch it.
			if existing.Labels == nil || existing.Labels["k3.network/managed"] != "true" {
				return nil
			}
			updated := buildNodeFromExisting(existing, name, addrs, port, txt, ready)
			return svc.store.Update(nodeGVK, updated)
		}
		// Unknown type in store; avoid overwriting.
		return nil
	}

	// Create new managed node.
	node := buildNode(name, addrs, port, txt, ready)
	return svc.store.Create(nodeGVK, node)
}

func (svc *Service) markManagedNodeReady(name string, ready bool, reason, message string) error {
	nodeGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
	obj, err := svc.store.Get(nodeGVK, "", name)
	if err != nil {
		return err
	}
	existing, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}
	if existing.Labels == nil || existing.Labels["k3.network/managed"] != "true" {
		return nil
	}

	n := existing.DeepCopy()
	n.Status.Phase = corev1.NodeRunning
	n.Status.Conditions = setNodeReadyCondition(n.Status.Conditions, ready, reason, message)
	if n.Annotations == nil {
		n.Annotations = map[string]string{}
	}
	n.Annotations["k3.network/lastSeen"] = time.Now().Format(time.RFC3339Nano)
	return svc.store.Update(nodeGVK, n)
}

func buildNode(name string, addrs []net.IP, port int, txt map[string]string, ready bool) *corev1.Node {
	n := &corev1.Node{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Node"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.Now(),
			Labels: map[string]string{
				"kubernetes.io/hostname": name,
				"k3.network/managed":     "true",
				"k3.network/discovered":  "true",
			},
			Annotations: map[string]string{},
		},
		Status: corev1.NodeStatus{
			Phase: corev1.NodeRunning,
		},
	}

	n.Status.Addresses = buildAddresses(name, addrs)
	n.Status.Conditions = setNodeReadyCondition(nil, ready, "LANDiscovery", "discovered via mDNS")
	n.Annotations["k3.network/lastSeen"] = time.Now().Format(time.RFC3339Nano)
	n.Annotations["k3.network/port"] = strconv.Itoa(port)
	if v := strings.TrimSpace(txt["pid"]); v != "" {
		n.Annotations["k3.network/pid"] = v
	}

	if n.UID == "" {
		n.UID = types.UID("node-" + name)
	}
	return n
}

func buildNodeFromExisting(existing *corev1.Node, name string, addrs []net.IP, port int, txt map[string]string, ready bool) *corev1.Node {
	n := existing.DeepCopy()
	n.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Node"}
	if n.Labels == nil {
		n.Labels = map[string]string{}
	}
	n.Labels["k3.network/managed"] = "true"
	n.Labels["k3.network/discovered"] = "true"
	n.Labels["kubernetes.io/hostname"] = name

	n.Status.Phase = corev1.NodeRunning
	n.Status.Addresses = buildAddresses(name, addrs)
	n.Status.Conditions = setNodeReadyCondition(n.Status.Conditions, ready, "LANDiscovery", "discovered via mDNS")

	if n.Annotations == nil {
		n.Annotations = map[string]string{}
	}
	n.Annotations["k3.network/lastSeen"] = time.Now().Format(time.RFC3339Nano)
	n.Annotations["k3.network/port"] = strconv.Itoa(port)
	if v := strings.TrimSpace(txt["pid"]); v != "" {
		n.Annotations["k3.network/pid"] = v
	}

	// Keep UID/CreationTimestamp from existing.
	return n
}

func buildAddresses(name string, addrs []net.IP) []corev1.NodeAddress {
	out := []corev1.NodeAddress{
		{Type: corev1.NodeHostName, Address: name},
	}
	for _, ip := range addrs {
		if ip == nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			out = append(out, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: v4.String()})
			continue
		}
		// Only keep global IPv6.
		if ip.To16() != nil && !ip.IsLinkLocalUnicast() && !ip.IsLoopback() {
			out = append(out, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip.String()})
		}
	}
	return dedupeNodeAddresses(out)
}

func setNodeReadyCondition(conds []corev1.NodeCondition, ready bool, reason, message string) []corev1.NodeCondition {
	now := metav1.Now()
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}

	found := false
	for i := range conds {
		if conds[i].Type != corev1.NodeReady {
			continue
		}
		found = true
		changed := conds[i].Status != status
		conds[i].Status = status
		conds[i].LastHeartbeatTime = now
		conds[i].Reason = reason
		conds[i].Message = message
		if conds[i].LastTransitionTime.IsZero() || changed {
			conds[i].LastTransitionTime = now
		}
	}
	if !found {
		conds = append(conds, corev1.NodeCondition{
			Type:               corev1.NodeReady,
			Status:             status,
			LastHeartbeatTime:  now,
			LastTransitionTime: now,
			Reason:             reason,
			Message:            message,
		})
	}
	return conds
}

func dedupeNodeAddresses(in []corev1.NodeAddress) []corev1.NodeAddress {
	seen := make(map[string]struct{}, len(in))
	out := make([]corev1.NodeAddress, 0, len(in))
	for _, a := range in {
		k := string(a.Type) + "|" + a.Address
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, a)
	}
	return out
}

func probePeer(addrs []net.IP, port int, timeout time.Duration) bool {
	if port <= 0 {
		return false
	}
	var candidates []net.IP
	for _, ip := range addrs {
		if ip == nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			candidates = append(candidates, v4)
		}
	}
	for _, ip := range addrs {
		if ip == nil || ip.To4() != nil {
			continue
		}
		if ip.To16() != nil && !ip.IsLinkLocalUnicast() && !ip.IsLoopback() {
			candidates = append(candidates, ip)
		}
	}
	for _, ip := range candidates {
		addr := net.JoinHostPort(ip.String(), strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

func parseTXT(lines []string) map[string]string {
	m := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		m[k] = v
	}
	return m
}

func defaultNodeName(logger logprovider.Logger) string {
	if v := strings.TrimSpace(os.Getenv("NODE_NAME")); v != "" {
		return v
	}
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		logger.Warn("network: cannot get hostname, fallback node-1")
		return "node-1"
	}
	return h
}

func listenTCP(addr string) (net.Listener, int, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, 0, fmt.Errorf("network: unexpected listener addr: %T", ln.Addr())
	}
	return ln, tcpAddr.Port, nil
}

func localIPv4s() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var ips []net.IP
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
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ips = append(ips, ip)
		}
	}
	return ips
}

func localIPv4sAsStrings() []string {
	ips := localIPv4s()
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

