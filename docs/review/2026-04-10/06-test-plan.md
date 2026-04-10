# Stage 6 — Test Suite & CI Design

Date: 2026-04-10
Scope: design-only. Proposes a layered test architecture, fakes/helpers to add to `testutil`, CI changes, and a regression matrix that locks in every Stage 3–5 must-fix.

## 6.1 Executive summary

The existing test foundation is already strong: `testcontainers-go v0.41.0` is in place, `internal/testutil` has a shared pool + `WithTx` rollback helper, 28 `_test.go` files exist, and 10/10 packages pass with the race detector. Stage 6 is therefore **additive** — it does not propose replacing or restructuring any existing tests.

What Stage 6 adds:

1. A fourfold **test layer model** (unit / integration / contract / end-to-end) with explicit ownership of what each layer proves.
2. **Five test helpers** (`FakeClock`, `FakeEmail`, `FakeStorage`, `WSTestClient`, `HTTPTest`) to kill the Clock blindspot and standardize HTTP/WS assertions.
3. **A CI overhaul**: drop the wasted postgres service, add `sqlc verify`, add `goleak`, add a frontend test job, add coverage reporting.
4. A **regression matrix** that locks in every Stage 3, 4, and 5 must-fix with a single named test.
5. **Coverage targets** by package — not as percentages, but as concrete "this type of test must exist" gates.

## 6.2 Layer 1 — Unit tests (fast, no I/O)

Target: pure-Go functions, data transformations, validators, registry lookups, small state machines.

**What goes here:**

| Package                              | Tests that must exist                                                                                                                                                                                                                                                        |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `game/types/meme_caption`            | `ValidateSubmission` boundary cases (empty, 300 chars, 301 chars, Unicode multi-byte captions); `ValidateVote` self-vote rejection; `CalculateRoundScores` with ties; `BuildSubmissionsShownPayload` author-hiding; `BuildVoteResultsPayload` vote count + score correctness |
| `auth/tokens`                        | `GenerateRawToken` returns 64 hex chars; `HashToken` is deterministic; cookie flags match ADR requirements (see 5.D regression test)                                                                                                                                         |
| `config`                             | Table-driven test of `Load()` with valid + invalid env matrices; specifically covers 4.D (bad FRONTEND_URL) and 4.E (negative durations)                                                                                                                                     |
| `storage.ValidateMIME`               | Magic-byte validation against real file headers; mismatches rejected                                                                                                                                                                                                         |
| `middleware.ClientIP` (new, see 5.B) | Trusted-proxy walking, untrusted peer ignored                                                                                                                                                                                                                                |
| `middleware.RateLimiter`             | Rate calculation; goroutine leak check via `goleak` (4.A)                                                                                                                                                                                                                    |

**Rules for unit tests:**

- Zero external I/O. No DB, no file, no network.
- Must run in <100ms each.
- Use only stdlib `testing.T` assertions (matches existing style).
- No helpers from `testutil` (which implies a container).

## 6.3 Layer 2 — Integration tests (testcontainers-backed)

Target: code that touches Postgres, storage, or email — but not end-to-end HTTP.

**What goes here:**

| Package                                      | Tests that must exist                                                                                                                                                         |
| -------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `db` (sqlc queries)                          | Every non-trivial query has a happy path + 1 edge case (already mostly present)                                                                                               |
| `auth` (handlers, not HTTP)                  | Register → magic link → verify flow; session renewal cadence (3.C regression); invite consumption atomicity; GDPR hard-delete sentinel-UUID reassignment                      |
| `game/hub`                                   | Hub lifecycle: lobby → playing → finished; max-players enforcement (3.D); grace window expiry (requires `FakeClock` — see 6.4); ghost-round-after-finishRoom regression (3.B) |
| `api` (handlers with `httptest.NewRecorder`) | All 40+ REST endpoints — happy path + authz matrix; contract checks for `Leave` state guard (3.E), `Leaderboard` state guard (3.F), `DownloadURL` authz matrix (5.A)          |
| `storage/s3`                                 | Upload/download/delete round-trip against a MinIO testcontainer (NOT mocked — see below)                                                                                      |
| `email`                                      | SMTP send against a Mailhog testcontainer                                                                                                                                     |

**Rules for integration tests:**

