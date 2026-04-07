# Backend — Auth (Sessions, Magic Links, Invites) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all auth HTTP handlers: register, magic-link, verify, logout, me, user profile (username/email change, history, GDPR export), admin hard-delete, and first-boot admin bootstrap.

**Architecture:** All handlers are methods on a `Handler` struct that takes injected DB, pool, config, email sender, and logger. The `EmailSender` interface is defined here so Phase 5 can implement it. A `sendMagicLinkToUser` helper is shared across register and magic-link flows. Hard-delete runs a 5-step transaction using sqlc's `WithTx`.

**Tech Stack:** Go stdlib, pgx/v5, sqlc-generated queries, `github.com/go-chi/chi/v5`, `github.com/google/uuid`.

**Prerequisite:** Phase 3 complete (middleware + config + testutil exist).

---

### Task 1: Token helpers + EmailSender interface + Handler struct

**Files:**

- Create: `backend/internal/auth/tokens.go`
- Create: `backend/internal/auth/email.go`
- Create: `backend/internal/auth/handler.go`
- Create: `backend/internal/auth/tokens_test.go`

- [ ] **Step 1: Write the token helpers test**

```go
// backend/internal/auth/tokens_test.go
package auth_test

import (
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
)

func TestHashToken_Deterministic(t *testing.T) {
	h1 := auth.HashToken("my-token")
	h2 := auth.HashToken("my-token")
	if h1 != h2 {
		t.Errorf("hash is not deterministic: %s != %s", h1, h2)
	}
}

func TestHashToken_Different(t *testing.T) {
	if auth.HashToken("token-a") == auth.HashToken("token-b") {
		t.Error("different tokens should produce different hashes")
	}
}

func TestGenerateRawToken_UniqueAndLen(t *testing.T) {
	t1, err := auth.GenerateRawToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t2, _ := auth.GenerateRawToken()
	if t1 == t2 {
		t.Error("two generated tokens should not be equal")
	}
	// 32 bytes hex-encoded = 64 chars
	if len(t1) != 64 {
		t.Errorf("expected 64 char token, got %d", len(t1))
	}
}
```

- [ ] **Step 2: Run test to see it fail**

```bash
cd backend && go test ./internal/auth/... -run TestHash -run TestGenerate
```

Expected: compile error (package does not exist yet).

- [ ] **Step 3: Write `tokens.go`**

```go
// backend/internal/auth/tokens.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// GenerateRawToken returns a random 32-byte hex-encoded token.
// Exported so tests can call it directly to construct test tokens.
func GenerateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of raw. Only the hash is stored in DB.
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ttl.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
```

- [ ] **Step 4: Write `email.go` — EmailSender interface**

```go
// backend/internal/auth/email.go
package auth

import "context"

// LoginEmailData is passed to SendMagicLinkLogin.
type LoginEmailData struct {
	Username      string
	MagicLinkURL  string
	FrontendURL   string
	ExpiryMinutes int
}

// EmailChangeData is passed to SendMagicLinkEmailChange.
type EmailChangeData struct {
	Username      string
	MagicLinkURL  string
	FrontendURL   string
	ExpiryMinutes int
}

// EmailChangedNotificationData is passed to SendEmailChangedNotification.
type EmailChangedNotificationData struct {
	Username       string
	NewEmailMasked string
	FrontendURL    string
}

// EmailSender is implemented by the email package (Phase 5).
// A stub is used in tests.
type EmailSender interface {
	SendMagicLinkLogin(ctx context.Context, to string, data LoginEmailData) error
	SendMagicLinkEmailChange(ctx context.Context, to string, data EmailChangeData) error
	SendEmailChangedNotification(ctx context.Context, to string, data EmailChangedNotificationData) error
}
```

- [ ] **Step 5: Write `handler.go` — Handler struct + helpers**

