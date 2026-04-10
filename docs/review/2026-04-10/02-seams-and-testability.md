# Stage 2 — Architecture & Seams for Testability

Date: 2026-04-10

## 2.1 Framing

A "seam" is a place where code lets you swap real dependencies for test fakes without changing the production call path. Great seams make CI trustworthy; missing seams force you to either (a) write flaky time-based tests with `time.Sleep`, or (b) not test critical paths at all.

This stage catalogs what is already there and what needs to be added.

## 2.2 Seams already in place — ✅

### 2.2.1 `storage.Storage` interface — `internal/storage/storage.go`

```go
type Storage interface {
    PresignUpload(ctx, key, ttl) (string, error)
    PresignDownload(ctx, key, ttl) (string, error)
    Delete(ctx, key) error
    Probe(ctx) error
}
```

Concrete impl: `S3Storage` with compile-time assertion `var _ Storage = (*S3Storage)(nil)`. Handlers and health checks depend on the interface, not the impl.

**Testability**: a handwritten `FakeStorage` with an in-memory map is trivial to write (probably ~30 LOC). Contract tests (call real S3Storage against MinIO/RustFS in a testcontainer) are also feasible.

**Gap**: no `FakeStorage` currently exists in `testutil`. Handler tests that exercise asset upload flows must currently hit the real S3 path or skip. Stage 6 should add one.

### 2.2.2 `auth.EmailSender` interface — `internal/auth/email.go`

```go
type EmailSender interface {
    SendMagicLinkLogin(ctx, to, data) error
    SendMagicLinkEmailChange(ctx, to, data) error
    SendEmailChangedNotification(ctx, to, data) error
}
```

Concrete impl: `email.Service` with compile-time check `var _ auth.EmailSender = (*Service)(nil)`. The `auth.Handler` depends on the interface.

**Testability**: a `FakeEmailSender` with per-method slices of captured messages is the standard pattern. Tests can assert "the magic link was sent to foo@example.com and contains token X" by inspecting the fake.

**Gap**: same as storage — no `FakeEmailSender` in `testutil`. Stage 6 fix.

**Observation**: `email.Service.RenderLogin` (`service.go:45`) exists _specifically_ to let tests render templates without sending. That's a deliberate seam inside the concrete type — nice touch.

### 2.2.3 `db.DBTX` interface (sqlc-generated) — `db/sqlc/db.go`

```go
type DBTX interface {
    Exec(ctx, sql, args...) (pgconn.CommandTag, error)
    Query(ctx, sql, args...) (pgx.Rows, error)
    QueryRow(ctx, sql, args...) pgx.Row
}
```

`db.Queries` wraps a `DBTX`, which can be satisfied by either a `*pgxpool.Pool` or a `pgx.Tx`.

**Testability**: this is the seam that `testutil.WithTx` exploits:

```go
func WithTx(t *testing.T, fn func(q *db.Queries)) {
    tx, _ := sharedPool.Begin(ctx)
    t.Cleanup(func() { tx.Rollback(ctx) })
    fn(db.New(tx))
}
```

Every test gets a clean DB slate because the outer transaction rolls back on cleanup. This is already used heavily in `db/db_test.go` and is the **single most valuable testing seam in the codebase**. Keep it.

**Gap**: no seam for swapping out `*pgxpool.Pool` itself in handlers that take a pool directly (instead of `*db.Queries`). `auth.Handler.New(pool, ...)` holds both `pool` and `db.Queries`. Tests that want to exercise auth flows must either use the shared pool or construct their own. `testutil.Pool()` already exposes the shared pool, so this is tolerable — but it means every auth test must tolerate cross-test state (they use unique `SeedName(t)` values to stay collision-free).

### 2.2.4 `game.GameTypeHandler` interface — `internal/game/handler.go`

```go
type GameTypeHandler interface {
    Slug() string
    SupportedPayloadVersions() []int
    SupportsSolo() bool
    ValidateSubmission(round, payload) error
    ValidateVote(round, submission, voterID, payload) error
    CalculateRoundScores(submissions, votes) map[uuid.UUID]int
    BuildSubmissionsShownPayload(submissions) (json.RawMessage, error)
    BuildVoteResultsPayload(submissions, votes, scores) (json.RawMessage, error)
}
```

Concrete impl: `meme_caption/handler.go`.

**Testability**: excellent. Game-type logic is pure — no DB, no time, no network. `meme_caption/handler_test.go` exists. New game types can be added and unit-tested in isolation.

