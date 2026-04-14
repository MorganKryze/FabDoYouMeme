package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// SessionLookupFn is the DB query function injected at startup.
// Returns userID, username, email, role, isActive, createdAt; returns zero values if not found.
type SessionLookupFn func(ctx context.Context, tokenHash string) (userID, username, email, role string, isActive bool, createdAt time.Time, err error)

func Session(lookup SessionLookupFn, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			hash := sha256token(cookie.Value)
			userID, username, email, role, isActive, createdAt, err := lookup(r.Context(), hash)
			if err != nil {
				logger.Error("session lookup failed",
					"error", err,
					"request_id", r.Header.Get("X-Request-ID"),
				)
				next.ServeHTTP(w, r)
				return
			}
			if !isActive {
				next.ServeHTTP(w, r)
				return
			}
			r = SetSessionUser(r, SessionUser{
				UserID:    userID,
				Username:  username,
				Email:     email,
				Role:      role,
				CreatedAt: createdAt,
			})
			next.ServeHTTP(w, r)
		})
	}
}

func sha256token(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
