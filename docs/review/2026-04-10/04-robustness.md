# Stage 4 — Robustness: Errors, Concurrency, Resources

Date: 2026-04-10
Scope: read-only. Focus: resource lifetimes, goroutine leaks, error swallowing, shutdown semantics, back-pressure.

## 4.1 Executive summary

Stage 4 audits the parts of the code that _work_ but would break under load, under failure, or at shutdown. Unlike Stage 3, nothing here is a "wrong result for a valid input" bug — these are the bugs that only show up at 2am when something else has already gone sideways.

| #   | Severity | Finding                                                                                                                                      |
| --- | -------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| 4.A | 🔴 HIGH  | `RateLimiter.evictLoop` spawns a goroutine with no cancellation — 5 leaked goroutines per server lifetime                                    |
| 4.B | 🔴 HIGH  | `KickPlayer` bare-sends on `h.incoming` — HTTP handler can block forever when the hub queue is saturated                                     |
| 4.C | 🟠 HIGH  | `manager.Shutdown()` doesn't actually cancel hubs — hardcoded `time.Sleep(1 * time.Second)` is a "best effort" without graceful flush        |
| 4.D | 🟠 MED   | `config.Load()` silently discards `url.Parse` error for `CookieDomain`                                                                       |
| 4.E | 🟡 MED   | `config.Load()` has no bounds validation — negative durations are accepted                                                                   |
| 4.F | 🟡 MED   | `runRounds` goroutine leak on shutdown — terminates only via `ctx.Done()`, not via `finishRoom` (cross-ref to 3.B)                           |
| 4.G | 🟡 MED   | `time.AfterFunc(grace, ...)` in `handleUnregister` can block on full `graceExpired` buffer (16) if >16 reconnects stall at shutdown          |
| 4.H | 🟡 low   | `json.Unmarshal` errors swallowed in 3 sites: `hub.handleMessage` (system:kick), `runRounds` (room config), `config.Load` (cookie domain)    |
| 4.I | 🟡 low   | `hub.safeSend` drops messages silently on full 64-slot buffer — broadcasts like `vote_results` can be lost for slow clients                  |
| 4.J | 🔵 info  | Correction to Stage 0: `Session` middleware early-returns when no cookie is present — `/api/health` probes without cookies do NOT hit the DB |

## 4.2 Finding 4.A — `RateLimiter.evictLoop` has no stop channel (HIGH)

### Evidence

`middleware/rate_limit.go:32, 37-50`:

```go
func NewRateLimiter(...) *RateLimiter {
    rl := &RateLimiter{...}
    go rl.evictLoop()  // ← spawned, never cancelled
    return rl
}

func (rl *RateLimiter) evictLoop() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        // ... evict stale entries ...
    }
}
```

And `cmd/server/main.go:107-111` — five rate limiters are constructed:

```go
authLimiter   := mw.NewRateLimiter(cfg.RateLimitAuthRPM, 60)
inviteLimiter := mw.NewRateLimiter(cfg.RateLimitInviteRPH, 3600)
globalLimiter := mw.NewRateLimiter(cfg.RateLimitGlobalRPM, 60)
roomLimiter   := mw.NewRateLimiter(cfg.RateLimitRoomsRPH, 3600)
uploadLimiter := mw.NewRateLimiter(cfg.RateLimitUploadsRPH, 3600)
```

Five goroutines, none can be stopped. On `srv.Shutdown(ctx)`, they keep running.

### Impact

- **Minor in practice for the server binary**: the process dies at exit, so the goroutines die with it. The impact is invisible on a single-binary shutdown.
- **Real in tests**: any Stage 6 integration test that creates a `RateLimiter` will leak 1 goroutine per test invocation. With `-race` on and 50 integration tests, that's 50 ticker goroutines running concurrently with test cleanup — a recipe for flaky race reports and slow teardown.
- **Blocks clean hot-reload**: if the server is ever embedded in a host process (e.g., future SvelteKit SSR colocation), these goroutines outlive the HTTP server and prevent clean teardown.

