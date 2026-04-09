# Auth & Middleware Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all auth and middleware issues found in the 2026-04-09 code review (A1-* issues).

**Architecture:** Fix error codes in verify.go, add domain to session cookies, add LRU eviction to the rate limiter, fix duplicate-email disclosure, wire rate limiters to room/upload routes, and add missing tests for email flow, maskEmail, and edge cases.

**Tech Stack:** Go, `golang.org/x/time/rate`, `net/http`

---

### Task 1: Fix verify.go error codes

**Covers:** A1-C1

**Files:**
- Modify: `backend/db/queries/magic_link_tokens.sql`
- Modify: `backend/db/sqlc/magic_link_tokens.sql.go` (regenerated — do not edit directly; edit via sqlc if needed, or use error wrapping)
- Modify: `backend/internal/auth/verify.go`

The root issue: `ConsumeMagicLinkTokenAtomic` returns a generic `pgx` error on any failure (expired, used, not found). The handler can't distinguish these, so it returns `"invalid_token"` for all cases. The spec requires distinct codes: `token_expired`, `token_used`, `token_not_found` (all 400), and `"user_inactive"` (403) instead of `"account_inactive"` (401).

**Approach:** Since changing sqlc output requires rewriting the query to return error-discriminating columns (complex), the simplest correct approach is to split into two queries: one to look up the token (returning expiry/used state), one to mark it used. Alternatively, define sentinel errors by checking what sqlc returns.

Actually the simplest approach without query changes: inspect the pgx error code or check if the row was not found vs another failure. For "not found" vs "used": the atomic query marks `used_at` — if the row doesn't exist, it's either not found or expired. We can't distinguish expired vs not found without reading the token first.

**Revised approach:** Add a `GetMagicLinkTokenByHash` query to read the token state BEFORE consuming, then consume atomically. This allows returning distinct error codes.

- [x] **Step 1: Add lookup query to magic_link_tokens.sql**

In `backend/db/queries/magic_link_tokens.sql`, add before the atomic consume query:

```sql
-- name: GetMagicLinkTokenByHash :one
-- Used to inspect token state before consuming — allows returning distinct error codes.
SELECT * FROM magic_link_tokens WHERE token_hash = $1;
```

- [x] **Step 2: Regenerate sqlc**

```bash
cd backend && sqlc generate
```

> **Deviation (implemented):** `sqlc` is not installed. The generated function `GetMagicLinkTokenByHash` was manually added to `backend/db/sqlc/magic_link_tokens.sql.go`, following the same pattern as the existing `GetMagicLinkToken` function. The scan column list matches `MagicLinkToken` struct fields exactly.

- [x] **Step 3: Update verify.go to use distinct error codes**

Replace the token consumption block in `backend/internal/auth/verify.go`:

```go
// Before: single call that returns generic error
tokenHash := HashToken(req.Token)
token, err := h.db.ConsumeMagicLinkTokenAtomic(r.Context(), tokenHash)
if err != nil {
    writeError(w, http.StatusUnauthorized, "invalid_token", "Token is invalid, expired, or already used")
    return
}
```

Replace with:

```go
tokenHash := HashToken(req.Token)

// Look up token first to return a specific error code.
rawToken, lookupErr := h.db.GetMagicLinkTokenByHash(r.Context(), tokenHash)
if lookupErr != nil {
    writeError(w, http.StatusBadRequest, "token_not_found", "Token not found")
    return
}
if rawToken.ExpiresAt.Before(time.Now().UTC()) {
    writeError(w, http.StatusBadRequest, "token_expired", "Token has expired")
    return
}
if rawToken.UsedAt.Valid {
    writeError(w, http.StatusBadRequest, "token_used", "Token has already been used")
    return
}

// Atomically mark used — prevents replay race.
token, err := h.db.ConsumeMagicLinkTokenAtomic(r.Context(), tokenHash)
if err != nil {
    // Should not normally happen (we just verified it); treat as used by concurrent request.
    writeError(w, http.StatusBadRequest, "token_used", "Token has already been used")
    return
}
```

