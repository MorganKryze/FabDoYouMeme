
package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// countingEmail is a test double for the email sender. MagicLink dispatches
// the SMTP call in a detached goroutine (see sendMagicLinkToUserAsync), so
// SendMagicLinkLogin is invoked from a goroutine the test does not own;
// the mutex keeps reads and writes of `sends` race-free under -race.
type countingEmail struct {
	mu    sync.Mutex
	sends int
}

func (s *countingEmail) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sends
}

func (s *countingEmail) SendMagicLinkLogin(_ context.Context, _, _ string, _ auth.LoginEmailData) error {
	s.mu.Lock()
	s.sends++
	s.mu.Unlock()
	return nil
}
func (s *countingEmail) SendMagicLinkEmailChange(_ context.Context, _, _ string, _ auth.EmailChangeData) error {
	return nil
}
func (s *countingEmail) SendEmailChangedNotification(_ context.Context, _, _ string, _ auth.EmailChangedNotificationData) error {
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
		Locale:    "en",
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

	// MagicLink dispatches SMTP in a goroutine (sendMagicLinkToUserAsync),
	// so the handler returns before the counter is incremented. waitFor
	// polls with a bounded deadline; the timeout is generous for CI load.
	waitFor := func(want int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if sender.count() >= want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	// First send — should go through.
	send()
	waitFor(1)
	if got := sender.count(); got != 1 {
		t.Fatalf("after first send: want 1 email sent, got %d", got)
	}

	// Second send immediately — cooldown active, no goroutine is dispatched
	// on this path, so any increment would be a bug. Sleep long enough that
	// a spuriously dispatched goroutine would have completed, then assert.
	send()
	time.Sleep(50 * time.Millisecond)
	if got := sender.count(); got != 1 {
		t.Fatalf("during cooldown: want 1 email sent, got %d", got)
	}

	// Advance past cooldown.
	clk.Advance(61 * time.Second)

	// Third send — cooldown expired, email goes through.
	send()
	waitFor(2)
	if got := sender.count(); got != 2 {
		t.Fatalf("after cooldown: want 2 emails sent, got %d", got)
	}
}
