package ping

import (
	"context"
	"time"
)

// PingResult captures the outcome of a ping attempt.
type PingResult struct {
	ResolvedIP   string
	ResolvedName string
	RTT          time.Duration
	Success      bool
	RawError     string
}

// PingBackend abstracts the mechanism used to ping a host.
type PingBackend interface {
	Ping(ctx context.Context, hostName string, timeout time.Duration) (PingResult, error)
}
