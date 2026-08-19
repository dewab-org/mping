package ping

import (
	"context"
	"net"
	"time"

	goping "github.com/go-ping/ping"
)

// NativeBackend uses the go-ping library for ICMP probing.
type NativeBackend struct{}

func NewNativeBackend() *NativeBackend {
	return &NativeBackend{}
}

func (b *NativeBackend) Ping(ctx context.Context, target Target, timeout time.Duration) (PingResult, error) {
	hostName := target.HostName

	pinger, err := goping.NewPinger(hostName)
	if err != nil {
		return PingResult{ResolvedName: hostName, RawError: err.Error()}, err
	}
	pinger.Count = 1
	pinger.Timeout = timeout
	pinger.SetPrivileged(false)

	// The pinger already resolved the target, so display the IP it actually
	// probes instead of issuing a second forward lookup that can disagree
	// under round-robin DNS. Only IP literals need a (cached) reverse lookup
	// for a display name; it runs concurrently with the probe, bounded by the
	// same timeout.
	ip := pinger.IPAddr().IP.String()
	resolvedCh := make(chan string, 1)
	if net.ParseIP(hostName) != nil {
		rctx, rcancel := context.WithTimeout(ctx, timeout)
		defer rcancel()
		go func() {
			_, name := resolveHost(rctx, hostName)
			resolvedCh <- name
		}()
	} else {
		resolvedCh <- hostName
	}

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
		ResolvedName: <-resolvedCh,
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