```go
// backend/internal/auth/handler.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

// Handler holds injected dependencies for all auth routes.
type Handler struct {
	db    *db.Queries
	pool  *pgxpool.Pool
	cfg   *config.Config
	email EmailSender
	log   *slog.Logger
}

// New creates a Handler. Call SeedAdmin separately after New.
func New(pool *pgxpool.Pool, cfg *config.Config, email EmailSender, log *slog.Logger) *Handler {
	return &Handler{
		db:    db.New(pool),
		pool:  pool,
		cfg:   cfg,
		email: email,
		log:   log,
	}
}

// SessionLookupFn satisfies middleware.SessionLookupFn. It validates the session
// and renews its TTL on every authenticated request so active users stay logged in.
func (h *Handler) SessionLookupFn(ctx context.Context, tokenHash string) (string, string, string, string, bool, error) {
	row, err := h.db.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return "", "", "", "", false, err
	}
	// Renew TTL for active sessions; log on failure but do not block the request
	newExpiry := time.Now().Add(h.cfg.SessionTTL)
	if _, err := h.db.RenewSession(ctx, db.RenewSessionParams{
		ID:        row.ID,
		ExpiresAt: newExpiry,
	}); err != nil && h.log != nil {
		h.log.WarnContext(ctx, "session renewal failed", "err", err)
	}
	return row.UID.String(), row.Username, row.Email, row.Role, row.IsActive, nil
}

// sendMagicLinkToUser invalidates prior tokens of the same purpose,
// generates a new one, persists its hash, and emails the raw token to the user.
func (h *Handler) sendMagicLinkToUser(ctx context.Context, user db.User, purpose string) error {
	// Invalidate any existing unused tokens for this purpose
	_ = h.db.InvalidatePendingTokens(ctx, db.InvalidatePendingTokensParams{
		UserID:  user.ID,
		Purpose: purpose,
	})

	rawToken, err := GenerateRawToken()
	if err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	tokenHash := HashToken(rawToken)
	expiresAt := time.Now().Add(h.cfg.MagicLinkTTL)

	if _, err := h.db.CreateMagicLinkToken(ctx, db.CreateMagicLinkTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Purpose:   purpose,
		ExpiresAt: expiresAt,
	}); err != nil {
		return fmt.Errorf("store token: %w", err)
	}

	magicURL := h.cfg.MagicLinkBaseURL + "/auth/verify?token=" + rawToken
	expiryMinutes := int(h.cfg.MagicLinkTTL.Minutes())

	if purpose == "email_change" {
		to := ""
		if user.PendingEmail != nil {
			to = *user.PendingEmail
		}
		return h.email.SendMagicLinkEmailChange(ctx, to, EmailChangeData{
			Username:      user.Username,
			MagicLinkURL:  magicURL,
			FrontendURL:   h.cfg.FrontendURL,
			ExpiryMinutes: expiryMinutes,
		})
	}
	return h.email.SendMagicLinkLogin(ctx, user.Email, LoginEmailData{
		Username:      user.Username,
		MagicLinkURL:  magicURL,
		FrontendURL:   h.cfg.FrontendURL,
		ExpiryMinutes: expiryMinutes,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

// maskEmail replaces all characters before '@' with "***".
// E.g. "alice@example.com" → "***@example.com".
func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at < 0 {
		return "***"
	}
	return "***" + email[at:]
}
```

- [ ] **Step 6: Run token tests**

```bash
cd backend && go test ./internal/auth/... -run TestHash -run TestGenerate -v
```

Expected: all `PASS`.

---

### Task 2: Register handler

**Files:**

- Create: `backend/internal/auth/register.go`
- Create: `backend/internal/auth/register_test.go`

- [ ] **Step 1: Write the test**

```go
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
	_, q := testutil.NewDB(t)
	cfg := &config.Config{
		FrontendURL:    "http://localhost:3000",
		MagicLinkBaseURL: "http://localhost:3000",
		MagicLinkTTL:   15 * time.Minute,
		SessionTTL:     720 * time.Hour,
	}
	// Handler needs pool for transactions; re-open pool in testutil
	pool, _ := testutil.NewDB(t)
	h := auth.New(pool, cfg, &stubEmail{}, nil) // logger nil is ok in tests
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
	// First registration
	body := `{"invite_token":"TEST_INVITE","username":"bob","email":"bob@test.com","consent":true,"age_affirmation":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first register: want 201, got %d", rec.Code)
	}
	// Second attempt with same email — must still return 201 (no enumeration)
	body2 := `{"invite_token":"TEST_INVITE","username":"bob2","email":"bob@test.com","consent":true,"age_affirmation":true}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body2))
	rec2 := httptest.NewRecorder()
	h.Register(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Errorf("duplicate email: want 201 (no enumeration), got %d", rec2.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (expected: compile error)**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestRegister -v
```

