package ping

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPPingCapturesStatus(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	res, err := httpPing(context.Background(), Target{HostName: "http://127.0.0.1/forbidden", Protocol: "http"}, time.Second, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if res.Status != "403 Forbidden" {
		t.Fatalf("status = %q, want 403 Forbidden", res.Status)
	}
}

func TestHTTPPingAcceptsBareHostWithProtocol(t *testing.T) {
	client := http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://127.0.0.1/health" {
			t.Fatalf("URL = %q, want https://127.0.0.1/health", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusAccepted,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	res, err := httpPing(context.Background(), Target{HostName: "127.0.0.1/health", Protocol: "https"}, time.Second, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "202 Accepted" {
		t.Fatalf("status = %q, want 202 Accepted", res.Status)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
