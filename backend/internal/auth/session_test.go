// backend/internal/auth/session_test.go
//go:build integration

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func seedSession(t *testing.T, q *db.Queries, userID uuid.UUID, ttl time.Duration) (rawToken string) {
	t.Helper()
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	if _, err := q.CreateSession(context.Background(), db.CreateSessionParams{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}); err != nil {
		t.Fatalf("seedSession: %v", err)
	}
	return raw
}

func TestLogout_ClearsSession(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "frank", "frank@test.com")
	raw := seedSession(t, q, user.ID, time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: raw})
	req = req.WithContext(context.WithValue(req.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}))
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" && c.MaxAge == -1 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("session cookie was not cleared on logout")
	}
}

func TestMe_ReturnsCurrentUser(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "grace", "grace@test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID:   user.ID.String(),
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
	}))
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["username"] != "grace" {
		t.Errorf("want grace, got %s", resp["username"])
	}
}

func TestLogout_NoCookie_StillReturns200(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 even with no cookie, got %d", rec.Code)
	}
}

func TestMe_NoSession_Returns401(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	h.Me(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 without session, got %d", rec.Code)
	}
}
