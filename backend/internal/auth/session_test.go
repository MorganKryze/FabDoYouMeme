// backend/internal/auth/session_test.go

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

func TestMeHandler_IncludesCreatedAt(t *testing.T) {
	h := &auth.Handler{}
	created := time.Date(2026, time.January, 15, 9, 30, 0, 0, time.UTC)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID:    "00000000-0000-0000-0000-000000000001",
		Username:  "morgan",
		Email:     "m@example.com",
		Role:      "player",
		CreatedAt: created,
	}))

	rec := httptest.NewRecorder()
	h.Me(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := body["created_at"].(string)
	if !ok {
		t.Fatalf("created_at missing or not string; body=%v", body)
	}
	if got != "2026-01-15T09:30:00Z" {
		t.Errorf("created_at = %q, want %q", got, "2026-01-15T09:30:00Z")
	}
}
