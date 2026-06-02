package ping

import (
	"context"
	"strings"
	"sync"
	"time"
)

// PingResult captures the outcome of a ping attempt.
type PingResult struct {
	ResolvedIP   string
	ResolvedName string
	RTT          time.Duration
	Success      bool
	Status       string
	RawError     string
}

// PingBackend abstracts the mechanism used to ping a host.
type PingBackend interface {
	Ping(ctx context.Context, target Target, timeout time.Duration) (PingResult, error)
}

// ModeConfig describes the active probe mode.
type ModeConfig struct {
	Protocol      string
	ICMPBackend   string
	SystemCommand string
	SystemArgs    []string
	TCPPort       int
}

// MultiBackend routes probes to the active ICMP, TCP, HTTP, or HTTPS implementation.
type MultiBackend struct {
	mu     sync.RWMutex
	config ModeConfig
	native *NativeBackend
}

func NewMultiBackend(cfg ModeConfig) *MultiBackend {
	return &MultiBackend{
		config: cfg,
		native: NewNativeBackend(),
	}
}

func (b *MultiBackend) Update(cfg ModeConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config = cfg
}

func (b *MultiBackend) Ping(ctx context.Context, target Target, timeout time.Duration) (PingResult, error) {
	b.mu.RLock()
	cfg := b.config
	cfg.SystemArgs = append([]string{}, cfg.SystemArgs...)
	b.mu.RUnlock()

	target = target.WithDefaults(cfg.Protocol, cfg.TCPPort)

	switch strings.ToLower(target.Protocol) {
	case "http", "https":
		return HTTPPing(ctx, target, timeout)
	case "tcp":
		return TCPPing(ctx, target.HostName, target.TCPPort, timeout)
	default:
		if strings.EqualFold(cfg.ICMPBackend, "native") {
			return b.native.Ping(ctx, target, timeout)
		}
		return NewSystemBackend(cfg.SystemCommand, cfg.SystemArgs).Ping(ctx, target, timeout)
	}
}
