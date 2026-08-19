package ping

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
)

// dnsCacheTTL bounds how often steady-state pinging re-queries DNS per host.
const dnsCacheTTL = 60 * time.Second

type dnsEntry struct {
	ip       string
	resolved string
	expires  time.Time
}

var dnsCache = struct {
	sync.Mutex
	entries map[string]dnsEntry
}{entries: map[string]dnsEntry{}}

// resolveHost resolves a host's IP and display name. Lookups honor the
// caller's context (so a stalled DNS server cannot outlive the ping timeout)
// and successful results are cached briefly to avoid issuing queries for
// every probe of every host.
func resolveHost(ctx context.Context, host string) (ip string, resolved string) {
	dnsCache.Lock()
	if e, ok := dnsCache.entries[host]; ok && time.Now().Before(e.expires) {
		dnsCache.Unlock()
		return e.ip, e.resolved
	}
	dnsCache.Unlock()

	ip, resolved = lookupHost(ctx, host)

	// Don't cache failures: a lookup truncated by the caller's timeout or a
	// transiently unreachable resolver should retry on the next probe rather
	// than pinning an empty result for a full TTL.
	if ctx.Err() != nil || ip == "" {
		return ip, resolved
	}

	dnsCache.Lock()
	dnsCache.entries[host] = dnsEntry{ip: ip, resolved: resolved, expires: time.Now().Add(dnsCacheTTL)}
	dnsCache.Unlock()
	return ip, resolved
}

func lookupHost(ctx context.Context, host string) (ip string, resolved string) {
	var resolver net.Resolver
	if parsed := net.ParseIP(host); parsed != nil {
		ip = parsed.String()
		names, err := resolver.LookupAddr(ctx, ip)
		if err != nil || len(names) == 0 {
			return ip, "N/A"
		}
		return ip, strings.TrimSuffix(names[0], ".")
	}

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err == nil && len(ips) > 0 {
		ip = ips[0].String()
	}
	return ip, host
}
