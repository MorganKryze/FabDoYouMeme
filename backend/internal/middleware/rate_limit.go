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
	mu      sync.Mutex
	clients map[string]*rateLimiterEntry
	rate    rate.Limit
	burst   int
	clock   clock.Clock
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter constructs a RateLimiter. Pass clock.Real{} in production;
// tests can inject a *clock.Fake to exercise the eviction cadence without
// real sleeps.
func NewRateLimiter(requestsPerPeriod int, periodSeconds int, clk clock.Clock) *RateLimiter {
	if clk == nil {
		clk = clock.Real{}
	}
	r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
	rl := &RateLimiter{
		clients: make(map[string]*rateLimiterEntry),
		rate:    r,
		burst:   requestsPerPeriod,
		clock:   clk,
	}
	go rl.evictLoop()
	return rl
}

// evictLoop removes entries that have been idle for more than 1 hour.
func (rl *RateLimiter) evictLoop() {
	ticker := rl.clock.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C() {
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
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !rl.getLimiter(ip).Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Too many requests",
				"code":  "rate_limited",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
