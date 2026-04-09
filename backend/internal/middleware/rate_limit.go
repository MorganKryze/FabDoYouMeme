package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*rateLimiterEntry
	rate    rate.Limit
	burst   int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(requestsPerPeriod int, periodSeconds int) *RateLimiter {
	r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
	rl := &RateLimiter{
		clients: make(map[string]*rateLimiterEntry),
		rate:    r,
		burst:   requestsPerPeriod,
	}
	go rl.evictLoop()
	return rl
}

// evictLoop removes entries that have been idle for more than 1 hour.
func (rl *RateLimiter) evictLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-1 * time.Hour)
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
	entry.lastSeen = time.Now()
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
