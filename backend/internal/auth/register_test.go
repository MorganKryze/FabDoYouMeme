// backend/internal/auth/register_test.go

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// stubEmail is a no-op EmailSender for tests.
type stubEmail struct{}

func (s *stubEmail) SendMagicLinkLogin(_ context.Context, _, _ string, _ auth.LoginEmailData) error {
	return nil
}
func (s *stubEmail) SendMagicLinkEmailChange(_ context.Context, _, _ string, _ auth.EmailChangeData) error {
	return nil
}
func (s *stubEmail) SendEmailChangedNotification(_ context.Context, _, _ string, _ auth.EmailChangedNotificationData) error {
	return nil
}

// failingSender is an EmailSender that always returns an error — used to test SMTP failure paths.
type failingSender struct{}

func (s *failingSender) SendMagicLinkLogin(_ context.Context, _, _ string, _ auth.LoginEmailData) error {
	return fmt.Errorf("smtp: connection refused")
}
func (s *failingSender) SendMagicLinkEmailChange(_ context.Context, _, _ string, _ auth.EmailChangeData) error {
	return fmt.Errorf("smtp: connection refused")
}
func (s *failingSender) SendEmailChangedNotification(_ context.Context, _, _ string, _ auth.EmailChangedNotificationData) error {
	return fmt.Errorf("smtp: connection refused")
}

func newTestHandler(t *testing.T) (*auth.Handler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	h := auth.New(pool, cfg, &stubEmail{}, nil, clock.Real{})
	return h, db.New(pool)
}

// shortUser returns a username derived from t.Name() but capped to fit
// inside the 30-character limit enforced by auth.ValidateUsername (P2.9).
// The prefix lets a single test mint distinguishable usernames (bob_, bob2_)
// without colliding across tests sharing the same DB.
func shortUser(t *testing.T, prefix string) string {
	t.Helper()
	slug := testutil.SeedName(t)
	max := 30 - len(prefix)
	if len(slug) > max {
		slug = slug[:max]
	}
	return prefix + slug
}

func seedInvite(t *testing.T, q *db.Queries) db.Invite {
	t.Helper()
	// Use a unique token per test to avoid collisions when multiple tests in this
	// package each seed an invite against the same shared database.
	token := "INV_" + testutil.SeedName(t)
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:   token,
		MaxUses: 10,
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("seedInvite: %v", err)
	}
	return invite
}

func TestRegister_ConsentRequired(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"invite_token":"x","username":"u","email":"u@example.com","consent":false,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "consent_required" {
		t.Errorf("want consent_required, got %s", resp["code"])
	}
}

func TestRegister_AgeAffirmationRequired(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"invite_token":"x","username":"u","email":"u@example.com","consent":true,"age_affirmation":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "age_affirmation_required" {
		t.Errorf("want age_affirmation_required, got %s", resp["code"])
	}
}

func TestRegister_InvalidInvite(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"invite_token":"NO_SUCH","username":"user","email":"u@example.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_invite" {
		t.Errorf("want invalid_invite, got %s", resp["code"])
	}
}

