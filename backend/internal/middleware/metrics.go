// backend/internal/middleware/metrics.go
package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// WSMessagesDroppedTotal counts WebSocket messages dropped because the
	// recipient's send buffer was full. A non-zero value indicates a slow
	// or wedged consumer; the hub closes such connections to force a
	// reconnect (finding 4.I in the 2026-04-10 review).
	WSMessagesDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ws_messages_dropped_total",
		Help: "Total WebSocket messages dropped due to slow consumers.",
	}, []string{"reason"})
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter so WebSocket upgrades
// on the /api/ws/* route work even though this middleware wraps the writer.
// Without this, gorilla/websocket's Upgrade() fails with HTTP 500
// ("websocket: response does not implement http.Hijacker").
func (rr *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rr.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("middleware: ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Metrics wraps handlers to record Prometheus HTTP metrics.
func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rr.status)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}
