package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestRequestID_SetsHeader(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Error("X-Request-ID not set on request")
		}
		w.Header().Set("X-Request-ID", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID not in response")
	}
}
