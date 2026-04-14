package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type contextKey string

const sessionUserKey contextKey = "session_user"

// SessionUserContextKey is exported for use in tests that need to inject a session user directly.
var SessionUserContextKey = sessionUserKey

type SessionUser struct {
	UserID    string
	Username  string
	Email     string
	Role      string
	CreatedAt time.Time
}

func SetSessionUser(r *http.Request, u SessionUser) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionUserKey, u))
}

func GetSessionUser(r *http.Request) (SessionUser, bool) {
	u, ok := r.Context().Value(sessionUserKey).(SessionUser)
	return u, ok
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetSessionUser(r); !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := GetSessionUser(r)
		if !ok || u.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "Admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
