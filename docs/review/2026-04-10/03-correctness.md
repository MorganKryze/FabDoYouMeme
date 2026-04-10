# Stage 3 — Correctness Review per Domain

Date: 2026-04-10
Scope: read-only. Confirming Stage 0–2 findings by re-tracing call graphs end-to-end, plus contract cross-check against `docs/api.md`.

## 3.1 Executive summary

Stage 3 promotes two findings from "suspected" to "confirmed bug that breaks user-visible functionality":

| #   | Severity        | Finding                                                                                                                                                        |
| --- | --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 3.A | 🔴 **CRITICAL** | No production path creates a `Hub`. WebSocket flow returns `404 room_not_found` on every attempt.                                                              |
| 3.B | 🔴 **HIGH**     | Host disconnect fires `finishRoom` but does not cancel `runRounds`, producing ghost rounds, duplicate `game_ended`, and post-finish DB writes.                 |
| 3.C | 🟠 **HIGH**     | `SessionLookupFn` renews the session on every authenticated request — `cfg.SessionRenewInterval` is loaded but never consulted. Turns every read into a write. |
| 3.D | 🟡 **MED**      | `handleRegister` does not enforce `max_players` even though the schema and `game_types.config` expose it.                                                      |
| 3.E | 🟡 **MED**      | `POST /api/rooms/:code/leave` contract drift: docs say "lobby only; host leaving closes the room" — the handler does neither.                                  |
| 3.F | 🟡 **MED**      | `GET /api/rooms/:code/leaderboard` contract drift: docs say "finished rooms only" — the handler returns a leaderboard in any state.                            |
| 3.G | 🔵 **low**      | `votes.value` hardcoded to `{"points":1}` — coincidentally correct for meme-caption, but couples the schema to one handler.                                    |
| 3.H | 🔵 **low**      | `handleMessage` silently ignores `system:kick` unmarshal errors (`//nolint:errcheck`). Harmless today, lands in Stage 4.                                       |

No NEW defects discovered that were not already suspected in Stage 0. But two of the Stage 0 suspicions upgrade to confirmed critical/high, and two docs-vs-code contract drifts are new.

---

## 3.2 Finding 3.A — Hub never created in production (CRITICAL)

### Evidence (re-traced in Stage 3)

`Grep GetOrCreate|NewHub` across the entire codebase returns **three call sites**:

```
internal/game/manager.go:39   func (m *Manager) GetOrCreate(...)       # definition
internal/game/hub.go:138      func NewHub(hc HubConfig) *Hub           # definition
internal/game/hub_test.go:103 manager.GetOrCreate(ctx, room.Code, ...) # test only
```

And three `manager.Get` call sites (non-creating):

```
internal/api/ws.go:36             hub, hubOK := h.manager.Get(roomCode)
internal/api/room_actions.go:114  if hub, ok := h.manager.Get(room.Code); ok { hub.KickPlayer(...) }
internal/game/manager.go (def)
```

### Call graph, annotated

```
Client:  POST /api/rooms  → rooms.Create (rooms.go:34)
           ├─ validates game type, pack, config, mode
           ├─ db.CreateRoom  → writes the `rooms` row
           └─ 201 Created                              ← NO HUB CREATED

Client:  GET /api/ws/rooms/:code  → ws.ServeHTTP (ws.go:28)
           ├─ auth check
           ├─ hub, hubOK := manager.Get(roomCode)     ← hubOK always false
           └─ 404 room_not_found                       ← CONNECTION NEVER UPGRADES
```

Even the `Kick` handler at `room_actions.go:114` defensively uses `if hub, ok := ... ; ok { }` — it _expects_ the hub might be missing. Nothing in the HTTP layer, nothing in startup, nothing in a hypothetical `Start` action, ever calls `GetOrCreate`.

The `hub_test.go:103` test passes because it constructs the hub directly — it exercises a code path the production WS handler does not take. This is the archetypal "tests pass, feature is broken" smell.

### Impact

End-to-end WebSocket flow — the entire gameplay loop — is broken. Every player joining any room receives `404 room_not_found` on WS connect. The product does not work.

### What the fix looks like (Stage 7)

