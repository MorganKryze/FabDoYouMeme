
package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

type countingEmail struct {
	sends int
}

func (s *countingEmail) SendMagicLinkLogin(_ context.Context, _ string, _ auth.LoginEmailData) error {
	s.sends++
	return nil
}
func (s *countingEmail) SendMagicLinkEmailChange(_ context.Context, _ string, _ auth.EmailChangeData) error {
	return nil
}
func (s *countingEmail) SendEmailChangedNotification(_ context.Context, _ string, _ auth.EmailChangedNotificationData) error {
	return nil
}

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

func TestMagicLink_MalformedJSON_Returns200(t *testing.T) {
	// MagicLink intentionally returns 200 for all inputs — including bad JSON —
	// to prevent email enumeration. Any parsing error is treated silently.
	h, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString("{bad json"))
	rec := httptest.NewRecorder()
	h.MagicLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200 (anti-enumeration) for malformed JSON, got %d", rec.Code)
	}
}

func TestMagicLink_Cooldown(t *testing.T) {
	pool := testutil.Pool()
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	sender := &countingEmail{}
	clk := clock.NewFake(time.Now())
	h := auth.New(pool, cfg, sender, nil, clk)
	q := db.New(pool)

	seedUser(t, q, "cooldownuser", "cooldown@cooldown.test")

	send := func() {
		body := `{"email":"cooldown@cooldown.test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.MagicLink(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rec.Code)
		}
	}

	// First send — should go through.
	send()
	if sender.sends != 1 {
		t.Errorf("after first send: want 1 email sent, got %d", sender.sends)
	}

	// Second send immediately — cooldown active, email suppressed.
	send()
	if sender.sends != 1 {
		t.Errorf("during cooldown: want 1 email sent, got %d", sender.sends)
	}

	// Advance past cooldown.
	clk.Advance(61 * time.Second)

	// Third send — cooldown expired, email goes through.
	send()
	if sender.sends != 2 {
		t.Errorf("after cooldown: want 2 emails sent, got %d", sender.sends)
	}
}