Expected: compile error (`Register` method does not exist yet).

- [ ] **Step 3: Implement `register.go`**

```go
// backend/internal/auth/register.go
package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/google/uuid"
)

type registerRequest struct {
	InviteToken    string `json:"invite_token"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Consent        bool   `json:"consent"`
	AgeAffirmation bool   `json:"age_affirmation"`
}

// Register handles POST /api/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	if !req.Consent {
		writeError(w, http.StatusBadRequest, "consent_required", "You must accept the terms to register")
		return
	}
	if !req.AgeAffirmation {
		writeError(w, http.StatusBadRequest, "age_affirmation_required", "You must confirm you are at least 16 years old")
		return
	}

	// Validate invite — generic error on any failure (invite enumeration defense)
	invite, err := h.db.GetInviteByToken(r.Context(), req.InviteToken)
	if err != nil || req.InviteToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	// Check restricted email match — fail generically
	if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" &&
		*invite.RestrictedEmail != req.Email {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	// Email already registered → return 201 silently (no enumeration)
	if existing, err := h.db.GetUserByEmail(r.Context(), req.Email); err == nil && existing.ID != uuid.Nil {
		writeJSON(w, http.StatusCreated, map[string]string{"user_id": existing.ID.String()})
		return
	}

	// Atomically consume invite (checks expires_at and max_uses in SQL)
	consumed, err := h.db.ConsumeInvite(r.Context(), invite.ID)
	if err != nil || consumed.ID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_invite", "Invalid or expired invite token")
		return
	}

	// Create user
	var invitedBy *uuid.UUID
	if invite.CreatedBy != nil {
		invitedBy = invite.CreatedBy
	}
	newUser, err := h.db.CreateUser(r.Context(), db.CreateUserParams{
		Username:  req.Username,
		Email:     req.Email,
		Role:      "player",
		IsActive:  true,
		InvitedBy: invitedBy,
		ConsentAt: time.Now(),
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") && strings.Contains(err.Error(), "username") {
			writeError(w, http.StatusConflict, "username_taken", "That username is already taken")
			return
		}
		// Duplicate email via race condition → still 201
		if strings.Contains(err.Error(), "unique") && strings.Contains(err.Error(), "email") {
			writeJSON(w, http.StatusCreated, map[string]string{"user_id": ""})
			return
		}
		if h.log != nil {
			h.log.Error("register: create user", "error", err)
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Registration failed")
		return
	}

	// Streamlined onboarding: auto-send magic link for restricted_email invites
	resp := map[string]string{"user_id": newUser.ID.String()}
	if invite.RestrictedEmail != nil && *invite.RestrictedEmail != "" {
		if sendErr := h.sendMagicLinkToUser(r.Context(), newUser, "login"); sendErr != nil {
			if h.log != nil {
				h.log.Error("register: auto magic link", "error", sendErr)
			}
			resp["warning"] = "smtp_failure"
		}
	}

	writeJSON(w, http.StatusCreated, resp)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestRegister -v
```

Expected: all `PASS`. (Requires `DATABASE_URL` set and migrations applied.)

---

### Task 3: Magic link handler

**Files:**

- Create: `backend/internal/auth/magic_link.go`
- Create: `backend/internal/auth/magic_link_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/auth/magic_link_test.go
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
		ConsentAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("seedUser %s: %v", email, err)
	}
	return user
}

func TestMagicLink_AlwaysReturns200(t *testing.T) {
	h, _ := newTestHandler(t)
	// Non-existent email must still return 200 (no enumeration)
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestMagicLink -v
```

Expected: compile error (`MagicLink` method does not exist yet).

- [ ] **Step 3: Implement `magic_link.go`**

```go
// backend/internal/auth/magic_link.go
package auth

import (
	"encoding/json"
	"net/http"
)

type magicLinkRequest struct {
	Email string `json:"email"`
}

// MagicLink handles POST /api/auth/magic-link.
// Always returns 200 — never reveals whether the email is registered.
func (h *Handler) MagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		// Still 200 — never hint that anything is wrong
		w.WriteHeader(http.StatusOK)
		return
	}

	user, err := h.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil || !user.IsActive {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.sendMagicLinkToUser(r.Context(), user, "login"); err != nil {
		if h.log != nil {
			h.log.Error("magic link send failed", "error", err, "user_id", user.ID)
		}
		// Still 200 — no enumeration
	}

	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestMagicLink -v
```

Expected: all `PASS`.

---

### Task 4: Verify handler

**Files:**

- Create: `backend/internal/auth/verify.go`
- Create: `backend/internal/auth/verify_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/auth/verify_test.go
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
)

