// backend/internal/middleware/real_ip.go
//
// Trusted-proxy aware client IP extraction. The walker is the foundation that
// fixes both the per-IP rate limiter (5.B in the 2026-04-10 review) and
// RequirePrivateIP — both previously used r.RemoteAddr unconditionally and
// therefore saw every request as coming from the reverse proxy.
package middleware

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP returns the originating client IP for r, walking X-Forwarded-For
// only as long as each hop (starting from r.RemoteAddr) is in trustedProxies.
// The result is always the *host* portion of an address — never "ip:port".
//
// Walking semantics:
//
//  1. Parse host out of r.RemoteAddr.
//  2. If that host is NOT in trustedProxies, return it. The immediate
//     connection is untrusted, so X-Forwarded-For is unverifiable and we
//     ignore it entirely. This is the anti-spoof guarantee that makes the
//     rest of the function safe.
//  3. Otherwise pop the rightmost X-Forwarded-For entry. If there is none,
//     return the current trusted hop — it's the most upstream IP we know.
//  4. If the popped entry is NOT in trustedProxies, return it. That's the
//     real client at the trust boundary.
//  5. Otherwise loop with the popped entry as the new "current".
//
// Empty trustedProxies disables XFF entirely and the function reduces to
// "return host portion of r.RemoteAddr" — which is the safe default for
// deployments without a reverse proxy.
func ClientIP(r *http.Request, trustedProxies []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// r.RemoteAddr might not contain a port (e.g. tests, unix sockets);
		// fall back to the raw value rather than dropping the request entirely.
		host = r.RemoteAddr
	}

	if !ipInTrusted(host, trustedProxies) {
		return host
	}

	// Flatten all X-Forwarded-For headers into a single ordered list of IPs.
	// Multiple headers may exist when several proxies in the chain each
	// appended their own header instead of mutating an existing one; the
	// arrival order in r.Header.Values is the order in which they were added,
	// so concatenating preserves "leftmost = original client".
	chain := flattenXFF(r.Header.Values("X-Forwarded-For"))
	current := host
	for i := len(chain) - 1; i >= 0; i-- {
		hop := chain[i]
		if !ipInTrusted(hop, trustedProxies) {
			return hop
		}
		current = hop
	}
	return current
}

// flattenXFF takes the slice of X-Forwarded-For header values, splits each
// on commas, trims whitespace, and returns the resulting list. Empty entries
// are dropped to tolerate malformed headers like "a, , b".
func flattenXFF(headers []string) []string {
	out := make([]string, 0, len(headers))
	for _, h := range headers {
		for _, part := range strings.Split(h, ",") {
			p := strings.TrimSpace(part)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// ipInTrusted reports whether ip parses and falls within any of the supplied
// CIDR ranges. An empty trusted slice always returns false.
func ipInTrusted(ip string, trusted []*net.IPNet) bool {
	if len(trusted) == 0 {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range trusted {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies parses a comma-separated list of CIDR ranges or bare IPs
// into a slice of *net.IPNet, suitable for passing to ClientIP. Bare IPs are
// promoted to /32 (v4) or /128 (v6). Empty input returns (nil, nil) — the
// safe "no proxies trusted" default. This is the entry point used by
// config.Load when reading the TRUSTED_PROXIES env var.
func ParseTrustedProxies(s string) ([]*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]*net.IPNet, 0, len(parts))
	for _, raw := range parts {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, &net.ParseError{Type: "trusted proxy IP", Text: entry}
			}
			if ip.To4() != nil {
				entry += "/32"
			} else {
				entry += "/128"
			}
		}
		_, n, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