- Use `testutil.SetupSuite(m)` per package (already the pattern).
- Use `testutil.WithTx` wherever transaction-rollback is acceptable (pure query tests).
- For handler tests that need multi-query coordination outside a single tx, use `testutil.SeedName(t)` to avoid collisions.
- Every test must be independent of the others' state. No `TestMain` helpers that rely on ordering.
- Must run in <5s each; typical <1s.

### 6.3.1 Two new containers to add to `testutil`

**MinIO** for `storage/s3`:

```go
// testutil/minio.go
func SetupMinIO(ctx context.Context) (*minio.MinioContainer, string, error) {
    c, err := minio.Run(ctx, "minio/minio:latest",
        minio.WithUsername("testkey"),
        minio.WithPassword("testsecret"),
    )
    // ... return connection info
}
```

Currently `storage/s3_test.go` exists but I did not fully audit it — Stage 6 recommendation is to ensure it runs against a container, not a mock, because the pre-signed URL path is notoriously subtle.

**Mailhog** for `email`:

```go
// testutil/mailhog.go
func SetupMailhog(ctx context.Context) (host string, apiURL string, err error) {
    c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "mailhog/mailhog:latest",
            ExposedPorts: []string{"1025/tcp", "8025/tcp"},
            WaitingFor:   wait.ForListeningPort("1025/tcp"),
        },
        Started: true,
    })
    // ... return SMTP host + API URL for assertion
}
```

Tests can then POST to the HTTP API at :8025 to assert emails arrived, without relying on log strings or fakes.

## 6.4 Layer 3 — Contract tests

Target: ensuring code matches the documentation and external consumers.

**What goes here:**