### Fix

```go
type RateLimiter struct {
    // ... existing fields ...
    stop chan struct{}
}

func NewRateLimiter(requestsPerPeriod, periodSeconds int) *RateLimiter {
    rl := &RateLimiter{
        // ... existing fields ...
        stop: make(chan struct{}),
    }
    go rl.evictLoop()
    return rl
}

func (rl *RateLimiter) evictLoop() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-rl.stop:
            return
        case <-ticker.C:
            // ... evict ...
        }
    }
}

func (rl *RateLimiter) Stop() { close(rl.stop) }
```

Then in `main.go` on shutdown:

```go
defer func() {
    authLimiter.Stop()
    inviteLimiter.Stop()
    globalLimiter.Stop()
    roomLimiter.Stop()
    uploadLimiter.Stop()
}()
```

### Stage 6 test plan

```
Given: a RateLimiter
When:  Stop() is called
Then:  within 100ms, the evictLoop goroutine has returned (assert via a done channel or goleak)
```

Use `go.uber.org/goleak` in test packages that construct RateLimiters — this detects 4.A mechanically.

## 4.3 Finding 4.B — `KickPlayer` can block the HTTP goroutine (HIGH)

### Evidence

`hub.go:771-780`:

```go
func (h *Hub) KickPlayer(userID string) {
    data, _ := json.Marshal(map[string]string{"target_user_id": userID})
    h.incoming <- playerMessage{       // ← bare send, no select/default
        player:  &connectedPlayer{userID: "system"},
        msgType: "system:kick",
        data:    data,
    }
}
```

Called from `room_actions.go:114`:

```go
if hub, ok := h.manager.Get(room.Code); ok {
    hub.KickPlayer(req.UserID)  // called synchronously from HTTP handler
}
```

### Impact

The hub's `incoming` channel is buffered to 64 (`hub.go:152`). Under sustained WS traffic (20 players × 20 msg/s = 400 msg/s), the buffer empties in 160ms. If `Run()` is stuck — e.g., processing a slow `db.CreateSubmission` call, or a slow `CalculateRoundScores` with many submissions — the buffer stays full and `KickPlayer` blocks the HTTP goroutine that called `Kick`.

The chi request goroutine has no deadline on this send. The HTTP `WriteTimeout: 60 * time.Second` would eventually cut the connection, but by then the goroutine is still stuck on the send — the HTTP server does not force-cancel goroutines, only closes the connection. That goroutine leaks until the hub drains.

Concrete scenario: host kicks a player during a 20-player match at round end when the scoring DB writes are piling up. Admin's browser hangs; they retry; now two goroutines are stuck. Repeat enough times and the Go runtime runs out of stack.

### Fix

```go
func (h *Hub) KickPlayer(userID string) error {
    data, _ := json.Marshal(map[string]string{"target_user_id": userID})
    select {
    case h.incoming <- playerMessage{...}:
        return nil
    case <-time.After(2 * time.Second):
        return fmt.Errorf("hub busy")
    }
}
```

Or, cleaner, thread a caller-provided ctx:

```go
func (h *Hub) KickPlayer(ctx context.Context, userID string) error {
    select {
    case h.incoming <- playerMessage{...}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

Then `room_actions.go`:

```go
if hub, ok := h.manager.Get(room.Code); ok {
    if err := hub.KickPlayer(r.Context(), req.UserID); err != nil {
        h.log.Warn("kick did not reach hub", "err", err)
    }
}
```

Note: the DB-side kick (row removal in `RemoveRoomPlayer`) is already done at this point, so failing to notify the hub is recoverable — the player will find out on their next WS message and the hub will treat them as "not a member" via the next lookup. This is an acceptable degradation.

### Stage 6 test plan

```
Given: a hub with incoming buffer artificially filled (run the test with buffer=2 via a test helper)
When:  KickPlayer is called with a ctx that times out in 50ms
Then:  KickPlayer returns context.DeadlineExceeded within 100ms
       (and the HTTP goroutine is not leaked — assert via goleak)
