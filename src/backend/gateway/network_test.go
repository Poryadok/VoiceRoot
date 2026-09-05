package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustedProxyForwardedForChain(t *testing.T) {
	t.Parallel()

	g := &gateway{trustedProxies: parseTrustedProxies([]string{"192.0.2.0/24"})}

	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		want         string
	}{
		{
			name:         "trusted append proxy selects rightmost untrusted address",
			remoteAddr:   "192.0.2.9:1234",
			forwardedFor: "198.51.100.77, 203.0.113.11",
			want:         "203.0.113.11",
		},
		{
			name:         "malformed forwarded chain fails safe to peer",
			remoteAddr:   "192.0.2.9:1234",
			forwardedFor: "198.51.100.77, not-an-ip, 192.0.2.11",
			want:         "192.0.2.9",
		},
		{
			name:         "untrusted peer cannot supply forwarded address",
			remoteAddr:   "203.0.113.11:1234",
			forwardedFor: "198.51.100.77",
			want:         "203.0.113.11",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Header.Set("X-Forwarded-For", tc.forwardedFor)
			req.RemoteAddr = tc.remoteAddr
			if got := g.clientIP(req); got != tc.want {
				t.Fatalf("clientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
