package ping

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// TCPPing measures reachability by opening a TCP connection.
func TCPPing(ctx context.Context, hostName string, defaultPort int, timeout time.Duration) (PingResult, error) {
	host, port, err := tcpTarget(hostName, defaultPort)
	ip, resolved := resolveHost(host)
	if err != nil {
		return PingResult{ResolvedIP: ip, ResolvedName: resolved, RawError: err.Error()}, err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctxTimeout, "tcp", target)
	rtt := time.Since(start)
	if conn != nil {
		_ = conn.Close()
	}

	res := PingResult{
		ResolvedIP:   ip,
		ResolvedName: resolved,
		RTT:          rtt,
		Success:      err == nil,
	}
	if err != nil {
		res.RawError = err.Error()
	}
	return res, err
}

func tcpTarget(input string, defaultPort int) (string, int, error) {
	host := strings.TrimSpace(input)
	if host == "" {
		return "", 0, errors.New("empty host")
	}
	port := defaultPort

	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		parsed, parseErr := strconv.Atoi(p)
		if parseErr != nil {
			return host, 0, fmt.Errorf("invalid tcp port %q", p)
		}
		port = parsed
	} else if strings.Count(host, ":") == 1 {
		h, p, ok := strings.Cut(host, ":")
		if ok && h != "" && p != "" {
			parsed, parseErr := strconv.Atoi(p)
			if parseErr == nil {
				host = h
				port = parsed
			}
		}
	}

	if host == "" {
		return "", port, errors.New("empty host")
	}
	if port < 1 || port > 65535 {
		return host, port, fmt.Errorf("tcp port must be 1-65535, got %d", port)
	}
	return strings.Trim(host, "[]"), port, nil
}