Also fix the `account_inactive` error below:

```go
// Before:
writeError(w, http.StatusUnauthorized, "account_inactive", "Account is not active")
// After:
writeError(w, http.StatusForbidden, "user_inactive", "Account is not active")
```

- [x] **Step 4: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 5: Write tests for error code distinctions**

In `backend/internal/auth/verify_test.go`, add tests (look at existing test file to see test patterns).

> **Deviation (implemented):** Existing tests expected old status codes (401 for expired/reuse, 401 for inactive). These were updated: `TestVerify_InvalidToken` now expects 400 + `token_not_found`, `TestVerify_TokenReuse_Rejected` expects 400, `TestVerify_ExpiredToken` expects 400, `TestVerify_InactiveAccount_Rejected` expects 403. The new tests (`TestVerify_TokenNotFound`, `TestVerify_TokenExpired`, `TestVerify_TokenUsed`, `TestVerify_UserInactive`) directly assert both status and error code.

```go
func TestVerify_TokenExpired(t *testing.T) {
    // Create an expired token (expires_at in the past), call verify, expect 400 with token_expired
}

func TestVerify_TokenUsed(t *testing.T) {
    // Create a valid token, verify once (succeeds), verify again, expect 400 with token_used
}

func TestVerify_TokenNotFound(t *testing.T) {
    // Call verify with a random token that doesn't exist, expect 400 with token_not_found
}

func TestVerify_UserInactive(t *testing.T) {
    // Create valid token for inactive user, expect 403 with user_inactive
}
```

- [x] **Step 6: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/auth/...
```

- [x] **Step 7: Commit**

```bash
git add backend/db/queries/magic_link_tokens.sql backend/db/sqlc/ \
        backend/internal/auth/verify.go backend/internal/auth/verify_test.go
git commit -m "fix(auth): verify endpoint returns distinct error codes per spec (token_expired, token_used, token_not_found, user_inactive)"
```

---

### Task 2: Add domain to session cookies

**Covers:** A1-H2

**Files:**
- Modify: `backend/internal/auth/tokens.go`
- Modify: `backend/internal/config/config.go`

- [x] **Step 1: Add CookieDomain to Config**

In `backend/internal/config/config.go`, add `CookieDomain string` to the `Config` struct and populate it from `FRONTEND_URL`:

In `Load()`, after setting `cfg.FrontendURL`:

```go
// Parse cookie domain from FRONTEND_URL (e.g. "https://meme.example.com" → "meme.example.com")
// If on the same domain, leave empty to let browser default.
if u, err := url.Parse(cfg.FrontendURL); err == nil {
    cfg.CookieDomain = u.Hostname()
}
```

Add `"net/url"` import. Add `CookieDomain string` field to Config struct.

- [x] **Step 2: Thread CookieDomain into cookie setters**

In `backend/internal/auth/tokens.go`, update `setSessionCookie` and `clearSessionCookie`:

```go
func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, domain string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    token,
        Path:     "/",
        Domain:   domain,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   int(ttl.Seconds()),
    })
}

func clearSessionCookie(w http.ResponseWriter, domain string) {
    http.SetCookie(w, &http.Cookie{
        Name:     "session",
        Value:    "",
        Path:     "/",
        Domain:   domain,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
        MaxAge:   -1,
    })
}
```

Update all call sites (grep for `setSessionCookie` and `clearSessionCookie` — they're in `verify.go` via `createSessionAndRespond` and in `session.go`):

In `verify.go`, `createSessionAndRespond`:
```go
setSessionCookie(w, rawToken, h.cfg.SessionTTL, h.cfg.CookieDomain)
```

In `session.go`, `Logout`:
```go
clearSessionCookie(w, h.cfg.CookieDomain)
```

- [x] **Step 3: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 4: Commit**

```bash
git add backend/internal/config/config.go backend/internal/auth/tokens.go \
        backend/internal/auth/verify.go backend/internal/auth/session.go