```

## 4.4 Finding 4.C — `manager.Shutdown()` doesn't cancel hubs (HIGH)

### Evidence

`manager.go:91-105`:

```go
func (m *Manager) Shutdown() {
    m.mu.RLock()
    hubs := make([]*Hub, 0, len(m.hubs))
    for _, h := range m.hubs {
        hubs = append(hubs, h)
    }
    m.mu.RUnlock()

    for _, h := range hubs {
        h.broadcast(buildMessage("server_restarting", map[string]string{
            "message": "Server is restarting. Please reconnect in a few moments.",
        }))
    }
    time.Sleep(1 * time.Second)  // ← magic wait
}
```

And `main.go:238-243`:

```go
manager.Shutdown() // Notify WS clients before closing listeners
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    logger.Error("shutdown error", "error", err)
}
```

### Impact analysis

Two separate problems:

1. **No per-hub context cancellation**. `Shutdown()` broadcasts `server_restarting` but does not cancel the contexts the hubs were spawned with. If the fix for 3.A creates hubs with a server-scoped context, `Shutdown()` needs to call `cancelServerCtx()` to actually stop the Run goroutines. Without that, `srv.Shutdown(ctx)` below gets 30s to drain HTTP, but the hub goroutines continue until `ctx` is cancelled — which in the current code is `context.Background()` (never).

2. **Fire-and-forget broadcast**. `h.broadcast` calls `safeSend` on each player, which uses `select/default` — meaning if any player's `send` channel is full (64 messages queued), the `server_restarting` message is dropped. A slow client misses the shutdown notice and sees a sudden connection close.

3. **1-second sleep is arbitrary**. It's neither enough to drain a saturated send channel (64 messages × ~5ms each = 320ms minimum, assuming no network congestion) nor short enough to be a no-op. At shutdown, if the writePump loops haven't flushed, clients lose messages.

### Fix sketch

```go
type Manager struct {
    // ... existing ...
    baseCtx    context.Context
    baseCancel context.CancelFunc
}

func NewManager(...) *Manager {
    ctx, cancel := context.WithCancel(context.Background())
    return &Manager{
        // ... existing ...
        baseCtx:    ctx,
        baseCancel: cancel,
    }
}

func (m *Manager) GetOrCreate(...) *Hub {
    // uses m.baseCtx instead of caller-provided ctx
    // ...
    go func() { h.Run(m.baseCtx); /* cleanup */ }()
}

func (m *Manager) Shutdown(ctx context.Context) error {
    // 1. snapshot hubs
    // 2. broadcast server_restarting
    // 3. wait up to `ctx` for writePumps to flush (use a waitgroup)
    // 4. cancel m.baseCancel — Hub.Run exits cleanly
    // 5. wait for all hub Run() goroutines to return (another waitgroup)
    // return ctx.Err() if deadline hit
}
```

The caller in `main.go` passes a deadline-bounded context:

```go
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer shutdownCancel()
if err := manager.Shutdown(shutdownCtx); err != nil {
    logger.Warn("manager shutdown incomplete", "err", err)
}
if err := srv.Shutdown(shutdownCtx); err != nil { ... }
```

**This ties into 3.A**: the fix for hub creation should introduce `baseCtx` on Manager, making 4.C fixable as a package.

## 4.5 Finding 4.D — `CookieDomain` derivation swallows `url.Parse` error (MED)

### Evidence

`config.go:82-84`:

```go
if u, err := url.Parse(cfg.FrontendURL); err == nil {
    cfg.CookieDomain = u.Hostname()
}
```

The `err != nil` branch is absent. If `FrontendURL=not a url`, `cfg.CookieDomain` silently stays at its zero value `""`.

### Impact

Empty `Domain` in `http.Cookie` means "this exact host" — which is actually SAFER than a wildcard-ish domain because the cookie won't leak to subdomains. But the failure mode is invisible: the operator thinks cookies are scoped to `FRONTEND_URL`'s hostname and gets something different. If the frontend and backend share a base domain (e.g., `meme.example.com` and `meme-api.example.com`) and rely on `CookieDomain=.example.com`, an empty domain means the backend can't set a cookie that the frontend sees — login silently fails.

`url.Parse` is **extremely permissive**; it rarely errors for human typos. More realistically, it parses `FRONTEND_URL=meme.example.com` (no scheme) successfully but `Hostname()` returns `""`. Same failure mode, same silence. A fail-fast startup check is the correct fix.

### Fix

```go
u, err := url.Parse(cfg.FrontendURL)
if err != nil {
    return nil, fmt.Errorf("FRONTEND_URL is not a valid URL: %w", err)
}
if u.Scheme == "" || u.Hostname() == "" {
    return nil, fmt.Errorf("FRONTEND_URL must include a scheme and host, got %q", cfg.FrontendURL)
}
cfg.CookieDomain = u.Hostname()
```

### Stage 6 test plan

```
test cases for config.Load:
    - FRONTEND_URL="https://meme.example.com" → CookieDomain="meme.example.com" ✓
    - FRONTEND_URL="meme.example.com"         → error ("must include a scheme")
    - FRONTEND_URL="not a url"                → error
    - FRONTEND_URL=""                         → error (already caught by required-vars check)
