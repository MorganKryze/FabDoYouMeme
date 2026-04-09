package middleware

import (
	"context"
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestID is a middleware that assigns a unique request ID to each request.
// The ID is stored in the chi middleware context (accessible via chiMiddleware.GetReqID)
// and also set as the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		// Store in chi's context key so chiMiddleware.GetReqID works.
		ctx := context.WithValue(r.Context(), chiMiddleware.RequestIDKey, id)
		r = r.WithContext(ctx)
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