func seedMagicToken(t *testing.T, q *db.Queries, userID uuid.UUID, purpose string) string {
	t.Helper()
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	_, err := q.CreateMagicLinkToken(context.Background(), db.CreateMagicLinkTokenParams{
		UserID:    userID,
		TokenHash: hash,
		Purpose:   purpose,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("seedMagicToken: %v", err)
	}
	return raw // raw token to submit to verify endpoint
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
	// Session cookie must be set
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestVerify -v
```

Expected: compile error (`Verify` method does not exist yet).

- [ ] **Step 3: Implement `verify.go`**

```go
// backend/internal/auth/verify.go
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

type verifyRequest struct {
	Token string `json:"token"`
}

// Verify handles POST /api/auth/verify.
// It consumes a magic link token (one-time use) and creates a session.
// For email_change tokens, it also swaps the email and invalidates all sessions.
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	tokenHash := HashToken(req.Token)
	token, err := h.db.GetMagicLinkToken(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "Token is invalid, expired, or already used")
		return
	}

	// Mark token used (one-time use)
	if _, err := h.db.ConsumeMagicLinkToken(r.Context(), token.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Token verification failed")
		return
	}

	// Re-fetch user to check is_active (deactivated accounts cannot log in even with a valid token)
	user, err := h.db.GetUserByID(r.Context(), token.UserID)
	if err != nil || !user.IsActive {
		writeError(w, http.StatusUnauthorized, "account_inactive", "Account is not active")
		return
	}

	switch token.Purpose {
	case "login":
		h.createSessionAndRespond(w, r, user)

	case "email_change":
		oldEmail := user.Email

		// Swap email ← pending_email, clear pending_email
		updated, err := h.db.ConfirmEmailChange(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Email change failed")
			return
		}

		// Invalidate all existing sessions (security: user must re-authenticate)
		if err := h.db.DeleteAllUserSessions(r.Context(), user.ID); err != nil && h.log != nil {
			h.log.Error("verify: delete sessions", "error", err)
		}

		// Notify old address (non-fatal)
		if err := h.email.SendEmailChangedNotification(r.Context(), oldEmail, EmailChangedNotificationData{
			Username:       updated.Username,
			NewEmailMasked: maskEmail(updated.Email),
			FrontendURL:    h.cfg.FrontendURL,
		}); err != nil && h.log != nil {
			h.log.Error("verify: email changed notification", "error", err)
		}

		// Issue new session with updated user
		h.createSessionAndRespond(w, r, updated)

	default:
		writeError(w, http.StatusBadRequest, "invalid_token", "Unknown token purpose")
	}
}