The natural fix is to create the hub lazily in `ws.ServeHTTP` — before calling `Get`, look up the room row, validate state ∈ {lobby, playing}, then call `manager.GetOrCreate(serverCtx, ...)`. Three non-trivial design choices come bundled:

1. **Context lifetime**. `GetOrCreate(ctx, ...)` currently spawns `go h.Run(ctx)`. If the caller passes `r.Context()`, the hub dies when the upgrade request completes. The `Manager` must own a server-scoped context created in `main()` and cancelled on shutdown — see `cmd/server/main.go` where `manager.Shutdown()` is already called. Thread a `serverCtx` into `Manager` via `NewManager(serverCtx, ...)` or add a `Manager.SetBaseContext` setter.
2. **Authorization check**. Only room participants (or the host in lobby state) should be able to cause hub creation. Creating-on-join is convenient but allows any authenticated user with a valid `roomCode` to spin up a hub for a room they aren't playing. Enforce: `user_id IN room_players OR user_id = host`.
3. **State guard**. If the room is already `finished`, do not create a hub. Return `410 Gone` or `404 room_not_active`.

### Stage 6 test plan

The acceptance test for the fix is a single integration test:

```
Given: a real room created via POST /api/rooms (testcontainers-backed)
When:  a WS client dials GET /api/ws/rooms/:code with a valid session cookie
Then:  the upgrade succeeds, the client receives room_state, and player_joined
       is broadcast
```

This test would fail _today_ against every commit on `main`. It is the most load-bearing acceptance test in the whole test plan.

---

## 3.3 Finding 3.B — Host disconnect produces ghost rounds (HIGH)

### The sequence

Reading `hub.go:241-270` and `runRounds` at `hub.go:424-539`:

1. Host disconnects → `handleUnregister` fires → `time.AfterFunc(grace, send-on-graceExpired)`.
2. Grace window elapses with no reconnect → `handleGraceExpired` runs.
3. If `msg.userID == h.hostUserID && h.state == HubPlaying`, it calls `h.finishRoom(ctx, "host_disconnected")`.
4. `finishRoom` writes `room.state = "finished"` and broadcasts `game_ended`. **It does NOT cancel `runRounds`, and it does NOT set `h.state` back to `HubLobby`.**
5. `runRounds`, still sleeping on `time.After(roundDuration)` in its goroutine, wakes up, emits `roundCtrlCloseSubmissions`, creates the next round in the DB (`db.CreateRound`), and pushes more `roundCtrlStartRound` messages.
6. The hub's `Run()` loop keeps processing those — _even after `finishRoom`_ — producing more `round_started`, `submissions_closed`, and a second `game_ended` broadcast at loop completion.

### Observable symptoms

- Duplicate `game_ended` messages — clients may double-navigate or crash on unexpected re-entry.
- DB rows inserted into `rounds` _after_ `rooms.state = 'finished'`. The schema does not prevent this.
- Wrong leaderboard: any votes that arrive after finish (racing against client disconnect) may still be credited.
- Goroutine leak: if the hub `ctx` has been cancelled but the hub context is NOT cancelled by `finishRoom`, `runRounds` survives until `ctx.Done()` (server shutdown) — for possibly many minutes in an idle room.

### Why the Stage 0 triage was conservative

In Stage 0 I flagged this as Stage 4 "robustness" because I had not yet read `finishRoom` and `handleGraceExpired` together. Stage 3 confirms `finishRoom` has no cancellation wiring — Stage 4 re-classification to Stage 3 correctness is warranted.

### Shape of the fix

Either:

- Give the Hub a derived cancellable context for the round runner: `h.roundsCtx, h.roundsCancel = context.WithCancel(ctx)` on `startGame`, pass `h.roundsCtx` to `runRounds`, call `h.roundsCancel()` inside `finishRoom`. Clean and localized.
- Or introduce an explicit `h.finished bool` guard checked at the top of every `handleRoundCtrl` case — simpler but leaves `runRounds` alive until its next `ctx` check.

The cancel-context variant is strictly better: it stops DB writes promptly and ends the goroutine.

### Stage 6 test plan

```
Given: a playing room with 2 players, host triggered start
When:  the host WS disconnects and the grace window elapses
Then:  - rooms.state is "finished" within 100 ms of grace expiry
       - no new rounds rows are created after that point
       - the round runner goroutine has exited (assert via a `runRoundsDone` channel)
       - exactly one game_ended broadcast is observed
```