- **`docs/api.md` vs router**: a test that parses `cmd/server/main.go` routes (via chi's `Walk()`) and asserts every `docs/api.md` entry has a matching handler and vice versa. Catches findings 3.E, 3.F and any future drift.
- **`docs/reference/error-codes.md` vs handlers**: grep all `writeError(..., "some_code", ...)` calls, assert each code is documented.
- **`sqlc diff`**: run `sqlc diff` in CI — if it exits non-zero, fail the build. Catches the Stage 1 sqlc-drift finding.
- **WebSocket message schema**: every `buildMessage("type", payload)` call site enumerated against a documented `docs/game-engine.md` table.

**Rules for contract tests:**

- Fast (no container needed for route-walking or grep-based checks).
- Run in a dedicated `contract_test.go` file in each relevant package.
- Fail loud with a diff of "documented - code" and "code - documented".

## 6.5 Layer 4 — End-to-end tests

Target: the full lifecycle that a user experiences — HTTP POST creates a room, WS upgrades, game runs, results arrive.

**What goes here:**

- **`TestE2E_GameHappyPath`** (the single most important test): spin up the full backend via `httptest.NewServer(r)` against a testcontainers postgres, create invite → register → magic link → verify → create room → upgrade WS → host start → submit → vote → assert `vote_results` → assert `game_ended`. This test **fails today** because of 3.A.
- **`TestE2E_HostDisconnectFinishesGame`**: 3.B regression.
- **`TestE2E_SessionRenewalRespectsInterval`**: 3.C regression, requires `FakeClock`.
- **`TestE2E_MaxPlayers`**: 3.D.
- **`TestE2E_LeaveStateGuard` / `TestE2E_LeaderboardStateGuard`**: 3.E, 3.F.
- **`TestE2E_DownloadURLAuthzMatrix`**: 5.A, 9-case matrix.
- **`TestE2E_TrustedProxyXFF`**: 5.B.
- **`TestE2E_WebSocketOriginCheck`**: 5.C.

**Rules for E2E tests:**

- One testcontainer per `TestMain`, shared across E2E tests in the package.
- Use `WSTestClient` (6.7) to avoid handrolling WebSocket clients.
- Every E2E test must either fail deterministically or pass — no retries.
- Must run in <30s each (ideally <10s).
- Tagged with `//go:build e2e` if they're slow; run on CI unconditionally, skip locally with `-short`.

## 6.6 The `Clock` seam — load-bearing

Three Stage 3/4 must-fix findings (3.B, 3.C, 4.F) cannot be tested without replacing `time.Now()`. Stage 2 recommended a `Clock` interface; Stage 6 specifies the exact shape.

### 6.6.1 Interface

```go
// backend/internal/clock/clock.go
package clock

import "time"

type Clock interface {
    Now() time.Time
    NewTimer(d time.Duration) Timer
    NewTicker(d time.Duration) Ticker
    After(d time.Duration) <-chan time.Time
    AfterFunc(d time.Duration, f func()) Timer
    Sleep(d time.Duration)
}

type Timer interface {
    C() <-chan time.Time
    Stop() bool
    Reset(d time.Duration) bool
}

type Ticker interface {
    C() <-chan time.Time
    Stop()
}
```

### 6.6.2 Real implementation

```go
// backend/internal/clock/real.go
type Real struct{}

func (Real) Now() time.Time { return time.Now() }
func (Real) NewTimer(d time.Duration) Timer { return realTimer{t: time.NewTimer(d)} }
// ... etc
```

### 6.6.3 Fake implementation

```go
// backend/internal/clock/fake.go
type Fake struct {
    mu      sync.Mutex
    now     time.Time
    timers  []*fakeTimer
}

func NewFake(start time.Time) *Fake { return &Fake{now: start} }

func (f *Fake) Now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }

// Advance moves the clock forward by d, firing any timers whose deadline is crossed.
// Deterministic: timers fire in deadline order, ties broken by creation order.
func (f *Fake) Advance(d time.Duration) {
    f.mu.Lock()
    target := f.now.Add(d)
    sort.Slice(f.timers, ...)
    for _, t := range f.timers {
        if t.deadline.After(target) { break }
        f.now = t.deadline
        f.mu.Unlock()
        t.fire()  // blocks until the test goroutine acknowledges
        f.mu.Lock()
    }
    f.now = target
    f.mu.Unlock()
}
```

The key design constraint: `Advance` must be **deterministic** — the same sequence of `Advance` calls always fires the same timers in the same order. This rules out background goroutines that race with `Advance`.

### 6.6.4 Where to plumb it

- `auth/handler.go`: `Handler.clock clock.Clock`; constructor takes it.
- `game/hub.go`: `Hub.clock clock.Clock`; replace every `time.Now()`, `time.After`, `time.AfterFunc` call.
- `middleware/rate_limit.go`: `RateLimiter.clock clock.Clock`.
- `cmd/server/main.go`: wires `clock.Real{}`.
- `testutil`: exposes `testutil.FakeClock(t)` helper returning `*clock.Fake`.

**This is a ~200-line refactor** but it unblocks ~10 high-value regression tests. Stage 7 punch list treats it as must-do foundation work, ahead of the correctness fixes.

### 6.6.5 Alternative considered and rejected

`benbjohnson/clock` is a well-known library providing this. Rejected because: (a) it adds a dep, (b) its API is similar to the above but slightly different (notably no `AfterFunc` deterministic fire order), and (c) rolling our own is ~150 lines and gives full control. Revisit if maintenance becomes a burden.

## 6.7 Other test helpers to add to `testutil`

### 6.7.1 `FakeStorage`

```go
// testutil/fake_storage.go
type FakeStorage struct {
    uploadURLs   map[string]string
    downloadURLs map[string]string
    // ... or: in-memory blob store
}

func (f *FakeStorage) PresignUpload(ctx context.Context, key string, ttl time.Duration) (string, error) {
    url := "https://fake.storage/upload/" + key
    f.uploadURLs[key] = url
    return url, nil
}
// ... implements storage.Storage
```

Used by handler tests that don't care about real pre-signing. Real S3 round-trip tests still use MinIO.

### 6.7.2 `FakeEmail`

```go
// testutil/fake_email.go
type FakeEmail struct {
    mu       sync.Mutex
    Sent     []FakeEmailMsg
    sendFail error
}

type FakeEmailMsg struct {
    To      string
    Purpose string // "login", "email_change"
    Data    any
}

func (f *FakeEmail) SendMagicLinkLogin(ctx context.Context, to string, d auth.LoginEmailData) error {
    if f.sendFail != nil { return f.sendFail }
    f.mu.Lock(); defer f.mu.Unlock()
    f.Sent = append(f.Sent, FakeEmailMsg{to, "login", d})
    return nil
}
// ... implements auth.EmailSender
```

Tests assert on `fakeEmail.Sent[0].Data.(auth.LoginEmailData).MagicLinkURL`.

### 6.7.3 `WSTestClient`

```go
// testutil/ws_client.go
type WSTestClient struct {
    t      *testing.T
    conn   *websocket.Conn
}

func DialWS(t *testing.T, srv *httptest.Server, roomCode, sessionCookie string) *WSTestClient {
    // Convert http:// to ws:// and upgrade
}

func (c *WSTestClient) Send(msgType string, data any)
func (c *WSTestClient) ExpectMessage(msgType string, within time.Duration) json.RawMessage
func (c *WSTestClient) ExpectError(code string, within time.Duration)
func (c *WSTestClient) Close()
```

`ExpectMessage` is the critical helper — it reads WS messages in a loop until the expected type arrives or the deadline elapses. Without this, every WS test becomes a tangle of ad-hoc goroutines.

### 6.7.4 `HTTPTest`

```go
// testutil/http.go
type HTTPTest struct {
    t      *testing.T
    router http.Handler
    cookie string // optional session cookie
}

func NewHTTPTest(t *testing.T, router http.Handler) *HTTPTest
func (h *HTTPTest) WithSession(userID string) *HTTPTest  // injects a real session row + cookie
func (h *HTTPTest) POST(path string, body any) *httptest.ResponseRecorder
func (h *HTTPTest) GET(path string) *httptest.ResponseRecorder
func (h *HTTPTest) AssertJSON(rec *httptest.ResponseRecorder, status int, expected any)
func (h *HTTPTest) AssertError(rec *httptest.ResponseRecorder, status int, code string)
```

Centralizes the "session row exists, cookie set, handler called" boilerplate that currently varies per test file.

## 6.8 Frontend tests (currently zero)

Stage 0 identified that `frontend/package.json` has no test framework. Stage 6 recommends:

### 6.8.1 Add `vitest` + `@testing-library/svelte`

Not for broad unit testing — the frontend is mostly declarative Svelte components. Targeted tests:

1. **State classes** (`src/lib/state/*`): reactive behavior. E.g., `ws.ts` (if present) — mock the WebSocket, assert state transitions on message arrivals.
2. **API wrappers** (`src/lib/api/*`): fetch mock, assert each call returns correctly-typed data; assert error responses surface.
3. **Critical components**: `SubmitForm`, `VoteForm`, `ResultsView` — assert render after state change, assert submission dispatch.

### 6.8.2 Add `@playwright/test` for E2E

Single smoke test matching the backend E2E: register → magic link → verify → create room → play round → see results. Playwright can launch the full Docker Compose stack via a `globalSetup`.

Run Playwright only on `main` and on PRs labeled `needs-e2e` — otherwise it's too slow for every commit.

### 6.8.3 Svelte 5 `state_referenced_locally` fixes

Independent of test framework: the 9 svelte-check warnings from Stage 1 should be addressed. Each site audited and wrapped in `$derived(...)`. These are correctness issues that will produce stale UI in production.

## 6.9 CI overhaul

Current state (Stage 0 / 1):

- `backend.yml` starts a wasted Postgres service container (testcontainers ignores it).
- `backend.yml` runs `migrate up` against the wasted service — weak assurance only.
- `frontend.yml` runs type-check only, no tests.
- No coverage, no sqlc verify, no goleak, no CodeQL, no dependency review.

### 6.9.1 New `backend.yml`

```yaml
name: Backend CI

on:
  push:
    paths: ['backend/**', '.github/workflows/backend.yml']
  pull_request:
    paths: ['backend/**']

jobs:
  build:
    runs-on: ubuntu-latest
    # NOTE: no services block — testcontainers starts its own postgres
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true
          cache-dependency-path: backend/go.sum
      - name: go mod download
        working-directory: backend
        run: go mod download
      - name: build
        working-directory: backend
        run: go build ./...
      - name: vet
        working-directory: backend
        run: go vet ./...

  sqlc-verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: sqlc-dev/setup-sqlc@v4
        with: { sqlc-version: '1.26.0' }
      - name: sqlc diff
        working-directory: backend
        run: sqlc diff

  test:
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
          cache: true
          cache-dependency-path: backend/go.sum
      - name: unit + integration tests with coverage
        working-directory: backend
        run: |
          go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
      - name: e2e tests
        working-directory: backend
        run: go test -race -count=1 -tags=e2e ./...
      - uses: actions/upload-artifact@v4
        with: { name: coverage, path: backend/coverage.out }

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: govulncheck
        working-directory: backend
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck -show traces ./... || {
            # Allow known transitive false-positives via suppression file
            govulncheck -config .govulncheck.yaml ./...
          }

  docker:
    runs-on: ubuntu-latest
    needs: [build]
    steps:
      - uses: actions/checkout@v4
      - name: docker build smoke test
        run: docker build -t fabyoumeme-backend:ci ./backend
```

Key changes:

- **Dropped**: `services.postgres`, `Install golang-migrate`, `Run migrations` — all unused by tests.
- **Added**: `sqlc-verify` job (5.F / Stage 1 punch), coverage collection, e2e test step, security suppression file support.
- **Split** into jobs that run in parallel (build, sqlc-verify, test, security, docker) — faster CI on typical commits.

### 6.9.2 New `frontend.yml`

```yaml
name: Frontend CI

on:
  push: { paths: ['frontend/**', '.github/workflows/frontend.yml'] }
  pull_request: { paths: ['frontend/**'] }

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          {
            node-version: '22',
            cache: 'npm',
            cache-dependency-path: frontend/package-lock.json
          }
      - run: npm ci
        working-directory: frontend
      - run: npm run check
        working-directory: frontend
      - run: npm run test # vitest (to be added)
        working-directory: frontend
      - run: npm run build
        working-directory: frontend
      - run: npm audit --audit-level=high
        working-directory: frontend
      - run: docker build -t fabyoumeme-frontend:ci ./frontend
```

### 6.9.3 Shared guardrails

- **`CODEOWNERS`**: require a second reviewer for any PR that touches `backend/internal/auth/`, `backend/internal/middleware/`, or `backend/cmd/server/main.go` — the security-critical surfaces.
- **Dependabot**: weekly updates for Go and npm dependencies.
- **CodeQL**: enable on `main` and PRs.

## 6.10 Regression matrix

Every Stage 3/4/5 must-fix has a named test. This matrix is the Stage 7 acceptance gate.

| Finding                                | Regression test name                        | Layer         | Depends on         |
| -------------------------------------- | ------------------------------------------- | ------------- | ------------------ |
| 3.A Hub never created                  | `TestE2E_WebSocketHappyPath`                | E2E           | Stage 7 fix        |
| 3.B Ghost rounds after host disconnect | `TestE2E_HostDisconnectFinishesGame`        | E2E           | FakeClock, 3.A fix |
| 3.C Session renewal every request      | `TestSession_RenewAtMostOncePerInterval`    | Integration   | FakeClock          |
| 3.D max_players not enforced           | `TestHub_RejectJoinWhenFull`                | Integration   | Stage 7 fix        |
| 3.E Leave state guard                  | `TestAPI_LeaveRejectsPlaying`               | Integration   | Stage 7 fix        |
| 3.F Leaderboard state guard            | `TestAPI_LeaderboardRejectsUnfinished`      | Integration   | Stage 7 fix        |
| 4.A RateLimiter goroutine leak         | `TestRateLimiter_StopEndsEvictLoop`         | Unit + goleak | Stage 7 fix        |
| 4.B KickPlayer blocking                | `TestHub_KickPlayerRespectsContext`         | Integration   | Stage 7 fix        |
| 4.C manager.Shutdown                   | `TestManager_ShutdownCancelsHubs`           | Integration   | 3.A + 4.C fix      |
| 4.D CookieDomain parse error           | `TestConfig_LoadRejectsBadFrontendURL`      | Unit          | Stage 7 fix        |
| 4.E Config bounds                      | `TestConfig_LoadRejectsNegativeDurations`   | Unit          | Stage 7 fix        |
| 4.G graceExpired buffer exhaustion     | `TestHub_GraceExpireDropsOnFullBuffer`      | Integration   | Stage 7 fix        |
| 5.A DownloadURL authz                  | `TestAPI_DownloadURLAuthzMatrix`            | Integration   | Stage 7 fix        |
| 5.B Proxy-blind IP                     | `TestMiddleware_ClientIPTrustedProxyWalk`   | Unit          | Stage 7 fix        |
| 5.C WS CheckOrigin                     | `TestWS_CheckOriginNormalizesTrailingSlash` | Integration   | Stage 7 fix        |
| 5.E Username validation                | `TestAuth_ValidateUsernameTable`            | Unit          | Stage 7 fix        |

16 regression tests. Each one, on a Stage 7 fix PR, must transition from RED to GREEN. This is the primary review gate.

## 6.11 `goleak` integration

Add `go.uber.org/goleak` and a `TestMain` in every integration test package:

```go
func TestMain(m *testing.M) {
    defer goleak.VerifyTestMain(m,
        goleak.IgnoreTopFunction("github.com/testcontainers/..."),  // containers' background goroutines
    )
    os.Exit(testutil.SetupSuite(m))
}
```

This automatically catches:

- 4.A (RateLimiter evictLoop)
- 4.F (runRounds leak after finishRoom)
- 4.G (graceExpired buffer exhaustion, if it triggers in a test)
- any future leaks introduced by new code

Zero marginal cost. High marginal safety.

## 6.12 Coverage targets

Stage 6 does NOT set percentage targets — percentages drive bad tests. Instead, each package has a list of "tests that must exist" (see §6.2–6.5). CI fails if any named regression test is absent.

Soft guidance for new code:

- **Pure logic**: 100% line coverage expected (it's unit-testable and cheap).
- **DB queries**: every `:one`, `:many`, `:exec`, `:batchexec` has at least one test calling it.
- **Handlers**: at least one happy-path + one error-path test per handler function.
- **Middleware**: state transitions + header handling exhaustively.

Emit the coverage report as a CI artifact (already in 6.9.1). Do not gate on the number.

## 6.13 Test data factories

To avoid duplicating "create user + login + session" boilerplate, add `testutil/factories.go`:

```go
func MakeUser(t *testing.T, role string) db.User
func MakeSession(t *testing.T, user db.User) (session db.Session, cookie string)
func MakeRoom(t *testing.T, host db.User, gameTypeSlug string) db.Room
func MakePack(t *testing.T, owner db.User, withItems int) db.GamePack
func MakeInvite(t *testing.T, createdBy db.User) db.Invite
```

Each factory uses `testutil.SeedName(t)` for unique IDs and returns real DB rows. Handler tests compose them:

```go
func TestSomething(t *testing.T) {
    admin := testutil.MakeUser(t, "admin")
    _, cookie := testutil.MakeSession(t, admin)
    http := testutil.NewHTTPTest(t, router).WithCookie(cookie)
    rec := http.POST("/api/admin/invites", map[string]any{"max_uses": 5})
    http.AssertJSON(rec, 201, ...)
}
```

This is the single biggest lever for reducing test LOC without reducing assertion count.

## 6.14 What Stage 6 does NOT propose

- **Replacing the existing `testutil` pattern** — it's good.
- **Adding mutation testing** — overkill for this codebase size.
- **Adding property-based testing** (`gopter`) — could be valuable for `meme_caption.CalculateRoundScores`, but not worth the learning curve right now.
- **Migrating to `testify`** — stdlib `testing.T` is already used consistently. Don't churn.
- **Introducing Docker Compose test harness for E2E** — already covered by `httptest.NewServer` + testcontainers. Simpler.

## 6.15 Ordering for Stage 7

The implementation order is dictated by dependencies:

1. **Add `Clock` seam** (6.6) and wire it into `hub`, `auth`, `middleware` — unblocks 10 regression tests.
2. **Add `testutil` helpers** (6.7) — unblocks handler + WS tests.
3. **Fix correctness bugs** (3.A–3.F) — each with its regression test.
4. **Fix robustness bugs** (4.A–4.I) — each with its regression test.
5. **Fix security bugs** (5.A–5.E) — each with its regression test.
6. **Overhaul CI** (6.9) — lock in the new tests as a gate.
7. **Add frontend tests** (6.8) — separate track, can run in parallel.

Stage 7 punch list will number these as discrete work packages.

## 6.16 Cost estimate

Rough effort budget (calendar days, solo dev):

| Work package                                                           | Days         |
| ---------------------------------------------------------------------- | ------------ |
| Clock seam + wire-in                                                   | 1.5          |
| testutil helpers (FakeStorage/Email/Clock/WSClient/HTTPTest/factories) | 2            |
| 16 regression tests                                                    | 3            |
| Correctness fixes (3.A–3.F) + passing tests                            | 3            |
| Robustness fixes (4.A–4.I) + passing tests                             | 2            |
| Security fixes (5.A–5.E) + passing tests                               | 2            |
| CI overhaul (backend + frontend + contracts + sqlc-verify + goleak)    | 1.5          |
| Frontend test bootstrap (vitest + 10 component tests + Playwright E2E) | 2            |
| **Total**                                                              | **~17 days** |

That's ~3.5 calendar weeks of focused work to bring the platform to "production-ready with CI-enforced quality gates". The correctness and security fixes (the must-fix bugs) account for ~5 days; the rest is foundation and hardening.
