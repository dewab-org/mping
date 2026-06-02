package state

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type HostSpec struct {
	Key      string
	Name     string
	Protocol string
	TCPPort  int
}

func ParseHostSpec(input, defaultProtocol string, defaultPort int) (HostSpec, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return HostSpec{}, fmt.Errorf("empty host")
	}

	if isHTTPURL(raw) {
		return parseHTTPSpec(raw)
	}

	protocol := strings.ToLower(strings.TrimSpace(defaultProtocol))
	hostPort := raw
	explicitProtocol := false
	if rest, ok := strings.CutPrefix(strings.ToLower(raw), "tcp://"); ok {
		protocol = "tcp"
		explicitProtocol = true
		hostPort = raw[len(raw)-len(rest):]
	} else if rest, ok := strings.CutPrefix(strings.ToLower(raw), "icmp://"); ok {
		protocol = "icmp"
		explicitProtocol = true
		hostPort = raw[len(raw)-len(rest):]
	} else if rest, ok := strings.CutPrefix(strings.ToLower(raw), "tcp:"); ok {
		protocol = "tcp"
		explicitProtocol = true
		hostPort = raw[len(raw)-len(rest):]
	} else if rest, ok := strings.CutPrefix(strings.ToLower(raw), "icmp:"); ok {
		protocol = "icmp"
		explicitProtocol = true
		hostPort = raw[len(raw)-len(rest):]
	}

	if protocol == "" {
		protocol = "icmp"
	}
	if protocol != "icmp" && protocol != "tcp" && protocol != "http" && protocol != "https" {
		return HostSpec{}, fmt.Errorf("protocol must be icmp, tcp, http, or https")
	}

	host := strings.TrimSpace(hostPort)
	if protocol == "http" || protocol == "https" {
		return parseHTTPSpec(protocol + "://" + host)
	}

	port := defaultPort
	if parsedHost, parsedPort, ok, err := splitHostPort(host); err != nil {
		return HostSpec{}, err
	} else if ok {
		if explicitProtocol && protocol == "icmp" {
			return HostSpec{}, fmt.Errorf("icmp targets do not accept a port")
		}
		host = parsedHost
		port = parsedPort
		protocol = "tcp"
	}

	host = strings.Trim(host, "[]")
	if host == "" {
		return HostSpec{}, fmt.Errorf("empty host")
	}
	if port < 1 || port > 65535 {
		return HostSpec{}, fmt.Errorf("tcp port must be 1-65535, got %d", port)
	}

	key := strings.ToLower(protocol) + "|" + strings.ToLower(host)
	if protocol == "tcp" {
		key = fmt.Sprintf("%s|%s|%d", protocol, strings.ToLower(host), port)
	}
	return HostSpec{
		Key:      key,
		Name:     host,
		Protocol: protocol,
		TCPPort:  port,
	}, nil
}

func isHTTPURL(input string) bool {
	lower := strings.ToLower(input)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func parseHTTPSpec(raw string) (HostSpec, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return HostSpec{}, err
	}
	protocol := strings.ToLower(parsed.Scheme)
	if protocol != "http" && protocol != "https" {
		return HostSpec{}, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.Hostname() == "" {
		return HostSpec{}, fmt.Errorf("URL host is required")
	}
	return HostSpec{
		Key:      protocol + "|" + strings.ToLower(parsed.String()),
		Name:     parsed.String(),
		Protocol: protocol,
	}, nil
}

func splitHostPort(input string) (host string, port int, ok bool, err error) {
	if h, p, splitErr := net.SplitHostPort(input); splitErr == nil {
		parsed, parseErr := strconv.Atoi(p)
		if parseErr != nil {
			return "", 0, false, fmt.Errorf("invalid tcp port %q", p)
		}
		return h, parsed, true, nil
	}

	if strings.Count(input, ":") != 1 {
		return "", 0, false, nil
	}
	h, p, found := strings.Cut(input, ":")
	if !found || h == "" || p == "" {
		return "", 0, false, nil
	}
	parsed, parseErr := strconv.Atoi(p)
	if parseErr != nil {
		return "", 0, false, fmt.Errorf("invalid tcp port %q", p)
	}
	return h, parsed, true, nil
}
