package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := middleware.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("invalid log JSON: %v", err)
	}
	if entry["msg"] != "http_request" {
		t.Errorf("expected msg=http_request, got %v", entry["msg"])
	}
	if entry["request_id"] != "req-123" {
		t.Errorf("expected request_id=req-123, got %v", entry["request_id"])
	}
}
