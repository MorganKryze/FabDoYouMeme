// backend/internal/auth/register_test.go
//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// stubEmail is a no-op EmailSender for tests.
type stubEmail struct{}

func (s *stubEmail) SendMagicLinkLogin(_ context.Context, _ string, _ auth.LoginEmailData) error {
	return nil
}
func (s *stubEmail) SendMagicLinkEmailChange(_ context.Context, _ string, _ auth.EmailChangeData) error {
	return nil
}
func (s *stubEmail) SendEmailChangedNotification(_ context.Context, _ string, _ auth.EmailChangedNotificationData) error {
	return nil
}

func newTestHandler(t *testing.T) (*auth.Handler, *db.Queries) {
	t.Helper()
	pool, q := testutil.NewDB(t)
	cfg := &config.Config{
		FrontendURL:      "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:     15 * time.Minute,
		SessionTTL:       720 * time.Hour,
	}
	h := auth.New(pool, cfg, &stubEmail{}, nil)
	return h, q
}

func seedInvite(t *testing.T, q *db.Queries) db.Invite {
	t.Helper()
	invite, err := q.CreateInvite(context.Background(), db.CreateInviteParams{
		Token:   "TEST_INVITE",
		MaxUses: 10,
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
	body := `{"invite_token":"NO_SUCH","username":"u","email":"u@example.com","consent":true,"age_affirmation":true}`
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
	seedInvite(t, q)
	body := `{"invite_token":"TEST_INVITE","username":"alice","email":"alice@test.com","consent":true,"age_affirmation":true}`
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

func TestRegister_DuplicateEmailReturns201(t *testing.T) {
	h, q := newTestHandler(t)
	seedInvite(t, q)
	body := `{"invite_token":"TEST_INVITE","username":"bob","email":"bob@test.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d", rec.Code)
	}
	body2 := `{"invite_token":"TEST_INVITE","username":"bob2","email":"bob@test.com","consent":true,"age_affirmation":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body2))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Errorf("duplicate email: want 201 (no enumeration), got %d", rec2.Code)
	}
}
