// backend/internal/middleware/ip_allowlist.go
package middleware

import (
	"net"
	"net/http"
)

var allowedNets = []*net.IPNet{
	mustParseCIDR("127.0.0.0/8"),
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// RequirePrivateIP blocks requests whose originating client IP is not in the
// loopback or RFC-1918 private ranges. Use it to protect internal-only
// endpoints such as /api/metrics.
//
// trustedProxies is the same allowlist passed to ClientIP — without it the
// middleware would compare the proxy's address (always private) instead of
// the real client's address (which is what we actually want to gate on).
// Pre-fix this was finding 5.B's "fully open or fully closed" bug.
func RequirePrivateIP(trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := ClientIP(r, trustedProxies)
			ip := net.ParseIP(host)
			for _, n := range allowedNets {
				if n.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
