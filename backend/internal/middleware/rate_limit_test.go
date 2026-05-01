package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestRateLimit_BlocksExcess(t *testing.T) {
	// 1 request per minute — first should pass, second should be blocked
	rl := middleware.NewRateLimiter(1, 60, clock.Real{}, nil) // 1 req per 60 seconds burst=1, no trusted proxies
	t.Cleanup(rl.Stop)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := func() int {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Code
	}

	if code := req(); code != http.StatusOK {
		t.Fatalf("first request: want 200, got %d", code)
	}
	if code := req(); code != http.StatusTooManyRequests {
		t.Fatalf("second request: want 429, got %d", code)
	}
}

// TestRateLimit_PerUser_KeysByUserAndFallsBackToIP guards the prod-topology
// fix: when authenticated, each user gets their own bucket so two users
// behind the same IP (or behind one shared SvelteKit container) don't pool
// requests. When unauthenticated, the middleware falls back to IP so the
// gate cannot be silently bypassed.
func TestRateLimit_PerUser_KeysByUserAndFallsBackToIP(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 60, clock.Real{}, nil) // 1 req/min/key
	t.Cleanup(rl.Stop)
	handler := rl.PerUserMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Two distinct users sharing the same RemoteAddr — the SvelteKit
	// container shape we're trying to defend against. IP keying would 429
	// the second user; user keying lets them through.
	call := func(userID string) int {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		if userID != "" {
			r = r.WithContext(context.WithValue(r.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
				UserID: userID, Role: "player",
			}))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Code
	}

	if code := call("user-A"); code != http.StatusOK {
		t.Fatalf("user-A first call: want 200, got %d", code)
	}
	if code := call("user-A"); code != http.StatusTooManyRequests {
		t.Fatalf("user-A second call: want 429 (own bucket exhausted), got %d", code)
	}
	if code := call("user-B"); code != http.StatusOK {
		t.Fatalf("user-B first call: want 200 (independent bucket), got %d", code)
	}

	// Anonymous fallback to IP keying — the second anon call from the same
	// RemoteAddr must be rejected so an unauthenticated client cannot escape
	// the limiter by simply omitting a session.
	if code := call(""); code != http.StatusOK {
		t.Fatalf("anon first call: want 200, got %d", code)
	}
	if code := call(""); code != http.StatusTooManyRequests {
		t.Fatalf("anon second call: want 429 (IP bucket exhausted), got %d", code)
	}
}

// TestRateLimit_PerKey_KeysByCallerProvidedFunctionAndFallsBackToIP guards
// the guest-join fix: the bucket must key on whatever the caller picks
// (e.g. room code) so a single shared SvelteKit container IP cannot
// ceiling the platform's guest onboarding. An empty key falls back to IP
// so the gate is never silently bypassed.
func TestRateLimit_PerKey_KeysByCallerProvidedFunctionAndFallsBackToIP(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 60, clock.Real{}, nil) // 1 req/min/key
	t.Cleanup(rl.Stop)
	keyFn := func(r *http.Request) string { return r.Header.Get("X-Test-Room") }
	handler := rl.PerKeyMiddleware(keyFn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	call := func(roomCode, remoteAddr string) int {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = remoteAddr
		if roomCode != "" {
			r.Header.Set("X-Test-Room", roomCode)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Code
	}

	// Two distinct rooms from the same shared SvelteKit IP: each gets its
	// own bucket — the explicit prod-topology guarantee.
	if code := call("ABCD", "10.0.0.1:1234"); code != http.StatusOK {
		t.Fatalf("room ABCD first call: want 200, got %d", code)
	}
	if code := call("ABCD", "10.0.0.1:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("room ABCD second call: want 429 (own bucket exhausted), got %d", code)
	}
	if code := call("WXYZ", "10.0.0.1:1234"); code != http.StatusOK {
		t.Fatalf("room WXYZ first call: want 200 (independent bucket), got %d", code)
	}

	// Empty key → IP fallback. Different IPs get independent buckets, but
	// repeats from the same IP collapse into one — the safe-default bypass
	// guard.
	if code := call("", "10.0.0.99:1234"); code != http.StatusOK {
		t.Fatalf("anon first call: want 200, got %d", code)
	}
	if code := call("", "10.0.0.99:1234"); code != http.StatusTooManyRequests {
		t.Fatalf("anon second call from same IP: want 429, got %d", code)
	}
}

// TestRateLimit_StopCancelsEvictLoop proves Stop() terminates the eviction
// goroutine — the regression test for finding 4.A. We count how many
// evictLoop frames exist before and after Stop() and assert the count drops
// by exactly one, which is robust to other tests in the same binary that
// may hold live limiters when this case runs.
func TestRateLimit_StopCancelsEvictLoop(t *testing.T) {
	before := countEvictLoops()

	rl := middleware.NewRateLimiter(1, 60, clock.Real{}, nil)

	// `go rl.evictLoop()` does not guarantee the goroutine has begun
	// executing by the time NewRateLimiter returns — under load (race
	// detector, parallel packages) the scheduler can delay the first run
	// long enough for the stack dump to miss the frame. Spin until the
	// dump observes it, same shape as the post-Stop wait below.
	startDeadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(startDeadline) && countEvictLoops() != before+1 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := countEvictLoops(); got != before+1 {
		t.Fatalf("precondition: want %d evictLoop goroutines after NewRateLimiter, got %d", before+1, got)
	}

	rl.Stop()

	// Stop() blocks until the goroutine's done channel closes, but the
	// runtime can take a scheduler tick to finalise the dead stack. Spin
	// briefly to avoid a racy assertion on the dump.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if countEvictLoops() == before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("want %d evictLoop goroutines after Stop, got %d", before, countEvictLoops())
}

// TestRateLimit_StopIsIdempotent guarantees operators can call Stop twice
// (e.g. defer + explicit main shutdown) without a panic on close of a closed
// channel.
func TestRateLimit_StopIsIdempotent(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 60, clock.Real{}, nil)
	rl.Stop()
	rl.Stop() // must not panic
}

func allGoroutines() string {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

func countEvictLoops() int {
	return strings.Count(allGoroutines(), "RateLimiter).evictLoop")
}