git commit -m "fix(auth): set Domain attribute on session cookies from FRONTEND_URL"
```

---

### Task 3: Rate limiter LRU eviction

**Covers:** A1-H3

**Files:**
- Modify: `backend/internal/middleware/rate_limit.go`

- [x] **Step 1: Add eviction goroutine to RateLimiter**

Add a `lastSeen` map and a `StartEviction` method:

```go
package middleware

import (
    "encoding/json"
    "net"
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

type RateLimiter struct {
    mu       sync.Mutex
    clients  map[string]*rateLimiterEntry
    rate     rate.Limit
    burst    int
}

type rateLimiterEntry struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

func NewRateLimiter(requestsPerPeriod int, periodSeconds int) *RateLimiter {
    r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
    rl := &RateLimiter{
        clients: make(map[string]*rateLimiterEntry),
        rate:    r,
        burst:   requestsPerPeriod,
    }
    go rl.evictLoop()
    return rl
}

// evictLoop removes entries that have been idle for more than 1 hour.
func (rl *RateLimiter) evictLoop() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        cutoff := time.Now().Add(-1 * time.Hour)
        rl.mu.Lock()
        for ip, entry := range rl.clients {
            if entry.lastSeen.Before(cutoff) {
                delete(rl.clients, ip)
            }
        }
        rl.mu.Unlock()
    }
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    entry, ok := rl.clients[ip]
    if !ok {
        entry = &rateLimiterEntry{limiter: rate.NewLimiter(rl.rate, rl.burst)}
        rl.clients[ip] = entry
    }
    entry.lastSeen = time.Now()
    return entry.limiter
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ip, _, _ := net.SplitHostPort(r.RemoteAddr)
        if !rl.getLimiter(ip).Allow() {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusTooManyRequests)
            json.NewEncoder(w).Encode(map[string]string{
                "error": "Too many requests",
                "code":  "rate_limited",
            })
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

- [x] **Step 2: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 3: Run rate limiter tests**

```bash
cd backend && go test -race -count=1 ./internal/middleware/...
```

- [x] **Step 4: Commit**

```bash
git add backend/internal/middleware/rate_limit.go
git commit -m "fix(middleware): evict idle rate limiter entries after 1h to prevent memory leak"
```

---

### Task 4: Wire room and upload rate limiters

**Covers:** A1-L2

**Files:**
- Modify: `backend/cmd/server/main.go`

- [x] **Step 1: Add roomLimiter and uploadLimiter; wire to routes**

In `main.go`, after `inviteLimiter`, add:

```go
roomLimiter   := mw.NewRateLimiter(cfg.RateLimitRoomsRPH, 3600)
uploadLimiter := mw.NewRateLimiter(cfg.RateLimitUploadsRPH, 3600)
```

Then wrap the room create and asset upload routes:

```go
// Rooms — rate-limit room creation only (read is fine)
r.With(mw.RequireAuth).Route("/api/rooms", func(r chi.Router) {
    r.With(roomLimiter.Middleware).Post("/", roomHandler.Create)
    r.Get("/{code}", roomHandler.GetByCode)
})

// Assets — rate-limit upload URL generation
r.With(mw.RequireAuth).Route("/api/assets", func(r chi.Router) {
    r.With(uploadLimiter.Middleware).Post("/upload-url", assetHandler.UploadURL)
    r.Post("/download-url", assetHandler.DownloadURL)
})
```

