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
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "token_not_found" {
		t.Errorf("want token_not_found, got %s", resp["code"])
	}
}

func TestVerify_TokenNotFound(t *testing.T) {
	h, _ := newTestHandler(t)
	body := `{"token":"nonexistent_token_abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "token_not_found" {
		t.Errorf("want token_not_found, got %s", resp["code"])
	}
}

func TestVerify_TokenExpired(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "expuser2_"+testutil.SeedName(t), "expuser2_"+testutil.SeedName(t)+"@test.com")

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
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for expired token, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "token_expired" {
		t.Errorf("want token_expired, got %s", resp["code"])
	}
}

func TestVerify_TokenUsed(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "usedtok_"+testutil.SeedName(t), "usedtok_"+testutil.SeedName(t)+"@test.com")
	rawToken := seedMagicToken(t, q, user.ID, "login")

	// First verify — should succeed
	body := `{"token":"` + rawToken + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Verify(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first verify: want 200, got %d", rec.Code)
	}

	// Second verify — token already used
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewBufferString(body))
	rec2 := httptest.NewRecorder()
	h.Verify(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("want 400 for used token, got %d", rec2.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec2.Body).Decode(&resp)
	if resp["code"] != "token_used" {
		t.Errorf("want token_used, got %s", resp["code"])
	}
}

func TestVerify_UserInactive(t *testing.T) {
	h, q := newTestHandler(t)
	user := seedUser(t, q, "inact2_"+testutil.SeedName(t), "inact2_"+testutil.SeedName(t)+"@test.com")

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
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for inactive account, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "user_inactive" {
		t.Errorf("want user_inactive, got %s", resp["code"])
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
	if code := doVerify(); code != http.StatusBadRequest {
		t.Errorf("second verify (reuse): want 400, got %d", code)
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
	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for inactive account, got %d", rec.Code)
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
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400 for expired token, got %d", rec.Code)
	}
}
