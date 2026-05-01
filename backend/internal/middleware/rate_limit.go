package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
)

type RateLimiter struct {
	mu             sync.Mutex
	clients        map[string]*rateLimiterEntry
	rate           rate.Limit
	burst          int
	clock          clock.Clock
	trustedProxies []*net.IPNet
	stop           chan struct{}
	done           chan struct{}
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter constructs a RateLimiter. Pass clock.Real{} in production;
// tests can inject a *clock.Fake to exercise the eviction cadence without
// real sleeps. trustedProxies is the allowlist used by ClientIP to walk
// X-Forwarded-For — pass nil for "no proxies trusted" (the safe default
// for direct deployments).
func NewRateLimiter(requestsPerPeriod int, periodSeconds int, clk clock.Clock, trustedProxies []*net.IPNet) *RateLimiter {
	if clk == nil {
		clk = clock.Real{}
	}
	r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
	rl := &RateLimiter{
		clients:        make(map[string]*rateLimiterEntry),
		rate:           r,
		burst:          requestsPerPeriod,
		clock:          clk,
		trustedProxies: trustedProxies,
		stop:           make(chan struct{}),
		done:           make(chan struct{}),
	}
	go rl.evictLoop()
	return rl
}

// Stop signals the eviction goroutine to exit and blocks until it has
// returned. Safe to call once; subsequent calls are no-ops. Callers should
// invoke Stop on every RateLimiter during server shutdown so the ticker
// goroutine does not outlive the process under embedded-server scenarios or
// across test runs (finding 4.A in the 2026-04-10 review).
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.stop:
		// already stopped
		return
	default:
	}
	close(rl.stop)
	<-rl.done
}

// evictLoop removes entries that have been idle for more than 1 hour.
// It returns when Stop() is called; the done channel is closed on exit so
// Stop can synchronise on the goroutine having actually returned.
func (rl *RateLimiter) evictLoop() {
	defer close(rl.done)
	ticker := rl.clock.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C():
			cutoff := rl.clock.Now().Add(-1 * time.Hour)
			rl.mu.Lock()
			for ip, entry := range rl.clients {
				if entry.lastSeen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	entry, ok := rl.clients[ip]
	if !ok {
		entry = &rateLimiterEntry{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.clients[ip] = entry
	}
	entry.lastSeen = rl.clock.Now()
	return entry.limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ClientIP returns the real client when r came through a trusted
		// reverse proxy, or the raw RemoteAddr otherwise. Without this
		// every client behind the proxy shared one rate-limit bucket
		// (finding 5.B in the 2026-04-10 review).
		ip := ClientIP(r, rl.trustedProxies)
		if !rl.getLimiter(ip).Allow() {
			writeRateLimited(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// PerUserMiddleware keys the rate-limit bucket by authenticated user ID
// rather than by client IP. Prefer this over Middleware on any handler
// that runs after the session middleware: in the prod reverse-proxy →
// SvelteKit → backend topology, SSR-side fetches (proxyToBackend,
// hydrateSession, apiFetch) all originate from the SvelteKit container's
// docker IP, so IP keying collapses every logged-in user into one shared
// bucket. Unauthenticated requests fall back to the IP bucket so the
// middleware can never be silently bypassed.
func (rl *RateLimiter) PerUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var key string
		if u, ok := GetSessionUser(r); ok && u.UserID != "" {
			key = "user:" + u.UserID
		} else {
			key = "ip:" + ClientIP(r, rl.trustedProxies)
		}
		if !rl.getLimiter(key).Allow() {
			writeRateLimited(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// PerKeyMiddleware lets the caller pick the bucket key (e.g. by URL param
// or any request-derived value). Use it when neither IP nor user identity
// is the right axis — most notably the unauthenticated guest-join, which
// must rate-limit per *room* so a single shared SvelteKit container IP
// (or operator-misconfigured TRUSTED_PROXIES) cannot ceiling the whole
// platform's guest onboarding. An empty key falls back to the IP bucket
// so the middleware can never be silently bypassed.
func (rl *RateLimiter) PerKeyMiddleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				key = "ip:" + ClientIP(r, rl.trustedProxies)
			}
			if !rl.getLimiter(key).Allow() {
				writeRateLimited(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"error": "Too many requests",
		"code":  "rate_limited",
	})
}