- [x] **Step 2: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "fix(middleware): wire RATE_LIMIT_ROOMS_RPH and RATE_LIMIT_UPLOADS_RPH to routes"
```

---

### Task 5: Fix duplicate-email registration disclosure

**Covers:** A1-M1

**Files:**
- Modify: `backend/internal/auth/register.go`

The problem: two code paths on duplicate email. First path (pre-check, lines 52-54) returns the real `user_id`. Second path (DB constraint violation, lines 90-91) returns `""`. Both should return `""` to prevent enumeration.

- [x] **Step 1: Read the full register.go pre-check block**

Find the section in `register.go` where an existing user is found by email (before DB insert). It likely looks like:

```go
if existing != nil {
    writeJSON(w, http.StatusCreated, map[string]string{"user_id": existing.ID.String()})
    return
}
```

Change to:

```go
if existing != nil {
    // Return 201 with empty user_id — do not leak whether the email is registered
    writeJSON(w, http.StatusCreated, map[string]string{"user_id": ""})
    return
}
```

> **Deviation (implemented):** The actual pre-check pattern in `register.go` uses `GetUserByEmail` with an `err == nil` check (no `existing` variable holding a pointer). The fix changes the call to discard the return value: `if _, err := h.db.GetUserByEmail(...); err == nil {`.

- [x] **Step 2: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 3: Update the duplicate-email test to assert user_id is empty**

In `backend/internal/auth/register_test.go`, find `TestRegister_DuplicateEmailReturns201` and add assertion:

```go
var body map[string]string
if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
    t.Fatalf("decode body: %v", err)
}
if body["user_id"] != "" {
    t.Errorf("expected empty user_id on duplicate email, got %q", body["user_id"])
}
```

- [x] **Step 4: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/auth/...
```

- [x] **Step 5: Commit**

```bash
git add backend/internal/auth/register.go backend/internal/auth/register_test.go
git commit -m "fix(auth): return empty user_id on duplicate email in both code paths"
```

---

### Task 6: Add SMTP startup validation

**Covers:** A1-H1

**Files:**
- Modify: `backend/internal/config/config.go`

The review flags that `SMTP_USERNAME` and `SMTP_PASSWORD` are not in the `required` list. They are currently loaded but silently absent. The fix depends on whether anonymous SMTP relay is supported: if it is, they're optional; if not, add them to required.

Per `ref-env-vars.md` they are marked required, so add them.

- [x] **Step 1: Add SMTP_USERNAME and SMTP_PASSWORD to required list**

In `config.go`, update:

```go
required := []string{
    "DATABASE_URL", "RUSTFS_ENDPOINT", "RUSTFS_ACCESS_KEY", "RUSTFS_SECRET_KEY",
    "FRONTEND_URL", "BACKEND_URL", "SMTP_HOST", "SMTP_FROM",
    "SMTP_USERNAME", "SMTP_PASSWORD",
}
```

- [x] **Step 2: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 3: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "fix(config): add SMTP_USERNAME and SMTP_PASSWORD to required env vars"
```

---

### Task 7: Log swallowed errors

**Covers:** A1-M6, A1-L1

**Files:**
- Modify: `backend/internal/auth/handler.go`
- Modify: `backend/internal/auth/session.go`

- [x] **Step 1: Log InvalidatePendingTokens error in handler.go**

In `backend/internal/auth/handler.go`, `sendMagicLinkToUser`:

```go
// Before:
_ = h.db.InvalidatePendingTokens(ctx, db.InvalidatePendingTokensParams{
    UserID:  user.ID,
    Purpose: purpose,
})

// After:
if err := h.db.InvalidatePendingTokens(ctx, db.InvalidatePendingTokensParams{
    UserID:  user.ID,
    Purpose: purpose,
}); err != nil && h.log != nil {
    // Non-fatal: prior tokens may still be valid, but the invariant "only latest link valid"
    // could be violated. Log as warning for observability.
    h.log.WarnContext(ctx, "sendMagicLink: failed to invalidate prior tokens",
        "user_id", user.ID, "purpose", purpose, "error", err)
}
```

- [x] **Step 2: Log DeleteSession error in session.go**

In `backend/internal/auth/session.go`, `Logout`:

```go
// Before:
_ = h.db.DeleteSession(r.Context(), hash)