```

## 4.6 Finding 4.E — No bounds validation on duration/int env vars (MED)

### Evidence

`config.go:100-132` — every `getEnvDuration` / `getEnvInt` goes directly into `cfg`. No `if d <= 0 { return error }` check anywhere.

### Impact

Semi-hypothetical misconfigurations that would boot a broken server:

- `RECONNECT_GRACE_WINDOW=-10s` → `time.AfterFunc` fires immediately → every disconnect removes the player instantly. Reconnection becomes impossible.
- `SESSION_TTL=0` → sessions expire at creation → users are logged out before their `/api/auth/verify` response returns.
- `MAGIC_LINK_TTL=1ns` → tokens expire before the email leaves the queue.
- `WS_RATE_LIMIT=0` → division by zero or locked-out clients depending on interpretation (actually: `rate.Every(1s/0)` panics).
- `RATE_LIMIT_GLOBAL_RPM=-1` → negative rate → `rate.Limiter` behavior undefined.

### Fix

Add a validation pass after `Load()`:

```go
type durationBound struct {
    name string
    val  time.Duration
    min, max time.Duration
}
bounds := []durationBound{
    {"SESSION_TTL", cfg.SessionTTL, 1 * time.Minute, 365 * 24 * time.Hour},
    {"MAGIC_LINK_TTL", cfg.MagicLinkTTL, 30 * time.Second, 24 * time.Hour},
    {"RECONNECT_GRACE_WINDOW", cfg.ReconnectGraceWindow, 1 * time.Second, 10 * time.Minute},
    // ... etc
}
for _, b := range bounds {
    if b.val < b.min || b.val > b.max {
        return nil, fmt.Errorf("%s=%v must be between %v and %v", b.name, b.val, b.min, b.max)
    }
}
```

Same for `int` fields with minimum 1.

### Stage 6 test plan

A table-driven test that feeds each bad value via `t.Setenv` and asserts `Load()` returns a specific error. No testcontainers needed — pure config test.

## 4.7 Finding 4.F — `runRounds` goroutine leak cross-ref (MED)

Already covered in Stage 3.3 (finding 3.B). Repeating here for completeness because it is both a **correctness** and a **resource** bug:

- Correctness: ghost rounds after host disconnect.
- Resource: the round runner goroutine survives the hub's "finished" state until server shutdown. For long-lived servers with many abandoned games, this accumulates.

The fix — a dedicated `roundsCtx, roundsCancel` inside the hub cancelled by `finishRoom` — addresses both.

## 4.8 Finding 4.G — `graceExpired` buffer exhaustion edge case (MED)

### Evidence

`hub.go:153`: `graceExpired: make(chan graceExpiredMsg, 16)`.
`hub.go:254`: `time.AfterFunc(grace, func() { h.graceExpired <- graceExpiredMsg{...} })`.

### Impact

The buffer is 16. If more than 16 simultaneous disconnect+grace windows are pending and the Run() loop is stuck (or has exited), the AfterFunc goroutine **blocks forever** on the send.

Realistic scenario: 20-player room, network blip disconnects all 20 at once, grace window fires for 17, 18, 19, 20 → four AfterFunc goroutines stuck on the send.

The comment at `hub.go:249-251` explicitly claims this avoids a leak:

```go
// time.AfterFunc avoids a goroutine leak: it uses an internal timer goroutine
// that exits after firing once. All state changes still happen inside Run()
// since the send is into a buffered channel read by Run().
```

This is correct _only while the buffer has capacity_. The comment underclaims the fragility.

### Fix

Make the AfterFunc non-blocking or context-aware:

```go
time.AfterFunc(grace, func() {
    select {
    case h.graceExpired <- graceExpiredMsg{userID: userID, username: username}:
    default:
        // buffer full — fall back to sync path or log + drop
    }
})
```

`default` drop is safer than block. A dropped grace-expired means the player row stays in `h.players` marked as `reconnecting` until the hub ends — not ideal, but survivable.

Alternative: buffer sized to `len(h.players)` at start of room, resized on join. Overengineered.

## 4.9 Finding 4.H — Silent `json.Unmarshal` errors (low)

Three sites with `//nolint:errcheck` or ignored errors:

1. `hub.go:308` — `system:kick` payload unmarshal. A malformed payload produces a no-op (zero value `TargetUserID==""`, `h.players[""]` miss). Low impact — but the code gives no signal that malformed kicks are arriving, which is an attack surface (probing).
2. `hub.go:440` — `runRounds` room config unmarshal. Malformed JSON falls through to zero values, then the zero checks (`if cfg.RoundCount == 0 { ... = 3 }`) silently substitute defaults. Operator has no idea their config was rejected.
3. `rooms.go:114` — same room config unmarshal as part of create validation (already parsed once into a different struct at line 67 — so this second unmarshal is redundant scaffolding).

### Fix pattern

Log (`h.log.Warn("hub: parse system:kick payload", "err", err)`) and early-return. No user-visible change; adds observability.

## 4.10 Finding 4.I — `safeSend` drops messages silently (low)

`hub.go:731-747`:

```go
func (h *Hub) safeSend(p *connectedPlayer, msg []byte) {
    defer func() { if r := recover(); r != nil { ... } }()
    select {
    case p.send <- msg:
    default:
        if h.log != nil {
            h.log.Warn("hub: dropped message for slow consumer", ...)
        }
    }
}
```

The drop is **logged**, which is half-good. But there's no back-pressure: the hub keeps generating messages and dropping them one by one. A player with a slow network in the middle of `vote_results` (which contains the full leaderboard, potentially ~4KB) will miss it entirely — their client never knows the round ended. The only recovery is a `room_state` on next message, which may not come until the next round starts.

### Fix options

1. **Close the slow connection on drop**. Gorilla WS already closes on `writePump` write deadline. Tightening the `WS_WRITE_DEADLINE` and forcing close-on-drop makes the failure mode "slow clients are disconnected" — cleaner than "slow clients silently desync".
2. **Queue critical messages on a separate, larger channel**. Overkill.
3. **Retry with shorter timeout**. Adds complexity.

Option 1 is the right call. Stage 7 punch list.

### Metrics

`safeSend` should increment a `ws_messages_dropped_total{reason="slow_consumer"}` counter. The Prometheus setup is already in place (see `/api/metrics`), but this counter does not exist. Adding it is a one-liner and turns a silent failure into an observable one.

## 4.11 Finding 4.J — Stage 0 correction (info)

