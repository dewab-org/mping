package ping

import (
	"context"
	"time"

	goping "github.com/go-ping/ping"
)

// NativeBackend uses the go-ping library for ICMP probing.
type NativeBackend struct{}

func NewNativeBackend() *NativeBackend {
	return &NativeBackend{}
}

func (b *NativeBackend) Ping(ctx context.Context, hostName string, timeout time.Duration) (PingResult, error) {
	ip, resolved := resolveHost(hostName)

	pinger, err := goping.NewPinger(hostName)
	if err != nil {
		return PingResult{ResolvedIP: ip, ResolvedName: resolved, RawError: err.Error()}, err
	}
	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(false)

	start := time.Now()
	done := make(chan error, 1)
	go func() {
		done <- pinger.Run()
	}()

	select {
	case <-ctx.Done():
		pinger.Stop()
		err = ctx.Err()
	case err = <-done:
	}

	rtt := time.Since(start)
	stats := pinger.Statistics()

	res := PingResult{
		ResolvedIP:   ip,
		ResolvedName: resolved,
		RTT:          rtt,
		Success:      err == nil && stats.PacketsRecv > 0,
	}
	if err != nil {
		res.RawError = err.Error()
	} else if stats.PacketsRecv == 0 {
		res.RawError = "no reply"
	}
	return res, err
}