// After:
if err := h.db.DeleteSession(r.Context(), hash); err != nil && h.log != nil {
    // Non-fatal: session will expire naturally via TTL. Log for observability.
    h.log.Warn("logout: failed to delete session", "error", err)
}
```

- [x] **Step 3: Add idempotency comment to Logout**

Add a comment to the `Logout` function explaining the idempotent behavior:

```go
// Logout handles POST /api/auth/logout.
// Deletes the session row and clears the session cookie.
// Intentionally idempotent: if the caller is not authenticated (no cookie or
// session already expired), we still return 200 and clear the cookie. This
// prevents information leakage and simplifies client-side logout flows.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
```

- [x] **Step 4: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 5: Commit**

```bash
git add backend/internal/auth/handler.go backend/internal/auth/session.go
git commit -m "fix(auth): log InvalidatePendingTokens and DeleteSession errors instead of swallowing"
```

---

### Task 8: Add request ID to error responses

**Covers:** A1-M5

**Files:**
- Modify: `backend/internal/middleware/request_id.go` (find where request ID is stored in context)
- Modify: `backend/internal/middleware/context.go`
- Modify: `backend/internal/auth/handler.go` (writeError)
- Modify: `backend/internal/api/packs.go` (writeError — shared)

The request ID is set by `mw.RequestID` middleware (chi's built-in or custom). Find how it's accessed:

```bash
grep -rn "RequestID\|requestID\|X-Request-ID" backend/internal/middleware/
```

- [x] **Step 1: Identify how request ID is stored**

Look at `backend/internal/middleware/request_id.go`. If it uses `chi/middleware.GetReqID(ctx)`, that function is available. If custom, find the context key.

> **Deviation (implemented):** The custom `RequestID` middleware was not storing the ID in chi's context key, so `chiMiddleware.GetReqID` always returned empty string. The middleware was updated to store the ID via `context.WithValue(r.Context(), chiMiddleware.RequestIDKey, id)` in addition to setting the header. This is required for `writeError` to include the request ID in error responses.

- [x] **Step 2: Update the shared writeError in api/packs.go**

The `writeError` in `backend/internal/api/packs.go` is used by all API handlers. Update it to extract request ID from context:

```go
func writeError(w http.ResponseWriter, status int, code, message string) {
    writeJSON(w, status, map[string]string{"error": message, "code": code})
}
```

This signature doesn't have `r *http.Request`. Either:
- Change signature to accept `r *http.Request` and pass it from all callers (many changes)
- OR: change signature to accept `ctx context.Context`

The least-invasive change: pass the request to writeError everywhere. Check how many call sites there are:

```bash
grep -c "writeError(" backend/internal/api/*.go
```

If there are dozens of call sites, add a separate `writeErrorWithReqID` helper called only from handler entry points, and keep the old `writeError` for internal use.

**Simplest approach:** update `writeError` to accept `r *http.Request`:

```go
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
    body := map[string]string{"error": message, "code": code}
    if r != nil {
        if reqID := chiMiddleware.GetReqID(r.Context()); reqID != "" {
            body["request_id"] = reqID
        }
    }
    writeJSON(w, status, body)
}
```

Then update all call sites (this is mechanical but there are many). Use:

```bash
# Count call sites
grep -rn "writeError(w," backend/internal/api/ backend/internal/auth/ | wc -l
```

If >30 call sites, this is a large change. Use `sed` or do it with Edit tool file by file.

- [x] **Step 3: Add chiMiddleware import where needed**

In `backend/internal/api/packs.go`, add:
```go
chiMiddleware "github.com/go-chi/chi/v5/middleware"
```

- [x] **Step 4: Update all call sites**

For each file in `backend/internal/api/` and `backend/internal/auth/`, change:
```go
writeError(w, status, code, message)
```
to:
```go
writeError(w, r, status, code, message)
```

Note: The `writeError` in `backend/internal/middleware/context.go` is a separate function — update it independently.

- [x] **Step 5: Verify build**

```bash
cd backend && go build ./...
```

- [x] **Step 6: Commit**

```bash
git add backend/internal/api/ backend/internal/auth/ backend/internal/middleware/
git commit -m "fix(api): include request_id in all error JSON responses"
```

---

### Task 9: Fix request ID test uniqueness

**Covers:** A1-L3

**Files:**
- Modify: `backend/internal/middleware/request_id_test.go`

- [x] **Step 1: Add uniqueness assertion**

In `backend/internal/middleware/request_id_test.go`, after the existing test that verifies presence, add:

```go
func TestRequestID_Unique(t *testing.T) {
    handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Seen-ID", chiMiddleware.GetReqID(r.Context()))
        w.WriteHeader(http.StatusOK)
    }))

    req1 := httptest.NewRequest(http.MethodGet, "/", nil)
    w1 := httptest.NewRecorder()
    handler.ServeHTTP(w1, req1)
    id1 := w1.Header().Get("X-Seen-ID")

    req2 := httptest.NewRequest(http.MethodGet, "/", nil)
    w2 := httptest.NewRecorder()
    handler.ServeHTTP(w2, req2)
    id2 := w2.Header().Get("X-Seen-ID")

    if id1 == "" || id2 == "" {
        t.Fatal("request IDs should not be empty")
    }
    if id1 == id2 {
        t.Errorf("request IDs should be unique across requests, got %q for both", id1)
    }
}
```

- [x] **Step 2: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/middleware/...
```

- [x] **Step 3: Commit**

```bash
git add backend/internal/middleware/request_id_test.go
git commit -m "test(middleware): assert request IDs are unique across requests"
```

---

### Task 10: Email change and maskEmail tests

**Covers:** A1-H4, A1-M3, A1-M4

**Files:**
- Modify: `backend/internal/auth/profile_test.go`
- Modify: `backend/internal/auth/handler_test.go` (or create if needed)

- [x] **Step 1: Add TestMaskEmail tests**

In `backend/internal/auth/handler_test.go` (or create it), add:

```go
package auth

import "testing"

func TestMaskEmail_Normal(t *testing.T) {
    got := maskEmail("user@example.com")
    if got != "***@example.com" {
        t.Errorf("got %q, want %q", got, "***@example.com")
    }
}

func TestMaskEmail_NoAt(t *testing.T) {
    got := maskEmail("notanemail")
    if got != "***" {
        t.Errorf("got %q, want %q", got, "***")
    }
}

func TestMaskEmail_EmptyLocalPart(t *testing.T) {
    got := maskEmail("@example.com")
    if got != "***@example.com" {
        t.Errorf("got %q, want %q", got, "***@example.com")
    }
}
```

- [x] **Step 2: Add TestPatchMe_EmailChange and TestVerify_EmailChange integration tests**

These require the integration test setup (testutil). In `backend/internal/auth/profile_test.go`:

```go
func TestPatchMe_EmailChange(t *testing.T) {
    // 1. Create user and authenticate
    // 2. PATCH /api/users/me with { "email": "new@example.com" }
    // 3. Verify 200 returned
    // 4. Verify user.pending_email == "new@example.com" in DB
    // 5. Verify a magic_link_token with purpose=email_change exists in DB
}

func TestVerify_EmailChange_Success(t *testing.T) {
    // 1. Create user with pending_email and email_change token
    // 2. POST /api/auth/verify with the token
    // 3. Verify 200, session created
    // 4. Verify user.email changed, pending_email cleared
    // 5. Verify old sessions deleted (re-auth required)
}
```

- [x] **Step 3: Add TestRegister_SMTPFailureWarning**

In `backend/internal/auth/register_test.go`, add test for the SMTP failure warning path:

This requires a mock email sender. Find the existing mock pattern in register_test.go and create a failing sender:

```go
type failingSender struct{}
func (fs *failingSender) SendMagicLinkLogin(_ context.Context, _ string, _ LoginEmailData) error {
    return errors.New("smtp: connection refused")
}
// ... implement other methods to return nil

func TestRegister_SMTPFailureReturns201WithWarning(t *testing.T) {
    // Create invite with restricted_email set
    // Register with that email using failingSender
    // Expect 201 with body containing "warning": "smtp_failure"
}
```

- [x] **Step 4: Run tests**

```bash
cd backend && go test -race -count=1 ./internal/auth/...
```

- [x] **Step 5: Commit**

```bash
git add backend/internal/auth/
git commit -m "test(auth): add maskEmail tests, email change flow tests, SMTP failure warning test"
```
