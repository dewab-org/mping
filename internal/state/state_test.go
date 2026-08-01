package state

import (
	"testing"
	"time"

	"mping/internal/ping"
)

func snapshotKeys(s *SharedState) []string {
	snap := s.Snapshot()
	keys := make([]string, len(snap))
	for i, h := range snap {
		keys[i] = h.Key
	}
	return keys
}

func TestAddHostSpec(t *testing.T) {
	s := NewSharedState(0)
	if err := s.AddHost("alpha", time.Second, time.Second); err != nil {
		t.Fatalf("AddHost: %v", err)
	}
	if err := s.AddHost("alpha", time.Second, time.Second); err != nil {
		t.Fatalf("duplicate AddHost should be a no-op, got %v", err)
	}
	if s.Count() != 1 {
		t.Errorf("Count = %d, want 1", s.Count())
	}
	if err := s.AddHost("  ", time.Second, time.Second); err == nil {
		t.Error("blank host should be rejected")
	}
	if _, _, _, _, _, ok := s.HostConfig("alpha"); !ok {
		t.Error("HostConfig should find alpha")
	}
}

func TestAddHostSpecDefaultsProtocol(t *testing.T) {
	s := NewSharedState(0)
	if err := s.AddHostSpec(HostSpec{Key: "h", Name: "h"}, time.Second, time.Second); err != nil {
		t.Fatalf("AddHostSpec: %v", err)
	}
	if _, protocol, _, _, _, _ := s.HostConfig("h"); protocol != "icmp" {
		t.Errorf("protocol = %q, want icmp", protocol)
	}
}

func TestAddHostMaxHosts(t *testing.T) {
	s := NewSharedState(1)
	if err := s.AddHost("one", time.Second, time.Second); err != nil {
		t.Fatalf("first AddHost: %v", err)
	}
	if err := s.AddHost("two", time.Second, time.Second); err == nil {
		t.Error("second AddHost should hit the max_hosts limit")
	}
}

func TestApplyResult(t *testing.T) {
	s := NewSharedState(0)
	if err := s.AddHost("h", time.Second, time.Second); err != nil {
		t.Fatalf("AddHost: %v", err)
	}

	if s.ApplyResult("missing", ping.PingResult{}, nil) {
		t.Error("ApplyResult for unknown key should return false")
	}

	ok := s.ApplyResult("h", ping.PingResult{
		Success:      true,
		RTT:          25 * time.Millisecond,
		ResolvedIP:   "10.0.0.1",
		ResolvedName: "h.example",
		Status:       "OK",
	}, nil)
	if !ok {
		t.Fatal("ApplyResult returned false for known key")
	}
	snap := s.Snapshot()[0]
	if snap.SuccessCount != 1 || snap.FailureCount != 0 {
		t.Errorf("counts = %d/%d, want 1/0", snap.SuccessCount, snap.FailureCount)
	}
	if snap.IP != "10.0.0.1" || snap.ResolvedName != "h.example" {
		t.Errorf("IP/name = %q/%q", snap.IP, snap.ResolvedName)
	}
	if snap.LastOK.IsZero() {
		t.Error("LastOK should be set after a success")
	}
	if snap.LastRTT != 25*time.Millisecond {
		t.Errorf("LastRTT = %v, want 25ms", snap.LastRTT)
	}

	s.ApplyResult("h", ping.PingResult{Success: false, RawError: "timeout"}, nil)
	snap = s.Snapshot()[0]
	if snap.SuccessCount != 1 || snap.FailureCount != 1 {
		t.Errorf("counts = %d/%d, want 1/1", snap.SuccessCount, snap.FailureCount)
	}
	if snap.LastError != "timeout" {
		t.Errorf("LastError = %q, want timeout", snap.LastError)
	}

	s.ApplyResult("h", ping.PingResult{Success: true}, nil)
	if got := s.Snapshot()[0].LastError; got != "" {
		t.Errorf("LastError = %q, want cleared after success", got)
	}
}

