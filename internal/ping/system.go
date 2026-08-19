package ping

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
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

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The lookup is display-only; run it concurrently with the probe so a
	// slow resolver cannot eat into the ping's timeout budget.
	type resolution struct{ ip, name string }
	resolvedCh := make(chan resolution, 1)
	go func() {
		ip, name := resolveHost(ctxTimeout, hostName)
		resolvedCh <- resolution{ip: ip, name: name}
	}()

	args := b.buildArgs(timeout)
	args = append(args, hostName)

	cmd := exec.CommandContext(ctxTimeout, b.Command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	// Prefer the RTT reported by ping itself; the wall clock includes
	// fork/exec and ping's own DNS resolution. On failure keep the wall
	// clock: a killed multi-packet run may still print an early reply's
	// time=, which would make a stalled host look fast.
	rtt := time.Since(start)
	if err == nil {
		if parsed, ok := parsePingRTT(stdout.String()); ok {
			rtt = parsed
		}
	}
	resolved := <-resolvedCh

	res := PingResult{
		ResolvedIP:   resolved.ip,
		ResolvedName: resolved.name,
		RTT:          rtt,
		Success:      err == nil,
		RawError:     strings.TrimSpace(stderr.String()),
	}
	return res, err
}

var pingRTTPattern = regexp.MustCompile(`time[=<]([0-9]+(?:\.[0-9]+)?)\s*ms`)

// parsePingRTT extracts the round-trip time from ping's output
// (e.g. "64 bytes from 1.1.1.1: icmp_seq=0 ttl=58 time=12.383 ms").
func parsePingRTT(output string) (time.Duration, bool) {
	m := pingRTTPattern.FindStringSubmatch(output)
	if m == nil {
		return 0, false
	}
	ms, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return time.Duration(ms * float64(time.Millisecond)), true
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
