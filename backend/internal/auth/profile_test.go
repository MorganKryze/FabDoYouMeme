// backend/internal/auth/profile_test.go
//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func requestWithUser(r *http.Request, userID, username, email, role string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), middleware.SessionUserContextKey, middleware.SessionUser{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
	}))
}

func TestPatchMe_UpdateUsername(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "helen", "helen@test.com")

	body := `{"username":"helen2"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(body))
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.PatchMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

func TestPatchMe_UsernameConflict(t *testing.T) {
	h, q := newTestHandler(t)
	seedUser(t, q, "ivan", "ivan@test.com")
	user2 := seedUser(t, q, "judy", "judy@test.com")

	body := `{"username":"ivan"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/users/me", bytes.NewBufferString(body))
	req = requestWithUser(req, user2.ID.String(), user2.Username, user2.Email, user2.Role)
	rec := httptest.NewRecorder()
	h.PatchMe(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["code"] != "username_taken" {
		t.Errorf("want username_taken, got %s", resp["code"])
	}
}

func TestGetHistory_Empty(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "kate", "kate@test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/history", nil)
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetHistory(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rooms, ok := resp["rooms"]
	if !ok {
		t.Error("response missing rooms key")
	}
	if rooms == nil {
		t.Error("rooms should be an empty array, not null")
	}
}

func TestGetExport_ContainsUser(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "laura", "laura@test.com")

	req := httptest.NewRequest(http.MethodGet, "/api/users/me/export", nil)
	req = requestWithUser(req, user.ID.String(), user.Username, user.Email, user.Role)
	rec := httptest.NewRecorder()
	h.GetExport(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	userField, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in export, got %T", resp["user"])
	}
	if userField["email"] != user.Email {
		t.Errorf("export email mismatch: got %v", userField["email"])
	}
}