Requires a `Clock` seam (Stage 2 recommendation) to avoid a 30-second real-time wait in tests.

---

## 3.4 Finding 3.C — Session renewal on every request (HIGH)

### Evidence

`auth/handler.go:44` (unconditionally):

```go
newExpiry := time.Now().Add(h.cfg.SessionTTL)
if _, err := h.db.RenewSession(ctx, db.RenewSessionParams{
    ID:        row.ID,
    ExpiresAt: newExpiry,
}); err != nil && h.log != nil { ... }
```

`config.go:97`:

```go
if cfg.SessionRenewInterval, err = getEnvDuration("SESSION_RENEW_INTERVAL", 60*time.Minute); err != nil { ... }
```

`Grep SessionRenewInterval` across the whole backend returns only the _definition_ and _loader_. Nothing reads the field. The 60-minute default is dead config.

### Impact

- Every authenticated request is a read _and_ a write against `sessions`. For a single active player hitting the WS control-plane and the REST API, that is ~10–60 writes per minute per user.
- `sessions` is one of the hottest tables (session middleware runs on every request including `/api/health` — see Stage 0 §0.6 — so even the liveness probe writes on every scrape).
- Under any meaningful concurrency this becomes a synchronization bottleneck: every connection serialize on `UPDATE sessions SET expires_at = ... WHERE id = ...`.
- Postgres MVCC cost: every update produces a new row version and pressures autovacuum.

### Fix sketch

```go
func (h *Handler) SessionLookupFn(ctx context.Context, tokenHash string) (...) {
    row, err := h.db.GetSessionByTokenHash(ctx, tokenHash)
    if err != nil { return ..., err }

    // Renew at most once per SessionRenewInterval.
    if time.Until(row.ExpiresAt) < h.cfg.SessionTTL - h.cfg.SessionRenewInterval {
        newExpiry := time.Now().Add(h.cfg.SessionTTL)
        _, _ = h.db.RenewSession(ctx, db.RenewSessionParams{ID: row.ID, ExpiresAt: newExpiry})
    }
    return row.UID.String(), row.Username, row.Email, row.Role, row.IsActive, nil
}
```

The condition `time.Until(expires) < SessionTTL - SessionRenewInterval` means "renew only if the session is now closer to expiry than the renewal grace" — at SessionTTL=720h, SessionRenewInterval=1h that means renew only once the session has ≤719h left.

### Stage 6 test plan

Replace `time.Now()` with an injectable clock (Stage 2 recommendation), then:

```
Given: a fresh session, clock at T0
When:  SessionLookupFn is called at T0+1s
Then:  RenewSession is NOT called
When:  SessionLookupFn is called at T0+61min
Then:  RenewSession IS called
```

This is precisely the kind of assertion that is impossible to express without a fake clock — which is why the finding lay dormant despite 9 existing auth tests.

---

## 3.5 Finding 3.D — `handleRegister` ignores `max_players` (MED)

### Evidence

`hub.go:206-239` — `handleRegister` has exactly two gating checks: reconnection path (existing player in grace) and a lobby-state check. It never references `len(h.players)` or any per-game-type max.

Migration `001_initial_schema.up.sql` seeds `game_types.config` with `max_players` in the JSONB, and `registry.go` exposes it via `handler.ConfigDefaults()`. The game-type handler is currently not consulted.

### Impact

A pathological host can invite unlimited players. For meme-caption this is cosmetic (the UI may degrade past ~12 players — no server crash). For a future game type with stricter player caps (e.g. 4-player match game) the hub would silently over-admit and the game logic may misbehave.

Not currently user-visible because meme-caption is the only registered type and has no hard cap. Classification: Stage 3 **MED** — it is a correctness bug against the documented interface (`docs/game-engine.md` promises `max_players` is enforced), but no shipped game type triggers it.

### Shape of the fix

Wire a `MaxPlayers()` method into `game.GameTypeHandler`, have each handler return its cap, check in `handleRegister` before adding to `h.players`:

```go
if h.state == HubLobby && len(h.players) >= h.handler.MaxPlayers() {
    writeWS(p.conn, buildMessage("error", map[string]string{
        "code": "room_full", "message": "Room is full",
    }))
    p.conn.Close()
    return
}
```

