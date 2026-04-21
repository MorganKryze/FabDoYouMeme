package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// okHandler is a trivial next handler that records it was reached.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// injectUser sets a SessionUser into the request context.
func injectUser(r *http.Request, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID: "00000000-0000-0000-0000-000000000002", Username: "testuser", Email: "t@t.com", Role: role,
	}))
}

// ─── RequireAuth ──────────────────────────────────────────────────────────────

func TestRequireAuth_NoCookie_Returns401(t *testing.T) {
	h := middleware.RequireAuth(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 without session, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "unauthorized" {
		t.Errorf("want code=unauthorized, got %s", resp["code"])
	}
}

func TestRequireAuth_WithSession_PassesThrough(t *testing.T) {
	h := middleware.RequireAuth(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req = injectUser(req, "player")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 with valid session, got %d", rec.Code)
	}
}

// ─── RequireAdmin ─────────────────────────────────────────────────────────────

func TestRequireAdmin_NoSession_Returns403(t *testing.T) {
	h := middleware.RequireAdmin(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 without session, got %d", rec.Code)
	}
}

func TestRequireAdmin_PlayerRole_Returns403(t *testing.T) {
	h := middleware.RequireAdmin(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = injectUser(req, "player")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for player role, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "forbidden" {
		t.Errorf("want code=forbidden, got %s", resp["code"])
	}
}

func TestRequireAdmin_AdminRole_PassesThrough(t *testing.T) {
	h := middleware.RequireAdmin(okHandler)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = injectUser(req, "admin")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200 for admin role, got %d", rec.Code)
	}
}

// ─── Session middleware ────────────────────────────────────────────────────────

func TestSession_NoCookie_PassesThrough(t *testing.T) {
	// Without a session cookie, Session middleware should pass through without
	// setting a user — the downstream handler runs normally.
	var userFound bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, userFound = middleware.GetSessionUser(r)
		w.WriteHeader(http.StatusOK)
	})

	lookup := func(ctx context.Context, hash string) (string, string, string, string, string, bool, time.Time, error) {
		return "", "", "", "", "", false, time.Time{}, nil
	}

	h := middleware.Session(lookup, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	// Wrap in a minimal slog-less middleware call.
	h(next).ServeHTTP(rec, req)

	if userFound {
		t.Error("expected no session user when cookie is absent")
	}
}

func TestSession_ValidCookie_SetsUser(t *testing.T) {
	var capturedUser middleware.SessionUser
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUser, _ = middleware.GetSessionUser(r)
		w.WriteHeader(http.StatusOK)
	})

	lookup := func(_ context.Context, _ string) (string, string, string, string, string, bool, time.Time, error) {
		return "some-uuid", "alice", "alice@test.com", "player", "en", true, time.Time{}, nil
	}

	h := middleware.Session(lookup, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "fake-token"})
	rec := httptest.NewRecorder()
	h(next).ServeHTTP(rec, req)

	if capturedUser.Username != "alice" {
		t.Errorf("expected username=alice, got %q", capturedUser.Username)
	}
}

func TestSession_InactiveUser_DoesNotSetUser(t *testing.T) {
	var userFound bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, userFound = middleware.GetSessionUser(r)
		w.WriteHeader(http.StatusOK)
	})

	// isActive = false
	lookup := func(_ context.Context, _ string) (string, string, string, string, string, bool, time.Time, error) {
		return "some-uuid", "banned", "banned@test.com", "player", "en", false, time.Time{}, nil
	}

	h := middleware.Session(lookup, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "fake-token"})
	rec := httptest.NewRecorder()
	h(next).ServeHTTP(rec, req)

	if userFound {
		t.Error("expected no user injected for inactive account")
	}
}
