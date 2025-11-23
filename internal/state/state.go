package state

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"mping/internal/ping"
)

type SortKey string
type SortDirection string

const (
	SortHost       SortKey = "host"
	SortRTT        SortKey = "rtt"
	SortSuccess    SortKey = "success"
	SortSuccessPct SortKey = "successpct"
	SortFailure    SortKey = "failure"
	SortLastOK     SortKey = "lastok"
	SortError      SortKey = "error"
	SortIP         SortKey = "ip"

	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// HostState holds mutable runtime statistics for a host.
type HostState struct {
	Name         string
	IP           string
	ResolvedName string
	SuccessCount int64
	FailureCount int64
	LastRTT      time.Duration
	LastError    string
	LastOK       time.Time
	Interval     time.Duration
	Timeout      time.Duration
}

// HostSnapshot is a read-only copy used for rendering.
type HostSnapshot struct {
	Key          string
	Name         string
	IP           string
	ResolvedName string
	SuccessCount int64
	FailureCount int64
	LastRTT      time.Duration
	LastError    string
	LastOK       time.Time
	Interval     time.Duration
	Timeout      time.Duration
}

// SharedState coordinates concurrent access to hosts and ordering.
type SharedState struct {
	mu       sync.RWMutex
	hosts    map[string]*HostState
	order    []string
	sortKey  SortKey
	sortDir  SortDirection
	maxHosts int
}

func NewSharedState(maxHosts int) *SharedState {
	return &SharedState{
		hosts:    make(map[string]*HostState),
		sortKey:  SortHost,
		sortDir:  SortAsc,
		maxHosts: maxHosts,
	}
}

func (s *SharedState) AddHost(host string, interval, timeout time.Duration) error {
	key := strings.TrimSpace(host)
	if key == "" {
		return errors.New("empty host")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.hosts[key]; exists {
		return nil
	}
	if s.maxHosts > 0 && len(s.hosts) >= s.maxHosts {
		return errors.New("max hosts reached")
	}

	s.hosts[key] = &HostState{
		Name:         key,
		IP:           "",
		ResolvedName: key,
		LastRTT:      0,
		Interval:     interval,
		Timeout:      timeout,
	}
	s.order = append(s.order, key)
	s.sortLocked()
	return nil
}

func (s *SharedState) DeleteHost(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.hosts[host]; !ok {
		return
	}
	delete(s.hosts, host)
	for i, k := range s.order {
		if k == host {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}

// ApplyResult merges a ping result into the host state.
func (s *SharedState) ApplyResult(key string, res ping.PingResult, err error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, ok := s.hosts[key]
	if !ok {
		return false
	}

	h.IP = res.ResolvedIP
	if res.ResolvedName != "" {
		h.ResolvedName = res.ResolvedName
	}
	if res.Success {
		h.SuccessCount++
		h.LastOK = time.Now()
		h.LastError = ""
	} else {
		h.FailureCount++
		h.LastError = res.RawError
	}
	h.LastRTT = res.RTT

	s.sortLocked()
	return true
}

func (s *SharedState) Snapshot() []HostSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]HostSnapshot, 0, len(s.order))
	for _, k := range s.order {
		h := s.hosts[k]
		out = append(out, HostSnapshot{
			Key:          k,
			Name:         h.Name,
			IP:           h.IP,
			ResolvedName: h.ResolvedName,
			SuccessCount: h.SuccessCount,
			FailureCount: h.FailureCount,
			LastRTT:      h.LastRTT,
			LastError:    h.LastError,
			LastOK:       h.LastOK,
			Interval:     h.Interval,
			Timeout:      h.Timeout,
		})
	}
	return out
}

func (s *SharedState) SetSort(key SortKey, dir SortDirection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sortKey = key
	s.sortDir = dir
	s.sortLocked()
}

func (s *SharedState) SortConfig() (SortKey, SortDirection) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sortKey, s.sortDir
}

func (s *SharedState) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.hosts {
		h.Interval = interval
	}
}

func (s *SharedState) SetTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.hosts {
		h.Timeout = timeout
	}
}

func (s *SharedState) HostConfig(key string) (name string, interval, timeout time.Duration, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hosts[key]
	if !ok {
		return "", 0, 0, false
	}
	return h.Name, h.Interval, h.Timeout, true
}

func (s *SharedState) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.hosts)
}

func (s *SharedState) sortLocked() {
	sort.SliceStable(s.order, func(i, j int) bool {
		return less(s.hosts, s.order[i], s.order[j], s.sortKey, s.sortDir)
	})
}
