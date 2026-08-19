package ping

import (
	"testing"
	"time"
)

func TestParsePingRTT(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   time.Duration
		ok     bool
	}{
		{
			name:   "macos",
			output: "64 bytes from 1.1.1.1: icmp_seq=0 ttl=58 time=12.383 ms",
			want:   12383 * time.Microsecond,
			ok:     true,
		},
		{
			name:   "linux",
			output: "64 bytes from 1.1.1.1: icmp_seq=1 ttl=58 time=8.06 ms",
			want:   8060 * time.Microsecond,
			ok:     true,
		},
		{
			name:   "sub-millisecond",
			output: "64 bytes from 127.0.0.1: icmp_seq=0 ttl=64 time<1 ms",
			want:   time.Millisecond,
			ok:     true,
		},
		{
			name:   "no reply",
			output: "Request timeout for icmp_seq 0",
			ok:     false,
		},
		{
			name:   "empty",
			output: "",
			ok:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePingRTT(tt.output)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("rtt = %v, want %v", got, tt.want)
			}
		})
	}
}