The `room_full` error code already exists in `docs/reference/error-codes.md`, implying this check was intended from the start.

---

## 3.6 Finding 3.E — `Leave` handler drifts from `docs/api.md` (MED)

### Evidence

`docs/api.md:46`:

> `POST /api/rooms/:code/leave` — Leave the room (lobby only; host leaving closes the room)

`room_actions.go:55-75`:

```go
func (h *RoomHandler) Leave(w http.ResponseWriter, r *http.Request) {
    // ... auth ...
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil { ... }
    userID, _ := uuid.Parse(u.UserID)
    if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
        RoomID: room.ID, UserID: userID,
    }); err != nil { ... }
    w.WriteHeader(http.StatusNoContent)
}
```

No state check (can leave a `playing` or `finished` room via REST). No host check (host leaving does _not_ close the room). No WS broadcast — any connected WS clients do not see `player_left` for REST-originated leaves (only for WS-originated disconnects via `handleUnregister`).

### Impact

- **Documentation lie**: anyone relying on `docs/api.md` to reason about state machine transitions will miscompute.
- **Host-absent playing room**: if the host REST-leaves during a playing game, the room stays in `playing` state with no host. The WS hub (if it ever gets created — see 3.A) would still process the host's connection if they re-joined.
- **Inconsistent player view**: WS-connected clients see stale player lists after a REST leave.

### Shape of the fix

Two options:

1. **Code follows docs**: enforce lobby state, host-leaving closes room, broadcast `player_left` over the hub if present.
2. **Docs follow code**: update `api.md` to note that leave is unconditional and callers must handle stale WS state.

Option 1 is safer because the documented behavior aligns with intuitive expectations. But this matters for the Stage 7 punch list — flagging it, not fixing it.

---

## 3.7 Finding 3.F — `Leaderboard` handler drifts from `docs/api.md` (MED)

### Evidence

`docs/api.md:48`:

> `GET /api/rooms/:code/leaderboard` — Final leaderboard (finished rooms only)

`room_actions.go:121-138`:

```go
func (h *RoomHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
    // ... auth ...
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil { ... }
    leaderboard, err := h.db.GetRoomLeaderboard(r.Context(), room.ID)
    // ... return leaderboard ...
}
```

No `room.State == "finished"` gate.

### Impact

Mid-game players can query a live leaderboard via REST — this leaks signal during the game. For meme-caption the competition hinges on vote outcomes, so leaking scores mid-round is minor; for future competitive game types it could be match-breaking.

### Fix

```go
if room.State != "finished" {
    writeError(w, r, http.StatusConflict, "room_not_finished", "Leaderboard is only available after the game ends")
    return
}
```

---

## 3.8 Finding 3.G — `votes.value` hardcoded to `{"points":1}` (low, design smell)

At `hub.go:645`:

```go
_, err := h.db.CreateVote(ctx, db.CreateVoteParams{
    SubmissionID: submissionID,
    VoterID:      voterID,
    Value:        json.RawMessage(`{"points":1}`),
})
```

The `room_players.score` column is updated via `UpdatePlayerScore`, which is **additive** (`UPDATE room_players SET score = score + $3` — confirmed at `db/queries/rooms.sql:35`), so the true per-round score is whatever `handler.CalculateRoundScores` returned. The hardcoded `votes.value` is only read back if some future code joins on `votes` for analytics or ranked-choice tiebreakers.

For meme-caption, `CalculateRoundScores` is "one point per vote received" (`handler.go:57-71`), which exactly matches the hardcoded value — pure coincidence. Adding any game type with weighted voting (e.g. 3-2-1 ranked votes) would require this line to become `handler.VotePayload(msg.data)` or equivalent. Classification: **low** — queued for Stage 4 as "remove the coupling", not a user-visible bug today.

---

## 3.9 Finding 3.H — `system:kick` unmarshal error swallowed (low, robustness)

`hub.go:308`:

```go
json.Unmarshal(msg.data, &d) //nolint:errcheck
```

A malformed `system:kick` payload silently no-ops (zero-value `d.TargetUserID == ""`, nothing in `h.players[""]` → the `if ok` falls through). No error logged; no error sent to the caller. Not a correctness issue — a robustness / observability one. Moving to Stage 4.