func (h *Handler) createSessionAndRespond(w http.ResponseWriter, r *http.Request, user db.User) {
	rawToken, err := GenerateRawToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}
	sessionHash := HashToken(rawToken)
	expiresAt := time.Now().Add(h.cfg.SessionTTL)

	if _, err := h.db.CreateSession(r.Context(), db.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: sessionHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Session creation failed")
		return
	}

	setSessionCookie(w, rawToken, h.cfg.SessionTTL)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": user.ID.String()})
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestVerify -v
```

Expected: all `PASS`.

---

### Task 5: Logout + Me handlers

**Files:**

- Create: `backend/internal/auth/session.go`
- Create: `backend/internal/auth/session_test.go`

- [ ] **Step 1: Write the tests**

```go
// backend/internal/auth/session_test.go
//go:build integration

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func seedSession(t *testing.T, q *db.Queries, userID interface{ String() string }, ttl time.Duration) (rawToken string) {
	t.Helper()
	raw, _ := auth.GenerateRawToken()
	hash := auth.HashToken(raw)
	uid, err := uuid.Parse(userID.String())
	if err != nil {
		t.Fatalf("seedSession: parse uuid: %v", err)
	}
	if _, err := q.CreateSession(context.Background(), db.CreateSessionParams{
		UserID:    uid,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(ttl),
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
	// Inject session user into context as middleware would do
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
	// Cookie should be cleared
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
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["username"] != "grace" {
		t.Errorf("want grace, got %s", resp["username"])
	}
}
```

Note: the test above references `middleware.SessionUserContextKey`. The Phase 3 plan exported `GetSessionUser` but used an unexported context key. You need to export the key or add a package-level exported var. Add to `backend/internal/middleware/context.go`:

```go
// Export the key so test packages can inject a fake session.
var SessionUserContextKey = sessionUserKey
```

- [ ] **Step 2: Export context key in Phase 3 middleware**

Edit `backend/internal/middleware/context.go` and add after the `const sessionUserKey` declaration:

```go
// SessionUserContextKey is exported for use in tests that need to inject a session user directly.
var SessionUserContextKey = sessionUserKey
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestLogout -run TestMe -v
```

Expected: compile error (`Logout` and `Me` methods do not exist yet).

- [ ] **Step 4: Implement `session.go`**

```go
// backend/internal/auth/session.go
package auth

import (
	"net/http"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// Logout handles POST /api/auth/logout.
// Deletes the session row and clears the session cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		hash := HashToken(cookie.Value)
		_ = h.db.DeleteSession(r.Context(), hash)
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusOK)
}

// Me handles GET /api/auth/me.
// Returns the current user's identity fields.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":       u.UserID,
		"username": u.Username,
		"email":    u.Email,
		"role":     u.Role,
	})
}
```

- [ ] **Step 5: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestLogout -run TestMe -v
```

Expected: all `PASS`.

---

### Task 6: User profile handlers

**Files:**

- Create: `backend/internal/auth/profile.go`
- Create: `backend/internal/auth/profile_test.go`

- [ ] **Step 1: Write the tests**

```go
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
	json.NewDecoder(rec.Body).Decode(&resp)
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
	json.NewDecoder(rec.Body).Decode(&resp)
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
	json.NewDecoder(rec.Body).Decode(&resp)
	userField, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user object in export, got %T", resp["user"])
	}
	if userField["email"] != user.Email {
		t.Errorf("export email mismatch: got %v", userField["email"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestPatchMe -run TestGetHistory -run TestGetExport -v
```

Expected: compile error (`PatchMe`, `GetHistory`, `GetExport` do not exist yet).

- [ ] **Step 3: Implement `profile.go`**

```go
// backend/internal/auth/profile.go
package auth

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

type patchMeRequest struct {
	Username *string `json:"username,omitempty"`
	Email    *string `json:"email,omitempty"`
}

// PatchMe handles PATCH /api/users/me.
// Accepts { username } or { email } — not both in one request.
func (h *Handler) PatchMe(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	var req patchMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid JSON body")
		return
	}

	if req.Username != nil {
		updated, err := h.db.UpdateUserUsername(r.Context(), db.UpdateUserUsernameParams{
			ID:       userID,
			Username: *req.Username,
		})
		if err != nil {
			if strings.Contains(err.Error(), "unique") {
				writeError(w, http.StatusConflict, "username_taken", "That username is already taken")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"id":       updated.ID.String(),
			"username": updated.Username,
			"email":    updated.Email,
		})
		return
	}

	if req.Email != nil {
		// Store pending email and send verification link to NEW address
		updated, err := h.db.SetPendingEmail(r.Context(), db.SetPendingEmailParams{
			ID:           userID,
			PendingEmail: req.Email,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		if sendErr := h.sendMagicLinkToUser(r.Context(), updated, "email_change"); sendErr != nil {
			if h.log != nil {
				h.log.Error("patch me: email change magic link", "error", sendErr)
			}
			writeError(w, http.StatusInternalServerError, "smtp_failure", "Failed to send verification email")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Verification link sent to new address",
		})
		return
	}

	writeError(w, http.StatusBadRequest, "bad_request", "Provide username or email to update")
}

type historyRoom struct {
	Code          string     `json:"code"`
	GameTypeSlug  string     `json:"game_type_slug"`
	PackName      string     `json:"pack_name"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	Score         int32      `json:"score"`
	Rank          int        `json:"rank"`
	PlayerCount   int        `json:"player_count"`
}