This is the textbook example of how the rest of the codebase should aspire to be: a stateless pure-function handler hanging off an interface.

### 2.2.5 `middleware.SessionLookupFn` — `internal/middleware/auth.go`

```go
type SessionLookupFn func(ctx, tokenHash) (userID, username, email, role string, isActive bool, err error)
```

The session middleware takes a function, not a `*auth.Handler`. That means middleware tests can pass a hand-rolled closure:

```go
fake := func(_ context.Context, _ string) (string, string, string, string, bool, error) {
    return "u1", "alice", "a@x", "player", true, nil
}
mw := middleware.Session(fake, logger)
```

**Testability**: already used in `middleware/auth_test.go`. No changes needed.

### 2.2.6 Game `Registry` + `NewManager` construction injection

```go
func NewManager(registry *Registry, queries *db.Queries, cfg *config.Config, log *slog.Logger) *Manager
```

All dependencies are passed in. Tests can build a manager with a stub registry + real queries + a minimal config. `game/hub_test.go` does exactly this.

**Testability**: good, except see the `GetOrCreate` / context-lifetime concern in Stage 0 — that's a correctness bug, not a seam gap.

## 2.3 Missing seams — 🔴🟡

### 2.3.1 No `Clock` seam — 🔴 most impactful gap

Production `time.Now()` call sites (excluding tests, excluding simple latency measurement):

| File & line                               | Purpose                               | Criticality |
| ----------------------------------------- | ------------------------------------- | ----------- |
| `internal/auth/handler.go:44`             | Session renewal expiry computation    | 🔴 high     |
| `internal/auth/handler.go:70`             | Magic-link expiry computation         | 🔴 high     |
| `internal/auth/verify.go:34`              | Magic-link expiry check (pre-consume) | 🔴 high     |
| `internal/auth/verify.go:109`             | New session expiry                    | 🔴 high     |
| `internal/auth/register.go:82`            | `consent_at` stamp for new user       | 🟡 med      |
| `internal/auth/bootstrap.go:33`           | Admin seed consent                    | 🟡 med      |
| `internal/auth/profile.go:251`            | GDPR export `exported_at`             | 🟡 med      |
| `internal/game/hub.go:501`                | Round `endsAt` broadcast              | 🔴 high     |
| `internal/game/hub.go:517`                | Voting `votingEndsAt` broadcast       | 🔴 high     |
| `internal/api/packs.go:298`               | Pagination cursor                     | 🟢 low      |
| `internal/middleware/rate_limit.go:41,60` | Rate-limit bucket eviction            | 🟡 med      |
| `internal/middleware/logging.go:22`       | Request latency                       | 🟢 low      |
| `internal/middleware/metrics.go:39`       | Request latency                       | 🟢 low      |

**Why this matters**: tests that want to verify "a magic link created at T expires correctly at T+15m" have three bad options today:

1. **Sleep for 15 minutes** — infeasible.
2. **Insert a token with `expires_at = time.Now() - 1m`** — works for "already-expired" case (`verify_test.go:79` uses this), but can't test "expires _while_ we hold the request", "session renewal boundary at exactly 60m", or "reconnect grace fires at exactly 30s".
3. **Skip the test** — current state for grace-window fires and session renewal thresholds.

**Recommended seam**:

```go
// internal/clock/clock.go
package clock

import "time"

type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    AfterFunc(d time.Duration, f func()) Timer
}

type Timer interface{ Stop() bool }

type Real struct{}
func (Real) Now() time.Time                             { return time.Now() }
func (Real) After(d time.Duration) <-chan time.Time     { return time.After(d) }
func (Real) AfterFunc(d time.Duration, f func()) Timer  { return time.AfterFunc(d, f) }

// Fake is a manually-advanced clock for tests.
type Fake struct { mu sync.Mutex; now time.Time; fns []scheduled }
func (f *Fake) Now() time.Time                          { f.mu.Lock(); defer f.mu.Unlock(); return f.now }
func (f *Fake) Advance(d time.Duration)                 { /* fire scheduled funcs due */ }
```

Inject `clock.Clock` into `auth.Handler`, `game.Hub`, and `middleware.RateLimiter`. Default to `clock.Real{}` in `main.go`; tests pass a `*clock.Fake`.

**Cost**: ~300 LOC of clock package + mechanical edits at ~10 call sites.
**Benefit**: unlocks deterministic tests for:

- Magic-link expiry boundary
- Session renewal across `SessionRenewInterval`
- Reconnect grace expiry (Stage 3 bug I'll describe)
- Round timer overrun
- Rate-limit eviction loop (currently has no exit)

This is the single highest-leverage test-infrastructure change. Put it in Stage 7 as a **should-fix** (not must-fix) because its absence doesn't break prod — it just blocks us from writing certain tests cheaply.

### 2.3.2 No RNG seam — 🟡 minor

Two sites:

- `internal/auth/tokens.go:13` — `GenerateRawToken()` uses `crypto/rand.Read`
- `internal/api/rooms.go:159` — `generateRoomCode()` uses `crypto/rand.Int`

Both use `crypto/rand` (secure, correct). Neither can produce a deterministic value for tests.

**Impact**: low. Tests can work around it by reading the generated token from the DB/response (both paths return the token). There's no test that needs to assert on a specific code.

**Recommendation**: **do not introduce a seam here.** The current design is secure and the test pain is zero. Leave it.

### 2.3.3 Rate limiter `evictLoop` has no cancellation — 🔴 correctness + testability

```go
func NewRateLimiter(...) *RateLimiter {
    rl := &RateLimiter{...}
    go rl.evictLoop()  // goroutine leak on shutdown + untestable
    return rl
}
```

- The goroutine runs forever. `main.go` never calls a `Stop()` method on it.
- The `evictLoop` waits 10 minutes between ticks, so it's completely untestable without either fudging the ticker or mocking the clock.
- **5 limiters are created in `main.go`** → 5 leaked goroutines per process.
- On test process exit (the `go test` binary) this leak is invisible, but running many tests in the same process may eventually trip the race detector's goroutine allowance if state ever gets shared.

**Fix for Stage 4**:

```go
type RateLimiter struct {
    // ... existing
    stop chan struct{}
}

func (rl *RateLimiter) Stop() {
    close(rl.stop)
}

func (rl *RateLimiter) evictLoop() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-rl.stop:
            return
        case <-ticker.C:
            // ... existing eviction logic
        }
    }
}
```

And `main.go` should hold the limiters in a slice and call `Stop()` in the shutdown path.

### 2.3.4 Hub uses `time.Now` and `time.After` directly — 🟡

`hub.runRounds` loops:

```go
endsAt := time.Now().Add(roundDuration)
h.roundCtrl <- roundCtrlStartRound{...endsAt...}
select {
case <-ctx.Done():
    return
case <-time.After(roundDuration):
}
```

- A test that wants to verify "round_started carries the correct `ends_at`" must either wait for the real duration or inject a tiny duration.
- Current `hub_test.go` presumably uses the "inject tiny duration" trick (I didn't read it fully). That's workable but brittle.
- The `time.AfterFunc` in `handleUnregister` is even worse — no way to advance the fake clock to trip the grace expiry without actually waiting.

**Recommendation**: once the `Clock` seam lands, plumb it through `HubConfig.Clock` and substitute all `time.Now()` / `time.After` / `time.AfterFunc` calls. This is part of the same refactor as 2.3.1.

### 2.3.5 No `context.Context` at `Manager` construction — 🔴

`Manager.GetOrCreate(ctx, ...)` takes the context as an argument and passes it to `h.Run(ctx)`. In production, the only sensible context is "server lifetime" — from `main.go`.

Currently:

- `main.go` creates a signal-based quit channel for shutdown but does **not** create a `context.Context` that's cancelled on shutdown.
- If `ws.go` ever gets a (missing) `GetOrCreate` call, it would pass `r.Context()` — scoped to the HTTP request that's already completed by the time the hub starts running. The hub would die instantly.

**Fix for Stage 3 (bug) + Stage 2 (seam)**:

```go
// main.go
srvCtx, srvCancel := context.WithCancel(context.Background())
defer srvCancel()

manager := game.NewManager(registry, queries, cfg, logger, srvCtx)
// ...
<-quit
srvCancel()  // cancel all hubs
manager.Shutdown()
```

Store `srvCtx` on `Manager`; `GetOrCreate` uses it instead of taking `ctx` as an argument.

### 2.3.6 `config.Load()` reads os.Getenv directly — 🟡 testability

Tests that want to exercise different config values must `os.Setenv` before calling `Load()` and clean up afterwards. This works (`config_test.go` does it) but:

- Parallel tests can collide on env state.
- Tests can't easily inject a mid-call config mutation.

**Optional seam**: accept an optional `func(key string) string` lookup in `Load()`:

```go
func Load(lookup func(string) string) (*Config, error) { ... }
// Production:
cfg, err := config.Load(os.Getenv)
// Tests:
cfg, err := config.Load(mapLookup(t, map[string]string{...}))
```

**Priority**: low. Current tests work. Flag for Stage 7 nice-to-have.

## 2.4 Places the code does not use seams correctly — ⚠️

### 2.4.1 `auth.Handler.SessionLookupFn` renews on every call

```go
func (h *Handler) SessionLookupFn(ctx, tokenHash) (...) {
    row, err := h.db.GetSessionByTokenHash(ctx, tokenHash)
    newExpiry := time.Now().Add(h.cfg.SessionTTL)
    h.db.RenewSession(ctx, { ID: row.ID, ExpiresAt: newExpiry })
    return ..., nil
}
```

Every authenticated request triggers:

1. `GetSessionByTokenHash` (SELECT)
2. `RenewSession` (UPDATE)

`config.SessionRenewInterval` (default 60 minutes) **exists but is not consulted**. The code should only renew if `now - row.ExpiresAt + SessionTTL > SessionRenewInterval` (i.e. the session was last renewed more than an hour ago).

**Impact**: every request path does 2 DB writes to the `sessions` table. At 50 req/s that's 100 writes/s, fine for Postgres, but wasteful. More importantly, it defeats the entire purpose of the `SessionRenewInterval` config variable.

**Correctness concern**: this is arguably a bug (Stage 3) more than a seam gap. Listed here because the fix requires a clock + SessionRenewInterval check.

### 2.4.2 `email.Service` is constructed per-request-side-effect-ish

```go
func (s *Service) send(ctx, ...) error {
    client, err := gomail.NewClient(...)  // new client every call
    ...
    return client.DialAndSend(m)
}
```

`gomail.NewClient` is cheap (it does not dial), so this isn't a performance problem. But it means every test that wants to verify SMTP dial behavior must intercept at the `EmailSender` interface (which is already the best seam) rather than at the `Client` level. Fine — noted for completeness.

## 2.5 Proposed `testutil` extensions for Stage 6

Once Stage 2 seams are in place, `testutil` should gain:

- `testutil.FakeStorage` — in-memory map, satisfies `storage.Storage`, captures writes for assertions.
- `testutil.FakeEmail` — slice-per-method recorder, satisfies `auth.EmailSender`, exposes `MagicLinks()` for inspection.
- `testutil.FakeClock` — `clock.Clock` with `Advance(d)` and `SetNow(t)`.
- `testutil.BuildAuthHandler(t, opts...)` — convenience constructor that wires `FakeEmail` + `FakeClock` + the real `testutil.Pool()`.
- `testutil.WSTestClient` — a helper that upgrades to a WS connection, exposes `ReadMessage` / `WriteJSON` with timeouts, and automatically cleans up.

None of these exist today. All are <100 LOC each. Budgeted in Stage 6.

## 2.6 Findings summary

| #   | Severity | Finding                                                                                            | Stage owner |
| --- | -------- | -------------------------------------------------------------------------------------------------- | ----------- |
| 2.1 | 🟡 med   | No `Clock` seam — blocks deterministic time-sensitive tests (TTL, grace, renew, round timer)       | Stage 7     |
| 2.2 | 🔴 high  | `RateLimiter.evictLoop` cannot be stopped — goroutine leak on shutdown + untestable                | Stage 4     |
| 2.3 | 🔴 high  | `Manager` has no server-scoped context — will break hub lifetime if `GetOrCreate` is ever wired up | Stage 3     |
| 2.4 | 🟡 med   | `SessionLookupFn` ignores `SessionRenewInterval` — over-renewing sessions on every request         | Stage 3     |
| 2.5 | 🟢 low   | No `FakeStorage` / `FakeEmail` / `FakeClock` helpers in `testutil`                                 | Stage 6     |
| 2.6 | 🟢 low   | `config.Load()` reads env directly — minor test ergonomics hit                                     | Stage 7     |

Good news: the codebase already has **5 strong seams** (Storage, EmailSender, DBTX, GameTypeHandler, SessionLookupFn) and one very-good test harness (`testutil`). The remaining gaps are surgical, not structural.
