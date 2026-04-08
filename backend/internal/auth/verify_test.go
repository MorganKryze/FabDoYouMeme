// backend/internal/auth/verify_test.go

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
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func seedMagicToken(t *testing.T, q *db.Queries, userID uuid.UUID, purpose string) string {
	t.Helper()
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	_, err := q.CreateMagicLinkToken(context.Background(), db.CreateMagicLinkTokenParams{
		UserID:    userID,
		TokenHash: hash,
		Purpose:   purpose,
		ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("seedMagicToken: %v", err)
	}
	return raw
}

func TestVerify_InvalidToken(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"token":"bad_token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}

func TestVerify_Success_Login(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "dave", "dave@test.com")
	rawToken := seedMagicToken(t, q, user.ID, "login")

	body := `{"token":"` + rawToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var sessionCookieFound bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" && c.Value != "" {
			sessionCookieFound = true
		}
	}
	if !sessionCookieFound {
		t.Error("expected session cookie to be set")
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["user_id"] == "" {
		t.Error("expected user_id in response")
	}
}

func TestVerify_TokenReuse_Rejected(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "eve", "eve@test.com")
	rawToken := seedMagicToken(t, q, user.ID, "login")

	doVerify := func() int {
		body := `{"token":"` + rawToken + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()
		h.Verify(rec, req)
		return rec.Code
	}

	if code := doVerify(); code != http.StatusOK {
		t.Fatalf("first verify: want 200, got %d", code)
	}
	if code := doVerify(); code != http.StatusUnauthorized {
		t.Errorf("second verify (reuse): want 401, got %d", code)
	}
}

func TestVerify_InactiveAccount_Rejected(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "inactive_user", "inactive@test.com")

	// Deactivate the user directly via DB (SetUserActive with false)
	_, err := q.SetUserActive(context.Background(), db.SetUserActiveParams{
		ID:       user.ID,
		IsActive: false,
	})
	if err != nil {
		t.Fatalf("deactivate user: %v", err)
	}

	rawToken := seedMagicToken(t, q, user.ID, "login")
	body := `{"token":"` + rawToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for inactive account, got %d", rec.Code)
	}
}

func TestVerify_EmptyToken_BadRequest(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"token":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for empty token, got %d", rec.Code)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "expuser_"+testutil.SeedName(t), "expuser_"+testutil.SeedName(t)+"@test.com")

	// Seed a token that's already expired.
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	if _, err := q.CreateMagicLinkToken(context.Background(), db.CreateMagicLinkTokenParams{
		UserID:    user.ID,
		TokenHash: hash,
		Purpose:   "login",
		ExpiresAt: time.Now().UTC().Add(-time.Minute), // past expiry
	}); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}

	body := `{"token":"` + raw + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for expired token, got %d", rec.Code)
	}
}
