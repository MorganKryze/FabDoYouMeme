// backend/internal/api/health.go
package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// HealthHandler handles /api/health and /api/health/deep.
type HealthHandler struct {
	pool       *pgxpool.Pool
	storage    storage.Storage
	emailProbe func(context.Context) error // optional — nil means "skipped"
}

// NewHealthHandler constructs a handler. emailProbe is optional; if nil, the
// SMTP check is reported as "skipped" rather than being run.
func NewHealthHandler(pool *pgxpool.Pool, store storage.Storage, emailProbe func(context.Context) error) *HealthHandler {
	return &HealthHandler{pool: pool, storage: store, emailProbe: emailProbe}
}

// Liveness handles GET /api/health — always 200 if process is up.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// checkResult is the per-dependency payload emitted under "checks".
// LatencyMs is a float so sub-millisecond probes (local Postgres Ping()
// often lands in the 200–500 µs range) still report a meaningful number
// instead of rounding to 0 and disappearing from the dashboard.
type checkResult struct {
	Status    string  `json:"status"`          // "ok", "degraded", or "skipped"
	LatencyMs float64 `json:"latency_ms"`      // always emitted (0 for skipped)
	Error     string  `json:"error,omitempty"` // truncated to keep payloads bounded
}

// Readiness handles GET /api/health/deep — runs all dependency probes
// concurrently under a shared 2 s deadline. Returns 503 if any probe fails.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		checks = map[string]checkResult{}
	)
	set := func(name string, res checkResult) {
		mu.Lock()
		defer mu.Unlock()
		checks[name] = res
	}
	run := func(name string, fn func(context.Context) error) {
		defer wg.Done()
		start := time.Now()
		err := fn(ctx)
		elapsed := float64(time.Since(start).Microseconds()) / 1000.0
		if err != nil {
			set(name, checkResult{Status: "degraded", LatencyMs: elapsed, Error: truncate(err.Error(), 120)})
			return
		}
		set(name, checkResult{Status: "ok", LatencyMs: elapsed})
	}

	wg.Add(2)
	go run("postgres", func(c context.Context) error { return h.pool.Ping(c) })
	go run("rustfs", h.storage.Probe)
	if h.emailProbe != nil {
		wg.Add(1)
		go run("smtp", h.emailProbe)
	} else {
		set("smtp", checkResult{Status: "skipped"})
	}
	wg.Wait()

	status := "ok"
	httpStatus := http.StatusOK
	for _, c := range checks {
		if c.Status == "degraded" {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}
	writeJSON(w, httpStatus, map[string]any{"status": status, "checks": checks})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
