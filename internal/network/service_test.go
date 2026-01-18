package network

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseTXT(t *testing.T) {
	in := []string{
		" node = n1 ",
		"port= 7946",
		"pid=123",
		"nope",
		"=bad",
		"",
		"  ",
		"k=v=keep-rest",
		"empty=",
	}

	got := parseTXT(in)

	if got["node"] != "n1" {
		t.Fatalf("node mismatch: got=%q", got["node"])
	}
	if got["port"] != "7946" {
		t.Fatalf("port mismatch: got=%q", got["port"])
	}
	if got["pid"] != "123" {
		t.Fatalf("pid mismatch: got=%q", got["pid"])
	}
	if got["k"] != "v=keep-rest" {
		t.Fatalf("k mismatch: got=%q", got["k"])
	}
	if got["empty"] != "" {
		t.Fatalf("empty mismatch: got=%q", got["empty"])
	}
	if _, ok := got[""]; ok {
		t.Fatalf("expected empty key to be ignored, got=%v", got)
	}
}

func TestBuildAddresses_DedupeAndFilter(t *testing.T) {
	name := "node-1"
	addrs := []net.IP{
		net.ParseIP("10.0.0.1"),
		net.ParseIP("10.0.0.1"),   // duplicate
		net.ParseIP("fe80::1"),    // link-local, should be filtered
		net.ParseIP("::1"),        // loopback, should be filtered
		net.ParseIP("2001:db8::1"), // global v6, keep
		nil,
	}

	got := buildAddresses(name, addrs)

	seen := map[string]bool{}
	for _, a := range got {
		seen[string(a.Type)+"|"+a.Address] = true
	}

	if !seen[string(corev1.NodeHostName)+"|"+name] {
		t.Fatalf("expected hostname address %q, got=%v", name, got)
	}
	if !seen[string(corev1.NodeInternalIP)+"|10.0.0.1"] {
		t.Fatalf("expected v4 internal ip, got=%v", got)
	}
	if !seen[string(corev1.NodeInternalIP)+"|2001:db8::1"] {
		t.Fatalf("expected global v6 internal ip, got=%v", got)
	}
	if seen[string(corev1.NodeInternalIP)+"|fe80::1"] {
		t.Fatalf("did not expect link-local v6, got=%v", got)
	}
	if seen[string(corev1.NodeInternalIP)+"|::1"] {
		t.Fatalf("did not expect loopback v6, got=%v", got)
	}
}

func TestSetNodeReadyCondition_NoStatusChangeKeepsTransition(t *testing.T) {
	old := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	conds := []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: old,
			LastHeartbeatTime:  old,
			Reason:             "Old",
			Message:            "Old",
		},
	}

	out := setNodeReadyCondition(conds, true, "NewReason", "NewMessage")

	if len(out) != 1 {
		t.Fatalf("unexpected conditions length: %d", len(out))
	}
	if out[0].Type != corev1.NodeReady {
		t.Fatalf("unexpected condition type: %v", out[0].Type)
	}
	if out[0].Status != corev1.ConditionTrue {
		t.Fatalf("unexpected status: %v", out[0].Status)
	}
	// Status didn't change, so transition time must remain as-is.
	if !out[0].LastTransitionTime.Equal(&old) {
		t.Fatalf("expected transition time to stay %v, got %v", old, out[0].LastTransitionTime)
	}
	// Heartbeat time should be refreshed.
	if !out[0].LastHeartbeatTime.After(old.Time) {
		t.Fatalf("expected heartbeat time after %v, got %v", old, out[0].LastHeartbeatTime)
	}
	if out[0].Reason != "NewReason" || out[0].Message != "NewMessage" {
		t.Fatalf("expected reason/message updated, got reason=%q message=%q", out[0].Reason, out[0].Message)
	}
}

func TestSetNodeReadyCondition_StatusChangeUpdatesTransition(t *testing.T) {
	old := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	conds := []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: old,
			LastHeartbeatTime:  old,
		},
	}

	out := setNodeReadyCondition(conds, true, "Changed", "Changed")

	if out[0].Status != corev1.ConditionTrue {
		t.Fatalf("unexpected status: %v", out[0].Status)
	}
	if !out[0].LastTransitionTime.After(old.Time) {
		t.Fatalf("expected transition time after %v, got %v", old, out[0].LastTransitionTime)
	}
}

func TestHealthMux(t *testing.T) {
	svc := &Service{
		s: Settings{
			NodeName:     "node-x",
			Service:      "_k3._tcp",
			Domain:       "local.",
			RegisterSelf: true,
		},
	}

	h := svc.healthMux(1234)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/healthz status: %d", rr.Code)
	}
	if rr.Body.String() != "ok\n" {
		t.Fatalf("/healthz body: %q", rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/info", nil)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("/info status: %d", rr2.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid /info json: %v; body=%q", err, rr2.Body.String())
	}
	if payload["node"] != "node-x" {
		t.Fatalf("node mismatch: %v", payload["node"])
	}
	if int(payload["port"].(float64)) != 1234 {
		t.Fatalf("port mismatch: %v", payload["port"])
	}
	if int(payload["pid"].(float64)) != os.Getpid() {
		t.Fatalf("pid mismatch: %v", payload["pid"])
	}
	mdns, ok := payload["mdns"].(map[string]any)
	if !ok {
		t.Fatalf("mdns not an object: %T", payload["mdns"])
	}
	if mdns["service"] != "_k3._tcp" || mdns["domain"] != "local." {
		t.Fatalf("mdns mismatch: %v", mdns)
	}
}

func TestProbePeer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Accept one connection so the dial completes cleanly.
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := ln.Accept()
		if err == nil {
			_ = c.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	if ok := probePeer([]net.IP{net.ParseIP("127.0.0.1")}, port, 500*time.Millisecond); !ok {
		t.Fatalf("expected probePeer to succeed")
	}

	<-done

	if ok := probePeer([]net.IP{net.ParseIP("127.0.0.1")}, 0, 100*time.Millisecond); ok {
		t.Fatalf("expected probePeer to fail when port<=0")
	}
}

