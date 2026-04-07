// backend/internal/api/health.go
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler handles /api/health and /api/health/deep.
type HealthHandler struct {
	pool *pgxpool.Pool
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// Liveness handles GET /api/health — always 200 if process is up.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness handles GET /api/health/deep — checks DB connectivity.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	status := http.StatusOK

	if err := h.pool.Ping(ctx); err != nil {
		checks["postgres"] = "unreachable: " + err.Error()
		status = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	resp := map[string]any{"status": "ok", "checks": checks}
	if status != http.StatusOK {
		resp["status"] = "degraded"
	}
	writeJSON(w, status, resp)
}
