package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

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

func TestRequestID_Unique(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-ID", chiMiddleware.GetReqID(r.Context()))
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	id1 := w1.Header().Get("X-Seen-ID")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	id2 := w2.Header().Get("X-Seen-ID")

	if id1 == "" || id2 == "" {
		t.Fatal("request IDs should not be empty")
	}
	if id1 == id2 {
		t.Errorf("request IDs should be unique across requests, got %q for both", id1)
	}
}