func TestDeleteHost(t *testing.T) {
	s := NewSharedState(0)
	for _, h := range []string{"a", "b", "c"} {
		if err := s.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost %s: %v", h, err)
		}
	}
	s.DeleteHost("b")
	s.DeleteHost("missing") // no-op
	if got := snapshotKeys(s); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Errorf("keys after delete = %v, want [a c]", got)
	}
	if s.ApplyResult("b", ping.PingResult{Success: true}, nil) {
		t.Error("ApplyResult after delete should return false")
	}
}

func TestSortByHostAndReverse(t *testing.T) {
	s := NewSharedState(0)
	for _, h := range []string{"bravo", "alpha", "charlie"} {
		if err := s.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost: %v", err)
		}
	}
	if got := snapshotKeys(s); got[0] != "alpha" || got[2] != "charlie" {
		t.Errorf("default host-asc order = %v", got)
	}
	s.SetSort(SortHost, SortDesc)
	if got := snapshotKeys(s); got[0] != "charlie" || got[2] != "alpha" {
		t.Errorf("host-desc order = %v", got)
	}
	if key, dir := s.SortConfig(); key != SortHost || dir != SortDesc {
		t.Errorf("SortConfig = %v %v", key, dir)
	}
}

func TestSortByRTTReordersOnResult(t *testing.T) {
	s := NewSharedState(0)
	for _, h := range []string{"slow", "fast"} {
		if err := s.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost: %v", err)
		}
	}
	s.SetSort(SortRTT, SortAsc)
	s.ApplyResult("slow", ping.PingResult{Success: true, RTT: 500 * time.Millisecond}, nil)
	s.ApplyResult("fast", ping.PingResult{Success: true, RTT: 5 * time.Millisecond}, nil)
	if got := snapshotKeys(s); got[0] != "fast" {
		t.Errorf("rtt-asc order = %v, want fast first", got)
	}
	s.SetSort(SortRTT, SortDesc)
	if got := snapshotKeys(s); got[0] != "slow" {
		t.Errorf("rtt-desc order = %v, want slow first", got)
	}
}

func TestSortByFailureAndSuccessPct(t *testing.T) {
	s := NewSharedState(0)
	for _, h := range []string{"good", "bad"} {
		if err := s.AddHost(h, time.Second, time.Second); err != nil {
			t.Fatalf("AddHost: %v", err)
		}
	}
	s.ApplyResult("good", ping.PingResult{Success: true}, nil)
	s.ApplyResult("bad", ping.PingResult{Success: false, RawError: "x"}, nil)

	s.SetSort(SortFailure, SortAsc)
	if got := snapshotKeys(s); got[0] != "good" {
		t.Errorf("failure-asc order = %v, want good first", got)
	}
	s.SetSort(SortSuccessPct, SortAsc)
	if got := snapshotKeys(s); got[0] != "bad" {
		t.Errorf("successpct-asc order = %v, want bad first", got)
	}
}

func TestSetIntervalAndTimeout(t *testing.T) {
	s := NewSharedState(0)
	if err := s.AddHost("h", time.Second, time.Second); err != nil {
		t.Fatalf("AddHost: %v", err)
	}
	s.SetInterval(7 * time.Second)
	s.SetTimeout(3 * time.Second)
	_, _, _, interval, timeout, ok := s.HostConfig("h")
	if !ok || interval != 7*time.Second || timeout != 3*time.Second {
		t.Errorf("HostConfig = %v/%v ok=%v, want 7s/3s", interval, timeout, ok)
	}
}

func TestDisplayNameFallback(t *testing.T) {
	for in, want := range map[string]string{
		"":      "~n/a~",
		"  ":    "~n/a~",
		"N/A":   "~n/a~",
		"real":  "real",
		"other": "other",
	} {
		if got := displayName(&HostState{ResolvedName: in}); got != want {
			t.Errorf("displayName(%q) = %q, want %q", in, got, want)
		}
	}
}
