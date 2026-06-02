package ping

import "testing"

func TestTCPTarget(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultPort int
		wantHost    string
		wantPort    int
		wantErr     bool
	}{
		{
			name:        "bare host uses default port",
			input:       "example.com",
			defaultPort: 443,
			wantHost:    "example.com",
			wantPort:    443,
		},
		{
			name:        "host port overrides default",
			input:       "example.com:8443",
			defaultPort: 443,
			wantHost:    "example.com",
			wantPort:    8443,
		},
		{
			name:        "bracketed ipv6 host port",
			input:       "[2001:db8::1]:443",
			defaultPort: 80,
			wantHost:    "2001:db8::1",
			wantPort:    443,
		},
		{
			name:        "invalid default port",
			input:       "example.com",
			defaultPort: 0,
			wantErr:     true,
		},
		{
			name:        "invalid host port",
			input:       "example.com:70000",
			defaultPort: 443,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPort, err := tcpTarget(tt.input, tt.defaultPort)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotHost != tt.wantHost || gotPort != tt.wantPort {
				t.Fatalf("got %s:%d, want %s:%d", gotHost, gotPort, tt.wantHost, tt.wantPort)
			}
		})
	}
}
