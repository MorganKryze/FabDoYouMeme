//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

func seedUser(t *testing.T, q *db.Queries, username, email string) db.User {
	t.Helper()
	user, err := q.CreateUser(context.Background(), db.CreateUserParams{
		Username:  username,
		Email:     email,
		Role:      "player",
		IsActive:  true,
		ConsentAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("seedUser %s: %v", email, err)
	}
	return user
}

func TestMagicLink_AlwaysReturns200(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"email":"nobody@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.MagicLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestMagicLink_ExistingUser(t *testing.T) {
	h, q := newTestHandler(t)
	seedUser(t, q, "carol", "carol@test.com")
	body := `{"email":"carol@test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.MagicLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}
