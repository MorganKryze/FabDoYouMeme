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
	clients map[string]*rate.Limiter
	rate    rate.Limit
	burst   int
}

func NewRateLimiter(requestsPerPeriod int, periodSeconds int) *RateLimiter {
	r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
	return &RateLimiter{
		clients: make(map[string]*rate.Limiter),
		rate:    r,
		burst:   requestsPerPeriod,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok := rl.clients[ip]; ok {
		return l
	}
	l := rate.NewLimiter(rl.rate, rl.burst)
	rl.clients[ip] = l
	return l
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
