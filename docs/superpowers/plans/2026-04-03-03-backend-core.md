# Backend — Config + Middleware — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the config loader (env → typed struct), all HTTP middleware (auth, rate limit, structured logging, request ID), and a test helper that wires a real DB for handler tests.

**Architecture:** Config is loaded once at startup in `main.go` and passed through via function arguments (no globals). Middleware is pure `net/http` — no framework coupling. The test helper creates an isolated DB connection per test using `DATABASE_URL` env var.

**Tech Stack:** Go stdlib `log/slog`, `net/http`, `go-chi/chi/v5`, `pgx/v5`, `golang.org/x/time/rate`.

**Prerequisite:** Phase 2 complete (sqlc generated, DB running).

---

### Task 1: Config loader

**Files:**

- Create: `backend/internal/config/config.go`
- Create: `backend/internal/config/config_test.go`

- [ ] **Step 1: Write the test first**

```go
// backend/internal/config/config_test.go
package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

func TestLoad_RequiredMissing(t *testing.T) {
	os.Clearenv()
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when required vars missing")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("DATABASE_URL", "postgres://x:y@localhost/db")
	os.Setenv("RUSTFS_ENDPOINT", "http://rustfs")
	os.Setenv("RUSTFS_ACCESS_KEY", "key")
	os.Setenv("RUSTFS_SECRET_KEY", "secret")
	os.Setenv("FRONTEND_URL", "http://localhost:3000")
	os.Setenv("BACKEND_URL", "http://localhost:8080")
	os.Setenv("SMTP_HOST", "smtp.example.com")
	os.Setenv("SMTP_USERNAME", "user")
	os.Setenv("SMTP_PASSWORD", "pass")
	os.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("default port: got %s", cfg.Port)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("default session TTL: got %v", cfg.SessionTTL)
	}
	if cfg.MagicLinkTTL != 15*time.Minute {
		t.Errorf("default magic link TTL: got %v", cfg.MagicLinkTTL)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("default SMTP port: got %d", cfg.SMTPPort)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/config/... -run TestLoad
```

Expected: compile error ("config" package does not exist).

- [ ] **Step 3: Implement config loader**