func TestRegister_Success(t *testing.T) {
	h, q := newTestHandler(t)
	invite := seedInvite(t, q)
	body := `{"invite_token":"` + invite.Token + `","username":"alice_` + testutil.SeedName(t) + `","email":"alice_` + testutil.SeedName(t) + `@test.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["user_id"] == "" {
		t.Error("expected user_id in response")
	}
	if _, err := uuid.Parse(resp["user_id"]); err != nil {
		t.Errorf("user_id is not a valid UUID: %s", resp["user_id"])
	}
}

// TestRegister_PropagatesLocaleToEmailSender is the end-to-end guard that
// proves a register with locale=fr causes the SMTP sender to receive "fr".
// Regressions that silently drop locale anywhere on the chain
// (handler → deliverMagicLink → EmailSender) surface here immediately.
func TestRegister_PropagatesLocaleToEmailSender(t *testing.T) {
	pool := testutil.Pool()
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	fake := testutil.NewFakeEmail()
	h := auth.New(pool, cfg, fake, nil, clock.Real{})
	q := db.New(pool)

	// Use a restricted-email invite so register auto-fires the magic link.
	slug := testutil.SeedName(t)
	email := "fr_" + slug + "@test.com"
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:           "FR_" + slug,
		MaxUses:         1,
		RestrictedEmail: &email,
		Locale:          "fr",
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	body := `{"invite_token":"` + invite.Token + `","username":"` + shortUser(t, "fr_") +
		`","email":"` + email + `","consent":true,"age_affirmation":true,"locale":"fr"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if len(fake.Sent) != 1 {
		t.Fatalf("expected exactly 1 email, got %d", len(fake.Sent))
	}
	if got := fake.Sent[0].Locale; got != "fr" {
		t.Errorf("SendMagicLinkLogin locale: want %q, got %q", "fr", got)
	}
}

func TestRegister_DuplicateEmailReturns201(t *testing.T) {
	h, q := newTestHandler(t)
	invite := seedInvite(t, q)
	slug := testutil.SeedName(t)
	body := `{"invite_token":"` + invite.Token + `","username":"` + shortUser(t, "bob_") + `","email":"bob_` + slug + `@test.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	body2 := `{"invite_token":"` + invite.Token + `","username":"` + shortUser(t, "bob2_") + `","email":"bob_` + slug + `@test.com","consent":true,"age_affirmation":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body2))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Errorf("duplicate email: want 201 (no enumeration), got %d", rec2.Code)
	}
	var dupResp map[string]string
	if err := json.NewDecoder(rec2.Body).Decode(&dupResp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if dupResp["user_id"] != "" {
		t.Errorf("expected empty user_id on duplicate email, got %q", dupResp["user_id"])
	}
}

func TestRegister_EmailMismatch(t *testing.T) {
	h, q := newTestHandler(t)
	// Create an invite restricted to a specific email.
	restrictedEmail := "allowed@test.com"
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:           "RESTRICTED_" + testutil.SeedName(t),
		MaxUses:         5,
		RestrictedEmail: &restrictedEmail,
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create restricted invite: %v", err)
	}

	body := `{"invite_token":"` + invite.Token + `","username":"mismatch_user","email":"wrong@test.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	// The handler returns invalid_invite (not email_mismatch) to avoid leaking
	// whether an invite's email restriction exists.
	if resp["code"] != "invalid_invite" {
		t.Errorf("want invalid_invite, got %s", resp["code"])
	}
}

func TestRegister_InviteExhausted(t *testing.T) {
	h, q := newTestHandler(t)
	// Create an invite with max_uses=1.
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:   "EXHAUST_" + testutil.SeedName(t),
		MaxUses: 1,
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	slug := testutil.SeedName(t)
	register := func(username, email string) int {
		body := `{"invite_token":"` + invite.Token + `","username":"` + username + `","email":"` + email + `","consent":true,"age_affirmation":true}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.Register(rec, req)
		return rec.Code
	}

	if code := register(shortUser(t, "u1_"), "user1_"+slug+"@test.com"); code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d", code)
	}
	// Second registration exhausts the invite.
	rec2 := httptest.NewRecorder()
	body2 := `{"invite_token":"` + invite.Token + `","username":"` + shortUser(t, "u2_") + `","email":"user2_` + slug + `@test.com","consent":true,"age_affirmation":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body2))
	h.Register(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("want 400 for exhausted invite, got %d", rec2.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec2.Body).Decode(&resp)
	if resp["code"] != "invalid_invite" {
		t.Errorf("want invalid_invite, got %s", resp["code"])
	}
}

func TestRegister_SMTPFailureReturns201WithWarning(t *testing.T) {
	pool := testutil.Pool()
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	h := auth.New(pool, cfg, &failingSender{}, nil, clock.Real{})
	q := db.New(pool)

	slug := testutil.SeedName(t)
	restrictedEmail := "smtp_fail_" + slug + "@test.com"
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:           "SMTPFAIL_" + slug,
		MaxUses:         1,
		RestrictedEmail: &restrictedEmail,
		Locale:    "en",
	})
	if err != nil {
		t.Fatalf("create restricted invite: %v", err)
	}

	body := fmt.Sprintf(`{"invite_token":%q,"username":%q,"email":%q,"consent":true,"age_affirmation":true}`,
		invite.Token, shortUser(t, "sf_"), restrictedEmail)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("want 201 even on SMTP failure, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp["warning"] != "smtp_failure" {
		t.Errorf("expected warning=smtp_failure, got %q", resp["warning"])
	}
}

func TestRegister_UsernameTaken(t *testing.T) {
	h, q := newTestHandler(t)
	slug := testutil.SeedName(t)

	inv1 := db.CreateInviteParams{Token: "INV1_" + slug, MaxUses: 5,
		Locale:    "en",}
	invite1, _ := q.CreateInvite(context.Background(), inv1)
	inv2 := db.CreateInviteParams{Token: "INV2_" + slug, MaxUses: 5,
		Locale:    "en",}
	invite2, _ := q.CreateInvite(context.Background(), inv2)

	// Both registrations reuse the same username — point of the test is to
	// prove the second one hits 409 username_taken.
	takenUsername := shortUser(t, "tk_")
	body1 := `{"invite_token":"` + invite1.Token + `","username":"` + takenUsername + `","email":"first_` + slug + `@test.com","consent":true,"age_affirmation":true}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body1))
	rec1 := httptest.NewRecorder()
	h.Register(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d — body: %s", rec1.Code, rec1.Body.String())
	}

	// Second registration with same username → 409 username_taken.
	body2 := `{"invite_token":"` + invite2.Token + `","username":"` + takenUsername + `","email":"second_` + slug + `@test.com","consent":true,"age_affirmation":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body2))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("want 409 for duplicate username, got %d — body: %s", rec2.Code, rec2.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec2.Body).Decode(&resp)
	if resp["code"] != "username_taken" {
		t.Errorf("want username_taken, got %s", resp["code"])
	}
}
