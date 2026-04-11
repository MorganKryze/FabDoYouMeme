// backend/internal/middleware/real_ip_test.go
//
// middleware.ClientIP must walk X-Forwarded-For from right to left, only
// following the chain while the current hop is in the trusted-proxy
// allowlist. Without this, behind a reverse proxy every request appears
// to come from the proxy's IP, so all rate limit buckets collapse into
// one and RequirePrivateIP is either fully open or fully closed.
package middleware_test

import (
	"net"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// mustCIDR is a tiny test helper that panics on parse failures so the table
// stays terse. Real callers (config loader) handle the error properly.
func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("mustCIDR(%q): %v", s, err)
	}
	return n
}

// TestMiddleware_ClientIPTrustedProxyWalk is the P1.5 acceptance test. Each
// row exercises one branch of the walker; comments call out which.
func TestMiddleware_ClientIPTrustedProxyWalk(t *testing.T) {
	trusted := []*net.IPNet{
		mustCIDR(t, "10.0.0.0/8"),     // typical internal proxy network
		mustCIDR(t, "192.168.0.0/16"), // typical home/lab proxy network
	}

	tests := []struct {
		name           string
		remoteAddr     string
		xff            []string // each entry becomes one X-Forwarded-For header (Go joins on read)
		trustedProxies []*net.IPNet
		want           string
	}{
		// Direct, no proxies trusted: XFF must be ignored even when present —
		// this is the core anti-spoof guarantee.
		{
			name:           "direct connection, empty trust list, ignores XFF",
			remoteAddr:     "203.0.113.10:5555",
			xff:            []string{"8.8.8.8"},
			trustedProxies: nil,
			want:           "203.0.113.10",
		},
		{
			name:           "direct connection, trusted list set but RemoteAddr untrusted, ignores XFF",
			remoteAddr:     "203.0.113.10:5555",
			xff:            []string{"8.8.8.8, 10.0.0.5"},
			trustedProxies: trusted,
			want:           "203.0.113.10",
		},

		// One trusted proxy, no XFF — best we can do is the proxy's own IP.
		{
			name:           "trusted proxy, no XFF",
			remoteAddr:     "10.0.0.5:443",
			xff:            nil,
			trustedProxies: trusted,
			want:           "10.0.0.5",
		},

		// Single XFF entry behind a single trusted proxy — that entry is the client.
		{
			name:           "trusted proxy, single XFF entry is the client",
			remoteAddr:     "10.0.0.5:443",
			xff:            []string{"203.0.113.99"},
			trustedProxies: trusted,
			want:           "203.0.113.99",
		},

		// Multi-hop chain: client → trusted A → trusted B → server. Walk pops
		// rightmost (B's recorded predecessor), continues through A, lands on
		// the leftmost untrusted entry (the real client).
		{
			name:           "trusted proxy chain, leftmost is the real client",
			remoteAddr:     "10.0.0.5:443",
			xff:            []string{"203.0.113.99, 10.1.1.1, 192.168.1.50"},
			trustedProxies: trusted,
			want:           "203.0.113.99",
		},

		// Spoof attempt inside the chain: client puts a fake "trusted" IP at
		// the start. Walker still stops at the first UNTRUSTED hop encountered
		// while popping right-to-left. Here that's "1.2.3.4" between two
		// trusted proxies — so 1.2.3.4 is returned (the real boundary), NOT
		// the spoofed leftmost 10.0.0.99.
		{
			name:           "spoof attempt — first untrusted hop wins",
			remoteAddr:     "10.0.0.5:443",
			xff:            []string{"10.0.0.99, 1.2.3.4, 192.168.1.50"},
			trustedProxies: trusted,
			want:           "1.2.3.4",
		},

		// All XFF entries trusted: we exhaust the chain. Per the walker's
		// "no more entries → return the current trusted hop" rule, we return
		// the leftmost trusted IP — that's the most upstream hop we know about.
		{
			name:           "all hops trusted — return leftmost trusted entry",
			remoteAddr:     "10.0.0.5:443",
			xff:            []string{"10.0.0.1, 192.168.1.50"},
			trustedProxies: trusted,
			want:           "10.0.0.1",
		},

		// Two separate X-Forwarded-For headers (curl -H "X-Forwarded-For: a"
		// -H "X-Forwarded-For: b") must be flattened in header-arrival order:
		// each header was added by an earlier proxy, so the second header is
		// the more recent hop.
		{
			name:           "multiple XFF headers are flattened in arrival order",
			remoteAddr:     "10.0.0.5:443",
			xff:            []string{"203.0.113.99", "10.1.1.1"},
			trustedProxies: trusted,
			want:           "203.0.113.99",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remoteAddr
			for _, h := range tc.xff {
				r.Header.Add("X-Forwarded-For", h)
			}
			got := middleware.ClientIP(r, tc.trustedProxies)
			if got != tc.want {
				t.Errorf("ClientIP(%q, xff=%v) = %q, want %q", tc.remoteAddr, tc.xff, got, tc.want)
			}
		})
	}
}
