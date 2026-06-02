package ping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPPing measures reachability by making an HTTP(S) request and recording the status.
func HTTPPing(ctx context.Context, target Target, timeout time.Duration) (PingResult, error) {
	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return httpPing(ctx, target, timeout, client)
}

func httpPing(ctx context.Context, target Target, timeout time.Duration, client http.Client) (PingResult, error) {
	rawURL := strings.TrimSpace(target.HostName)
	if rawURL == "" {
		err := fmt.Errorf("empty URL")
		return PingResult{RawError: err.Error()}, err
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = target.Protocol + "://" + rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return PingResult{RawError: err.Error()}, err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		err := fmt.Errorf("URL scheme must be http or https")
		return PingResult{RawError: err.Error()}, err
	}
	if parsed.Hostname() == "" {
		err := fmt.Errorf("URL host is required")
		return PingResult{RawError: err.Error()}, err
	}

	ip, _ := resolveHost(parsed.Hostname())
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return PingResult{ResolvedIP: ip, ResolvedName: parsed.String(), RawError: err.Error()}, err
	}

	start := time.Now()
	resp, err := client.Do(req)
	rtt := time.Since(start)
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	if err != nil {
		return PingResult{
			ResolvedIP:   ip,
			ResolvedName: parsed.String(),
			RTT:          rtt,
			RawError:     err.Error(),
		}, err
	}

	status := strconv.Itoa(resp.StatusCode) + " " + http.StatusText(resp.StatusCode)
	return PingResult{
		ResolvedIP:   ip,
		ResolvedName: parsed.String(),
		RTT:          rtt,
		Success:      true,
		Status:       strings.TrimSpace(status),
	}, nil
}
