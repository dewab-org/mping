package state

import "testing"

func TestParseHostSpec(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		defaultProtocol string
		defaultPort     int
		wantName        string
		wantProtocol    string
		wantPort        int
		wantErr         bool
	}{
		{
			name:            "bare host uses defaults",
			input:           "example.com",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "example.com",
			wantProtocol:    "icmp",
			wantPort:        443,
		},
		{
			name:            "bare host port implies tcp",
			input:           "example.com:8443",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "example.com",
			wantProtocol:    "tcp",
			wantPort:        8443,
		},
		{
			name:            "tcp prefix with host port",
			input:           "tcp:example.com:8443",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "example.com",
			wantProtocol:    "tcp",
			wantPort:        8443,
		},
		{
			name:            "tcp scheme with default port",
			input:           "tcp://example.com",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "example.com",
			wantProtocol:    "tcp",
			wantPort:        443,
		},
		{
			name:            "icmp prefix",
			input:           "icmp:example.com",
			defaultProtocol: "tcp",
			defaultPort:     443,
			wantName:        "example.com",
			wantProtocol:    "icmp",
			wantPort:        443,
		},
		{
			name:            "bracketed ipv6 tcp",
			input:           "tcp://[2001:db8::1]:8443",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "2001:db8::1",
			wantProtocol:    "tcp",
			wantPort:        8443,
		},
		{
			name:            "explicit icmp rejects port",
			input:           "icmp:example.com:443",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantErr:         true,
		},
		{
			name:            "invalid named port",
			input:           "tcp:example.com:https",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantErr:         true,
		},
		{
			name:            "https URL",
			input:           "https://example.com/health",
			defaultProtocol: "icmp",
			defaultPort:     443,
			wantName:        "https://example.com/health",
			wantProtocol:    "https",
			wantPort:        0,
		},
		{
			name:            "global http protocol builds URL",
			input:           "example.com/status",
			defaultProtocol: "http",
			defaultPort:     443,
			wantName:        "http://example.com/status",
			wantProtocol:    "http",
			wantPort:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHostSpec(tt.input, tt.defaultProtocol, tt.defaultPort)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.wantName || got.Protocol != tt.wantProtocol || got.TCPPort != tt.wantPort {
				t.Fatalf("got %#v, want name=%q protocol=%q port=%d", got, tt.wantName, tt.wantProtocol, tt.wantPort)
			}
		})
	}
}
