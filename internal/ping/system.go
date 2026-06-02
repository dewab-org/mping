package ping

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SystemBackend invokes the system ping command with OS-aware arguments.
type SystemBackend struct {
	Command string
	Args    []string
}

func NewSystemBackend(command string, args []string) *SystemBackend {
	return &SystemBackend{
		Command: command,
		Args:    args,
	}
}

func (b *SystemBackend) Ping(ctx context.Context, target Target, timeout time.Duration) (PingResult, error) {
	hostName := target.HostName
	ip, resolvedName := resolveHost(hostName)

	args := b.buildArgs(timeout)
	args = append(args, hostName)

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, b.Command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	rtt := time.Since(start)

	res := PingResult{
		ResolvedIP:   ip,
		ResolvedName: resolvedName,
		RTT:          rtt,
		Success:      err == nil,
		RawError:     strings.TrimSpace(stderr.String()),
	}
	return res, err
}

func (b *SystemBackend) buildArgs(timeout time.Duration) []string {
	secs := int(timeout.Seconds())
	msecs := int(timeout.Milliseconds())

	switch runtime.GOOS {
	case "linux":
		args := append([]string{}, b.Args...)
		args = append(args, "-w", fmt.Sprintf("%d", secs))
		return args
	case "darwin":
		args := append([]string{}, b.Args...)
		args = append(args, "-W", fmt.Sprintf("%d", msecs))
		return args
	default:
		args := append([]string{}, b.Args...)
		args = append(args, "-W", fmt.Sprintf("%d", secs))
		return args
	}
}

func resolveHost(host string) (ip string, resolved string) {
	if parsed := net.ParseIP(host); parsed != nil {
		ip = parsed.String()
		names, err := net.LookupAddr(ip)
		if err != nil || len(names) == 0 {
			return ip, "N/A"
		}
		return ip, strings.TrimSuffix(names[0], ".")
	}

	ips, err := net.LookupIP(host)
	if err == nil && len(ips) > 0 {
		ip = ips[0].String()
	}
	return ip, host
}