```go
// backend/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL     string
	Port            string
	FrontendURL     string
	BackendURL      string
	RustFSEndpoint  string
	RustFSAccessKey string
	RustFSSecretKey string
	RustFSBucket    string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	MagicLinkTTL     time.Duration
	MagicLinkBaseURL string
	SessionTTL       time.Duration
	SessionRenewInterval time.Duration

	SeedAdminEmail string
	LogLevel       string

	ReconnectGraceWindow time.Duration
	WSRateLimit          int
	WSReadLimitBytes     int64
	WSReadDeadline       time.Duration
	WSPingInterval       time.Duration

	MaxUploadSizeBytes int64

	RateLimitAuthRPM          int
	RateLimitInviteRPH        int
	RateLimitRoomsRPH         int
	RateLimitUploadsRPH       int
	RateLimitGlobalRPM        int
}

func Load() (*Config, error) {
	required := []string{
		"DATABASE_URL", "RUSTFS_ENDPOINT", "RUSTFS_ACCESS_KEY", "RUSTFS_SECRET_KEY",
		"FRONTEND_URL", "BACKEND_URL", "SMTP_HOST", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM",
	}
	for _, k := range required {
		if os.Getenv(k) == "" {
			return nil, fmt.Errorf("required env var %s is not set", k)
		}
	}

	cfg := &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		Port:            getEnv("PORT", "8080"),
		FrontendURL:     os.Getenv("FRONTEND_URL"),
		BackendURL:      os.Getenv("BACKEND_URL"),
		RustFSEndpoint:  os.Getenv("RUSTFS_ENDPOINT"),
		RustFSAccessKey: os.Getenv("RUSTFS_ACCESS_KEY"),
		RustFSSecretKey: os.Getenv("RUSTFS_SECRET_KEY"),
		RustFSBucket:    getEnv("RUSTFS_BUCKET", "fabyoumeme-assets"),
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPUsername:    os.Getenv("SMTP_USERNAME"),
		SMTPPassword:    os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:        os.Getenv("SMTP_FROM"),
		MagicLinkBaseURL: os.Getenv("FRONTEND_URL"),
		SeedAdminEmail:  os.Getenv("SEED_ADMIN_EMAIL"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
	}

	var err error
	if cfg.SMTPPort, err = getEnvInt("SMTP_PORT", 587); err != nil {
		return nil, fmt.Errorf("SMTP_PORT: %w", err)
	}
	if cfg.MagicLinkTTL, err = getEnvDuration("MAGIC_LINK_TTL", 15*time.Minute); err != nil {
		return nil, fmt.Errorf("MAGIC_LINK_TTL: %w", err)
	}
	if cfg.SessionTTL, err = getEnvDuration("SESSION_TTL", 720*time.Hour); err != nil {
		return nil, fmt.Errorf("SESSION_TTL: %w", err)
	}
	if cfg.SessionRenewInterval, err = getEnvDuration("SESSION_RENEW_INTERVAL", 60*time.Minute); err != nil {
		return nil, fmt.Errorf("SESSION_RENEW_INTERVAL: %w", err)
	}
	if cfg.ReconnectGraceWindow, err = getEnvDuration("RECONNECT_GRACE_WINDOW", 30*time.Second); err != nil {
		return nil, fmt.Errorf("RECONNECT_GRACE_WINDOW: %w", err)
	}
	if cfg.WSReadDeadline, err = getEnvDuration("WS_READ_DEADLINE", 60*time.Second); err != nil {
		return nil, fmt.Errorf("WS_READ_DEADLINE: %w", err)
	}
	if cfg.WSPingInterval, err = getEnvDuration("WS_PING_INTERVAL", 25*time.Second); err != nil {
		return nil, fmt.Errorf("WS_PING_INTERVAL: %w", err)
	}
	if cfg.WSRateLimit, err = getEnvInt("WS_RATE_LIMIT", 20); err != nil {
		return nil, fmt.Errorf("WS_RATE_LIMIT: %w", err)
	}
	if cfg.WSReadLimitBytes, err = getEnvInt64("WS_READ_LIMIT_BYTES", 4096); err != nil {
		return nil, fmt.Errorf("WS_READ_LIMIT_BYTES: %w", err)
	}
	if cfg.MaxUploadSizeBytes, err = getEnvInt64("MAX_UPLOAD_SIZE_BYTES", 2097152); err != nil {
		return nil, fmt.Errorf("MAX_UPLOAD_SIZE_BYTES: %w", err)
	}
	if cfg.RateLimitAuthRPM, err = getEnvInt("RATE_LIMIT_AUTH_RPM", 10); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_AUTH_RPM: %w", err)
	}
	if cfg.RateLimitInviteRPH, err = getEnvInt("RATE_LIMIT_INVITE_VALIDATION_RPH", 20); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_INVITE_VALIDATION_RPH: %w", err)
	}
	if cfg.RateLimitRoomsRPH, err = getEnvInt("RATE_LIMIT_ROOMS_RPH", 10); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_ROOMS_RPH: %w", err)
	}
	if cfg.RateLimitUploadsRPH, err = getEnvInt("RATE_LIMIT_UPLOADS_RPH", 50); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_UPLOADS_RPH: %w", err)
	}
	if cfg.RateLimitGlobalRPM, err = getEnvInt("RATE_LIMIT_GLOBAL_RPM", 100); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_GLOBAL_RPM: %w", err)
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	return time.ParseDuration(v)
}
```

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/config/... -v
```

Expected: `PASS` for both tests.

---

### Task 2: Request ID middleware

**Files:**

- Create: `backend/internal/middleware/request_id.go`
- Create: `backend/internal/middleware/request_id_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/middleware/request_id_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestRequestID_SetsHeader(t *testing.T) {
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			t.Error("X-Request-ID not set on request")
		}
		w.Header().Set("X-Request-ID", id)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID not in response")
	}
}
```

- [ ] **Step 2: Implement**

```go
// backend/internal/middleware/request_id.go
package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend && go test ./internal/middleware/... -v -run TestRequestID
```

Expected: `PASS`.

---

### Task 3: Structured logging middleware

**Files:**

- Create: `backend/internal/middleware/logging.go`
- Create: `backend/internal/middleware/logging_test.go`

- [ ] **Step 1: Write the test**

```go
// backend/internal/middleware/logging_test.go
package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := middleware.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("invalid log JSON: %v", err)
	}
	if entry["msg"] != "http_request" {
		t.Errorf("expected msg=http_request, got %v", entry["msg"])
	}
	if entry["request_id"] != "req-123" {
		t.Errorf("expected request_id=req-123, got %v", entry["request_id"])
	}
}
```

- [ ] **Step 2: Implement**

```go
// backend/internal/middleware/logging.go
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			logger.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", r.Header.Get("X-Request-ID"),
			)
		})
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend && go test ./internal/middleware/... -v -run TestLogger
```

Expected: `PASS`.

---

### Task 4: Rate limiting middleware

**Files:**

- Create: `backend/internal/middleware/rate_limit.go`
- Create: `backend/internal/middleware/rate_limit_test.go`

- [ ] **Step 1: Add `golang.org/x/time/rate` dependency**

```bash
cd backend && go get golang.org/x/time/rate
```

- [ ] **Step 2: Write the test**

```go
// backend/internal/middleware/rate_limit_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