Stage 0 §0.6 claimed that `Session` middleware running on every request means every `/api/health` probe hits Postgres. Re-reading `middleware/auth.go:17-22`:

```go
cookie, err := r.Cookie("session")
if err != nil || cookie.Value == "" {
    next.ServeHTTP(w, r)
    return
}
```

The middleware early-returns when no `session` cookie is present. Health probes from kube/docker typically don't carry cookies, so no DB hit. **Stage 0 was wrong.** Correction: only requests that carry a session cookie trigger the renewal write. This downgrades 3.C's impact slightly — it's still a bug (every real user's every request causes a write), but it's not "every health probe writes too".

## 4.12 Things I checked and found acceptable

- **`pgxpool` default size** — not tuned in config but `pgxpool.New` default is `max(4, num_cpu)`. For a single-machine deploy this is reasonable. No finding.
- **`srv.ReadTimeout = 15s, WriteTimeout = 60s, IdleTimeout = 120s`** — sensible. No finding.
- **Request body size limits**: `MAX_UPLOAD_SIZE_BYTES` defaulted to 2MB, checked in `assets.go:55` **before** the pre-signed URL is issued. But `assets.go` only sees the declared size; the actual upload goes directly to RustFS with no body size cap. S3/RustFS enforces this via the pre-signed URL policy — see Stage 5. Deferred.
- **`pool.Close()` on defer** — correct defer in `main.go:52`.
- **`migrate.Up()` idempotency** — correctly uses `errors.Is(err, migrate.ErrNoChange)`. Good.
- **`runMigrations` uses iofs** — embedded migrations, correct pattern for single-binary deploy.

## 4.13 Findings summary

| #   | Severity | Finding                                                     | Fix owner                           |
| --- | -------- | ----------------------------------------------------------- | ----------------------------------- |
| 4.A | 🔴 HIGH  | `RateLimiter.evictLoop` no stop, 5 leaked goroutines        | Stage 7 punch #4                    |
| 4.B | 🔴 HIGH  | `KickPlayer` bare-send can block HTTP goroutine             | Stage 7 punch #5                    |
| 4.C | 🟠 HIGH  | `manager.Shutdown()` no real hub cancellation               | Stage 7 punch #6 (bundled with 3.A) |
| 4.D | 🟠 MED   | `CookieDomain` `url.Parse` error swallowed                  | Stage 7 punch #7                    |
| 4.E | 🟡 MED   | No bounds validation on duration/int env vars               | Stage 7 punch #8                    |
| 4.F | 🟡 MED   | `runRounds` goroutine leak (cross-ref to 3.B)               | Bundled with 3.B                    |
| 4.G | 🟡 MED   | `graceExpired` buffer exhaustion edge case                  | Stage 7 punch #9                    |
| 4.H | 🔵 low   | 3x silent `json.Unmarshal` errors                           | Stage 7 punch #10                   |
| 4.I | 🔵 low   | `safeSend` drops messages silently                          | Stage 7 punch #11                   |
| 4.J | ℹ info   | Stage 0 correction: session mw early-returns without cookie | —                                   |

## 4.14 Stage 6 preview: tests that must be added for 4.x

1. **Goroutine leak detection** (`goleak` package check in every integration test) catches 4.A + 4.F + 4.G automatically.
2. **Hub back-pressure test**: fill `incoming` to capacity, assert `KickPlayer(ctx)` with short deadline returns `ctx.Err()` within deadline. Catches 4.B.
3. **Graceful shutdown test**: spawn server, create room, connect WS, SIGTERM, assert WS received `server_restarting` within 1s and all goroutines exited within 10s. Catches 4.C.
4. **Config validation table test**: ~20 bad inputs, assert each returns a descriptive error. Catches 4.D + 4.E.
5. **JSON parse error logging assertion**: inject a malformed system:kick payload, assert log line with `err` field present. Catches 4.H.
6. **`ws_messages_dropped_total` Prometheus counter test**: simulate slow consumer, assert counter increments. Supports 4.I.