---

## 3.10 Correctness checks that PASSED

Worth recording what did _not_ break under scrutiny, so Stage 4 doesn't re-investigate:

- **Atomic magic-link consumption** (`auth/verify.go` + `db/queries/magic_link_tokens.sql`): the `ConsumeMagicLinkTokenAtomic` query uses `UPDATE ... WHERE expires_at > NOW() AND used_at IS NULL RETURNING ...`, which is correctly atomic under concurrent verification attempts. The pre-check for `expires_at` is only for distinct error codes and does not introduce a TOCTOU window. ✅
- **GDPR sentinel UUID** (`db/queries/users.sql`): the hard-delete paths correctly reassign `submissions.user_id` AND `votes.voter_id` to the sentinel UUID before the user row DELETE. Matches ADR-006. ✅
- **`UpdatePlayerScore` is additive**, not assignment — I confirmed at `db/queries/rooms.sql:35`. Multi-round scoring is correct. ✅
- **`handleRegister` reconnection path** closes the old `send` channel before swapping `h.players[userID]` (`hub.go:213`) — the comment explicitly notes this avoids a race with the old `writePump`. Careful implementation. ✅
- **`runRounds` ctx checks** between phases (`hub.go:464, 511, 523, 531`) correctly bail on server shutdown (`ctx.Done()`), though they do NOT bail on `finishRoom` — see finding 3.B.
- **Vote self-vote prevention**: `ValidateVote` in `meme_caption/handler.go:48` checks `submission.UserID == voterID` and returns `ErrSelfVote`. ✅
- **`submission_closed` double-submission prevention**: `hub.go:565` `if _, already := h.roundSubmissions[msg.player.userID]; already` — correct. ✅
- **Author hiding during voting**: `BuildSubmissionsShownPayload` deliberately omits `author_username` (`meme_caption/handler.go:77`). ✅

## 3.11 Findings summary

| #   | Severity | Finding                                                                          | Stage owner           |
| --- | -------- | -------------------------------------------------------------------------------- | --------------------- |
| 3.A | 🔴 CRIT  | Hub never created in production → WS 404 on every room                           | Stage 7 (must-fix #1) |
| 3.B | 🔴 HIGH  | `finishRoom` doesn't cancel `runRounds` → ghost rounds                           | Stage 7 (must-fix #2) |
| 3.C | 🟠 HIGH  | `SessionLookupFn` renews on every request; `SessionRenewInterval` is dead config | Stage 7 (must-fix #3) |
| 3.D | 🟡 MED   | `handleRegister` doesn't enforce `max_players`                                   | Stage 7 (should-fix)  |
| 3.E | 🟡 MED   | `Leave` handler doesn't enforce lobby-only or host-leaves-closes                 | Stage 7 (should-fix)  |
| 3.F | 🟡 MED   | `Leaderboard` handler doesn't enforce finished-only                              | Stage 7 (should-fix)  |
| 3.G | 🔵 low   | `votes.value` hardcoded — design smell, coincidentally correct                   | Stage 4               |
| 3.H | 🔵 low   | `system:kick` unmarshal error swallowed                                          | Stage 4               |

**Three must-fix correctness bugs** (3.A, 3.B, 3.C). All three need a `Clock` seam for the regression tests — reinforcing Stage 2's seam proposal.

## 3.12 Stage 6 preview: correctness tests that MUST be added

The regression matrix for Stage 3 findings:

1. **WS end-to-end happy path**: create room → WS connect → host start → submit → vote → results. This single test catches 3.A.
2. **Host disconnect during playing**: grace window expiry with host-leaves → assert no more rounds created, exactly one `game_ended`, runner goroutine exited. Catches 3.B.
3. **Session renewal cadence**: clock-advanced test that `RenewSession` is called at most once per `SessionRenewInterval`. Catches 3.C.
4. **Max-players**: join N+1 players where N = handler.MaxPlayers() → 11th receives `room_full`. Catches 3.D.
5. **Leave state guard**: leave from a `playing` room → `409 room_not_in_lobby`. Catches 3.E.
6. **Leaderboard state guard**: GET leaderboard on a `playing` room → `409 room_not_finished`. Catches 3.F.

These six tests are the acceptance suite for Stage 7 fixes.