func TestRateLimit_BlocksExcess(t *testing.T) {
	// 1 request per minute — first should pass, second should be blocked
	rl := middleware.NewRateLimiter(1, 60) // 1 req per 60 seconds burst=1
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := func() int {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec.Code
	}

	if code := req(); code != http.StatusOK {
		t.Fatalf("first request: want 200, got %d", code)
	}
	if code := req(); code != http.StatusTooManyRequests {
		t.Fatalf("second request: want 429, got %d", code)
	}
}
```

- [ ] **Step 3: Implement**

```go
// backend/internal/middleware/rate_limit.go
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
	mu      sync.Mutex
	clients map[string]*rate.Limiter
	rate    rate.Limit
	burst   int
}

func NewRateLimiter(requestsPerPeriod int, periodSeconds int) *RateLimiter {
	r := rate.Every(time.Duration(periodSeconds) * time.Second / time.Duration(requestsPerPeriod))
	return &RateLimiter{
		clients: make(map[string]*rate.Limiter),
		rate:    r,
		burst:   requestsPerPeriod,
	}
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok := rl.clients[ip]; ok {
		return l
	}
	l := rate.NewLimiter(rl.rate, rl.burst)
	rl.clients[ip] = l
	return l
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

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./internal/middleware/... -v -run TestRateLimit
```

Expected: `PASS`.

---

### Task 5: Auth middleware (session lookup)

**Files:**

- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/context.go`

- [ ] **Step 1: Write context key helpers**

```go
// backend/internal/middleware/context.go
package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const sessionUserKey contextKey = "session_user"

type SessionUser struct {
	UserID   string
	Username string
	Email    string
	Role     string
}

func SetSessionUser(r *http.Request, u SessionUser) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionUserKey, u))
}

func GetSessionUser(r *http.Request) (SessionUser, bool) {
	u, ok := r.Context().Value(sessionUserKey).(SessionUser)
	return u, ok
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetSessionUser(r); !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := GetSessionUser(r)
		if !ok || u.Role != "admin" {
			writeError(w, http.StatusForbidden, "forbidden", "Admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
```

- [ ] **Step 2: Implement session middleware**

The session middleware reads the cookie, hashes it, and looks up in DB. It uses a function type so the DB query is injected (no global DB):

```go
// backend/internal/middleware/auth.go
package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
)

// SessionLookupFn is the DB query function injected at startup.
// Returns userID, username, email, role, isActive; returns ("","","","",false,nil) if not found.
type SessionLookupFn func(ctx context.Context, tokenHash string) (userID, username, email, role string, isActive bool, err error)

func Session(lookup SessionLookupFn, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			hash := sha256token(cookie.Value)
			userID, username, email, role, isActive, err := lookup(r.Context(), hash)
			if err != nil {
				logger.Error("session lookup failed",
					"error", err,
					"request_id", r.Header.Get("X-Request-ID"),
				)
				next.ServeHTTP(w, r)
				return
			}
			if !isActive {
				next.ServeHTTP(w, r)
				return
			}
			r = SetSessionUser(r, SessionUser{
				UserID:   userID,
				Username: username,
				Email:    email,
				Role:     role,
			})
			next.ServeHTTP(w, r)
		})
	}
}

func sha256token(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 3: Run all middleware tests**

```bash
cd backend && go test ./internal/middleware/... -v
```

Expected: all `PASS`.

---

### Task 6: Test helper — DB-backed handler tests

**Files:**

- Create: `backend/internal/testutil/testutil.go`

- [ ] **Step 1: Write test utility**

```go
// backend/internal/testutil/testutil.go
//go:build integration

package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// NewDB opens a test DB connection. Skips if DATABASE_URL is not set.
func NewDB(t *testing.T) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("testutil.NewDB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool, db.New(pool)
}
```

---

### Verification

```bash
cd backend && go build ./...
cd backend && go test ./internal/config/... ./internal/middleware/... -v
```

Expected: all unit tests pass. Integration tests skipped without `DATABASE_URL`.

Mark phase 3 complete in `docs/implementation-status.md`.