// GetHistory handles GET /api/users/me/history.
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	limit := int32(50)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if lv, err := strconv.Atoi(l); err == nil && lv > 0 && lv <= 100 {
			limit = int32(lv)
		}
	}
	if cursor := r.URL.Query().Get("after"); cursor != "" {
		if ov, err := strconv.Atoi(cursor); err == nil && ov > 0 {
			offset = int32(ov)
		}
	}

	rows, err := h.db.GetUserGameHistory(r.Context(), db.GetUserGameHistoryParams{
		UserID: userID,
		Lim:    limit,
		Off:    offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load history")
		return
	}

	rooms := make([]historyRoom, 0, len(rows))
	for _, row := range rows {
		players, err := h.db.ListRoomPlayers(r.Context(), row.RoomID)
		playerCount := len(players)
		rank := 1
		if err == nil {
			for _, p := range players {
				if p.Score > row.Score {
					rank++
				}
			}
		}
		rooms = append(rooms, historyRoom{
			Code:         row.Code,
			GameTypeSlug: row.GameTypeSlug,
			PackName:     row.PackName,
			StartedAt:    row.StartedAt,
			FinishedAt:   row.FinishedAt,
			Score:        row.Score,
			Rank:         rank,
			PlayerCount:  playerCount,
		})
	}

	var nextCursor *string
	if int32(len(rows)) == limit {
		next := strconv.Itoa(int(offset + limit))
		nextCursor = &next
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"rooms":       rooms,
		"next_cursor": nextCursor,
	})
}

type exportUser struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	ConsentAt time.Time `json:"consent_at"`
	CreatedAt time.Time `json:"created_at"`
}

type exportSubmission struct {
	RoundID      string          `json:"round_id"`
	RoomCode     string          `json:"room_code"`
	GameTypeSlug string          `json:"game_type_slug"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
}

type exportGameHistory struct {
	RoomCode     string     `json:"room_code"`
	GameTypeSlug string     `json:"game_type_slug"`
	PackName     string     `json:"pack_name"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Score        int32      `json:"score"`
}

