package middleware_test

import (
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