// GetExport handles GET /api/users/me/export (GDPR Art. 20).
func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	userID, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Invalid session")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load user")
		return
	}

	historyRows, _ := h.db.GetUserGameHistory(r.Context(), db.GetUserGameHistoryParams{
		UserID: userID,
		Lim:    1000,
		Off:    0,
	})
	gameHistory := make([]exportGameHistory, 0, len(historyRows))
	for _, row := range historyRows {
		gameHistory = append(gameHistory, exportGameHistory{
			RoomCode:     row.Code,
			GameTypeSlug: row.GameTypeSlug,
			PackName:     row.PackName,
			StartedAt:    row.StartedAt,
			FinishedAt:   row.FinishedAt,
			Score:        row.Score,
		})
	}

	submissionRows, _ := h.db.GetUserSubmissions(r.Context(), userID)
	submissions := make([]exportSubmission, 0, len(submissionRows))
	for _, s := range submissionRows {
		submissions = append(submissions, exportSubmission{
			RoundID:      s.RoundID.String(),
			RoomCode:     s.RoomCode,
			GameTypeSlug: s.GameTypeSlug,
			Payload:      s.Payload,
			CreatedAt:    s.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"exported_at": time.Now().UTC(),
		"user": exportUser{
			ID:        user.ID.String(),
			Username:  user.Username,
			Email:     user.Email,
			Role:      user.Role,
			ConsentAt: user.ConsentAt,
			CreatedAt: user.CreatedAt,
		},
		"game_history": gameHistory,
		"submissions":  submissions,
	})
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestPatchMe -run TestGetHistory -run TestGetExport -v
```

Expected: all `PASS`.

---

### Task 7: Admin hard-delete handler

**Files:**

- Create: `backend/internal/auth/admin.go`
- Create: `backend/internal/auth/admin_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/auth/admin_test.go
//go:build integration

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func newChiRequest(method, path, paramKey, paramVal string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramVal)
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestDeleteUser_HardDeletes(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_del", "admin_del@test.com")
	target := seedUser(t, q, "victim", "victim@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+target.ID.String(), "id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}

	// User must no longer exist
	if _, err := q.GetUserByID(context.Background(), target.ID); err == nil {
		t.Error("expected user to be deleted")
	}
}

func TestDeleteUser_CannotDeleteSentinel(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_s", "admin_s@test.com")
	sentinelID := "00000000-0000-0000-0000-000000000001"

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+sentinelID, "id", sentinelID)
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

func TestDeleteUser_AuditLogCreated(t *testing.T) {
	h, q := newTestHandler(t)
	admin := seedUser(t, q, "admin_al", "admin_al@test.com")
	target := seedUser(t, q, "victim2", "victim2@test.com")

	req := newChiRequest(http.MethodDelete, "/api/admin/users/"+target.ID.String(), "id", target.ID.String())
	req = requestWithUser(req, admin.ID.String(), admin.Username, admin.Email, "admin")
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}

	// Check audit log was written
	logs, err := q.ListAuditLogs(context.Background(), db.ListAuditLogsParams{Lim: 10, Off: 0})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	var found bool
	for _, l := range logs {
		if l.Action == "hard_delete_user" && l.Resource == "user:"+target.ID.String() {
			found = true
		}
	}
	if !found {
		t.Error("expected audit log entry for hard_delete_user")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestDeleteUser -v
```

Expected: compile error (`DeleteUser` method does not exist yet).

- [ ] **Step 3: Implement `admin.go`**

```go
// backend/internal/auth/admin.go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

const sentinelUserID = "00000000-0000-0000-0000-000000000001"

// DeleteUser handles DELETE /api/admin/users/:id.
// Runs the 5-step hard-delete protocol in a single transaction.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	adminUser, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	targetID := chi.URLParam(r, "id")
	targetUUID, err := uuid.Parse(targetID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Guard: cannot delete the sentinel user
	if targetID == sentinelUserID {
		writeError(w, http.StatusConflict, "cannot_delete_sentinel", "The sentinel user cannot be deleted")
		return
	}

	// Guard: cannot delete yourself
	if targetID == adminUser.UserID {
		writeError(w, http.StatusConflict, "cannot_delete_self", "Cannot delete your own account")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Transaction failed")
		return
	}
	defer tx.Rollback(r.Context())

	q := h.db.WithTx(tx)

	// Step 1: Capture PII before deletion (audit trail requires it)
	target, err := q.GetUserByID(r.Context(), targetUUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	// Step 2: Write audit log (PII recorded before delete so log is meaningful)
	adminUUID, _ := uuid.Parse(adminUser.UserID)
	changes, _ := json.Marshal(map[string]string{
		"username": target.Username,
		"email":    target.Email,
	})
	if _, err := q.CreateAuditLog(r.Context(), db.CreateAuditLogParams{
		AdminID:  &adminUUID,
		Action:   "hard_delete_user",
		Resource: fmt.Sprintf("user:%s", targetID),
		Changes:  json.RawMessage(changes),
	}); err != nil {
		if h.log != nil {
			h.log.Error("delete user: audit log", "error", err)
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Audit log failed")
		return
	}

	// Step 3: Replace submissions with sentinel (preserve game history integrity)
	if err := q.UpdateSubmissionsSentinel(r.Context(), targetUUID); err != nil && h.log != nil {
		h.log.Error("delete user: submissions sentinel", "error", err)
	}

	// Step 4: Replace votes with sentinel (preserve vote history integrity)
	if err := q.UpdateVotesSentinel(r.Context(), targetUUID); err != nil && h.log != nil {
		h.log.Error("delete user: votes sentinel", "error", err)
	}

	// Step 5: Delete user — cascades: sessions, magic_link_tokens, room_players
	//           sets NULL on: invites.created_by, audit_logs.admin_id, game_packs.owner_id
	if err := q.HardDeleteUser(r.Context(), targetUUID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Transaction commit failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestDeleteUser -v
```

Expected: all `PASS`.

---

### Task 8: First-boot admin bootstrap

**Files:**

- Create: `backend/internal/auth/bootstrap.go`
- Create: `backend/internal/auth/bootstrap_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/auth/bootstrap_test.go
//go:build integration

package auth_test

import (
	"context"
	"testing"
)

func TestSeedAdmin_CreatesAdminUser(t *testing.T) {
	h, q := newTestHandler(t)
	h.SetSeedAdminEmail("bootstrap_admin@test.com")

	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("SeedAdmin: %v", err)
	}

	user, err := q.GetUserByEmail(context.Background(), "bootstrap_admin@test.com")
	if err != nil {
		t.Fatalf("user not found: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role=admin, got %s", user.Role)
	}
}

func TestSeedAdmin_Idempotent(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetSeedAdminEmail("idempotent_admin@test.com")

	// Run twice — must not error or create duplicates
	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("first SeedAdmin: %v", err)
	}
	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("second SeedAdmin: %v", err)
	}
}

func TestSeedAdmin_NoEmail_IsNoop(t *testing.T) {
	h, _ := newTestHandler(t)
	h.SetSeedAdminEmail("") // empty — must be no-op

	if err := h.SeedAdmin(context.Background()); err != nil {
		t.Fatalf("SeedAdmin with no email: %v", err)
	}
}
```

Note: `SetSeedAdminEmail` is a test-only helper that overrides `cfg.SeedAdminEmail`. Add to `handler.go`:

```go
// SetSeedAdminEmail overrides the config field — for use in tests only.
func (h *Handler) SetSeedAdminEmail(email string) {
	h.cfg.SeedAdminEmail = email
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd backend && go test -tags integration ./internal/auth/... -run TestSeedAdmin -v
```

Expected: compile error (`SeedAdmin` method does not exist yet).

- [ ] **Step 3: Implement `bootstrap.go`**

```go
// backend/internal/auth/bootstrap.go
package auth

import (
	"context"
	"fmt"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// SeedAdmin creates the first admin user if SEED_ADMIN_EMAIL is set and no user
// with that email exists yet. Idempotent: safe to call on every startup.
func (h *Handler) SeedAdmin(ctx context.Context) error {
	if h.cfg.SeedAdminEmail == "" {
		return nil
	}

	// Check if the user already exists (idempotency)
	existing, err := h.db.GetUserByEmail(ctx, h.cfg.SeedAdminEmail)
	if err == nil {
		// User exists — do not modify; log if not admin
		if existing.Role != "admin" && h.log != nil {
			h.log.Warn("SEED_ADMIN_EMAIL is set but the account is not admin role; skipping bootstrap",
				"email", h.cfg.SeedAdminEmail)
		}
		return nil
	}

	// Create admin user with legitimate-interest consent_at (Art. 6(1)(f))
	consentAt := time.Now()
	admin, err := h.db.CreateUser(ctx, db.CreateUserParams{
		Username:  "admin",
		Email:     h.cfg.SeedAdminEmail,
		Role:      "admin",
		IsActive:  true,
		InvitedBy: nil,
		ConsentAt: consentAt,
	})
	if err != nil {
		return fmt.Errorf("seed admin: create user: %w", err)
	}

	if h.log != nil {
		h.log.Info("admin user created via SEED_ADMIN_EMAIL", "email", h.cfg.SeedAdminEmail)
	}

	// Send magic link so admin can log in immediately
	if err := h.sendMagicLinkToUser(ctx, admin, "login"); err != nil {
		return fmt.Errorf("seed admin: send magic link: %w", err)
	}

	if h.log != nil {
		h.log.Info("admin magic link sent", "email", h.cfg.SeedAdminEmail)
	}
	return nil
}
```

- [ ] **Step 4: Add `SetSeedAdminEmail` to `handler.go`** (as shown in Step 1)

Edit `backend/internal/auth/handler.go` and append:

```go
// SetSeedAdminEmail overrides cfg.SeedAdminEmail — for use in tests only.
func (h *Handler) SetSeedAdminEmail(email string) {
	h.cfg.SeedAdminEmail = email
}
```

- [ ] **Step 5: Run all auth tests**

```bash
cd backend && go test -tags integration ./internal/auth/... -v
```

Expected: all tests `PASS`.

- [ ] **Step 6: Build check**

```bash
cd backend && go build ./...
```

Expected: no errors.

---

### Verification

```bash
# Unit tests (no DB required)
cd backend && go test ./internal/auth/... -run TestHash -run TestGenerate -v

# Integration tests (requires DATABASE_URL + migrations applied)
cd backend && go test -tags integration ./internal/auth/... -v

# Full build
cd backend && go build ./...
```

Expected: all tests pass, `go build` succeeds with no errors.

Mark phase 4 complete in `docs/implementation-status.md`.
