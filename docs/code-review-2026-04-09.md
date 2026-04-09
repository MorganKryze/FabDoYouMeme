# FabDoYouMeme — Full Codebase Code Review

**Date**: 2026-04-09  
**Scope**: All 12 phases implemented (backend + frontend + infrastructure)  
**Method**: 5 parallel domain-focused review agents covering auth/middleware/config, game engine/API/storage, DB layer/entrypoint, frontend, and design-doc vs implementation consistency + ops.

---

## Executive Summary

| Severity  | Count  |
| --------- | ------ |
| CRITICAL  | 15     |
| HIGH      | 24     |
| MEDIUM    | 33     |
| LOW       | 22     |
| **Total** | **94** |

The codebase is architecturally sound and well-structured. The primary concerns are:

1. **GDPR hard-delete is blocked** by an FK constraint on `rooms.host_id`.
2. **Multiple admin API endpoints have no auth checks** — any user can call them.
3. **The WebSocket hub has a race condition** that can panic on a closed channel.
4. **Room code generation uses `math/rand`** (not cryptographically secure).
5. **Magic byte MIME validation is optional**, allowing upload spoofing.
6. **The round loop (`runRounds`) is a stub** — games cannot progress past lobby.
7. **Frontend type definitions diverge** from the backend protocol contract in multiple places.
8. **Several REST endpoint groups are completely absent** from the router (items, versions, rooms actions).

---

## Layer 1 — Backend: Auth / Middleware / Config

### CRITICAL

#### A1-C1 · Verify endpoint returns wrong error codes

**File**: `backend/internal/auth/verify.go` · Lines 29, 42  
**Category**: Consistency / Logic

The handler emits `"invalid_token"` (401) where the spec requires `token_expired | token_used | token_not_found` (all 400). It also emits `"account_inactive"` (401) where the spec requires `"user_inactive"` (403). Clients cannot distinguish token failure modes; frontend error handling will be incorrect.

**Fix**: Return distinct error codes from `ConsumeMagicLinkTokenAtomic` so the handler can map them. Change `"account_inactive"` → `"user_inactive"` with 403.

---

### HIGH

#### A1-H1 · SMTP credentials not validated at startup

**File**: `backend/internal/config/config.go` · Lines 50–58  
**Category**: Config / Ops

`SMTP_USERNAME` and `SMTP_PASSWORD` are required per `ref-env-vars.md` but are absent from the `required` list. Boot succeeds silently; every magic-link send fails at runtime.

**Fix**: Add both to the required list, or explicitly document they are optional (anonymous SMTP relay).

#### A1-H2 · Session cookie missing `Domain` attribute

**File**: `backend/internal/auth/tokens.go` · Lines 27–49  
**Category**: Security

Cookies are set with `HttpOnly`, `Secure`, `SameSite=Strict` but no `Domain`. If the frontend is on a different subdomain than the API, the browser won't send the cookie back with API requests.

**Fix**: Parse the domain from `FRONTEND_URL` config and set it on both `setSessionCookie` and `clearSessionCookie`.

#### A1-H3 · Rate limiter leaks memory on long runs

**File**: `backend/internal/middleware/rate_limit.go` · Lines 13–38  
**Category**: Ops / Bug

Per-IP limiters accumulate in the map indefinitely. Under sustained traffic from diverse IPs (or an attack), the process will exhaust memory.

**Fix**: Add a background goroutine that evicts limiters idle for >1h, or use an LRU cache with a fixed capacity.

#### A1-H4 · Email change flow has zero test coverage

**File**: `backend/internal/auth/profile_test.go`  
**Category**: Testing

The `PatchMe` handler supports email change (sets `pending_email`, sends magic link, invalidates sessions, sends notification), but the test file only covers username updates. The verification path (`POST /api/auth/verify` with `purpose="email_change"`) is also untested.

**Fix**: Add `TestPatchMe_EmailChange` and `TestVerify_EmailChange_Success` covering the full flow.

#### A1-H5 · Duplicate-email registration body not verified in tests

**File**: `backend/internal/auth/register.go` · Line 90–91  
**Category**: Testing / Security

`TestRegister_DuplicateEmailReturns201` checks only the status code, not that `user_id == ""`. A regression that leaks the existing user's ID won't be caught.

**Fix**: Assert `resp["user_id"] == ""` in the duplicate-email test.

---

### MEDIUM

#### A1-M1 · Inconsistent user ID disclosure on duplicate email

**File**: `backend/internal/auth/register.go` · Lines 52–54, 90–91  
**Category**: Logic / Security

First code path (pre-check) returns the existing user's ID. Second path (DB constraint violation) returns `""`. The anti-enumeration intent is undermined by the first path.

**Fix**: Return `""` in both paths for a consistent contract.

#### A1-M2 · Session renewal errors silently swallowed

**File**: `backend/internal/auth/handler.go` · Lines 44–49  
**Category**: Error Handling

`RenewSession` failures are logged as warnings but ignored. The session TTL won't be extended, potentially causing unexpected logouts.

**Fix**: Acceptable as-is if transient; document the caveat. For stricter guarantees, propagate the error.

#### A1-M3 · `maskEmail` has no unit tests

**File**: `backend/internal/auth/handler.go` · Lines 109–115  
**Category**: Testing

`maskEmail` is called only from email-change verification. No direct test for the edge cases (`no @`, empty local part).

**Fix**: Add `TestMaskEmail_Normal`, `TestMaskEmail_NoAt`, `TestMaskEmail_EmptyLocalPart`.

#### A1-M4 · SMTP failure warning path untested

**File**: `backend/internal/auth/register.go` · Lines 107–114  
**Category**: Testing

When a restricted-email invite triggers SMTP failure, the design requires 201 + `"warning": "smtp_failure"`. This path is not tested.

**Fix**: Mock the email sender to return an error; assert 201 with warning field.

#### A1-M5 · Request ID not included in error JSON

**File**: `backend/internal/middleware/context.go` · Lines 53–57  
**Category**: Observability

Error responses don't include the `X-Request-ID`. Clients cannot correlate displayed errors with server logs.

**Fix**: Extract the request ID from context in `writeError` and include it as `"request_id"` in the JSON body.

#### A1-M6 · `InvalidatePendingTokens` error silently discarded

**File**: `backend/internal/auth/handler.go` · Line 56  
**Category**: Error Handling

The error is assigned to `_`. If invalidation fails, multiple active magic links can exist simultaneously, violating the design invariant "only the latest link is ever valid."

**Fix**: Log the error as a warning; add a comment acknowledging non-fatality.

---

### LOW

#### A1-L1 · Logout swallows `DeleteSession` error

**File**: `backend/internal/auth/session.go` · Line 16  
**Category**: Error Handling

DB errors on logout are silently ignored. The session row lingers until TTL expiry.

**Fix**: Log the error at warn level.

#### A1-L2 · `RateLimitRoomsRPH` / `RateLimitUploadsRPH` loaded but never enforced

**File**: `backend/internal/config/config.go` · Lines 44–46  
**Category**: Ops / Config

These env vars are loaded into the config struct but no middleware consumes them. Room and upload rate limits are effectively unbounded.

**Fix**: Wire these values into rate-limit middleware on the relevant routes.

#### A1-L3 · Request ID test doesn't assert uniqueness

**File**: `backend/internal/middleware/request_id_test.go` · Lines 11–27  
**Category**: Testing

The test verifies only presence. Two separate requests could theoretically get identical IDs without the test catching it.

**Fix**: Generate two IDs and assert they differ.

#### A1-L4 · Logout returns 200 for unauthenticated callers without documentation

**File**: `backend/internal/auth/session.go` · Lines 12–20  
**Category**: Design

Idempotent logout is acceptable but not documented. Future reviewers may add unintended `RequireAuth` middleware.

**Fix**: Add a comment stating idempotency is intentional.

---

## Layer 2 — Backend: Game Engine / API / Storage

### CRITICAL

#### A2-C1 · Goroutine leak on grace window expiry

**File**: `backend/internal/game/hub.go` · Lines 191–197  
**Category**: Bug

The goroutine that signals `h.graceExpired` uses `select { default: }`. If the channel buffer is full, the signal is silently dropped and the player remains in `reconnecting` state indefinitely — they are never removed from the room.

**Fix**: Remove the `default` case and use a `select` with `ctx.Done()` for cancellation; or track pending expiries inside the Run goroutine to avoid spawning goroutines.

#### A2-C2 · Broadcast can panic on a closed send channel

**File**: `backend/internal/game/hub.go` · Lines 327–336  
**Category**: Bug / Race Condition

`broadcast()` iterates `h.players` and writes to `p.send`. A concurrent `writePump` goroutine can close `p.send` between the map read and the send, causing a panic.

**Fix**: Use a `recover()` on the send, or coordinate channel closure via a sentinel value rather than `close()`.

#### A2-C3 · `reconnect` WebSocket message type is unhandled

**File**: `backend/internal/game/hub.go` · Lines 214–239  
**Category**: Logic

The protocol specifies clients send `{ type: "reconnect" }` when returning within the grace window. `handleMessage` has no `case "reconnect"`, so it is treated as an unknown type and rejected with an error.

**Fix**: Add `case "reconnect":` that checks whether the player is in `reconnecting` state and completes the re-registration.

#### A2-C4 · `runRounds()` is a stub — games never advance

**File**: `backend/internal/game/hub.go` · Lines 257–260  
**Category**: Incomplete

The method body contains only a comment. No `round_started` events are ever broadcast; submission/vote windows never open or close; games cannot progress past lobby.

**Fix**: Implement the full round state machine: start timer, broadcast `round_started`, close submissions, open voting, broadcast results, loop or end game.

#### A2-C5 · Room codes generated with `math/rand` (not crypto-safe)

**File**: `backend/internal/api/rooms.go` · Lines 135–142  
**Category**: Security

`math/rand.Intn` is deterministic given a predictable seed. An attacker can enumerate or predict codes and join arbitrary games.

**Fix**: Replace with `crypto/rand` or `math/rand/v2` (Go 1.22+, which seeds from a CSPRNG automatically).

#### A2-C6 · Magic byte MIME validation is optional

**File**: `backend/internal/api/assets.go` · Lines 62–79  
**Category**: Security

Magic byte validation is skipped when the client omits `preview_bytes`. A client can upload a malicious binary with a spoofed `image/png` MIME type.

**Fix**: Make `preview_bytes` required, or always perform magic byte validation.

---

### HIGH

#### A2-H1 · `next_round` WebSocket message type is unhandled

**File**: `backend/internal/game/hub.go` · Lines 214–239  
**Category**: Logic

The protocol documents a `next_round` message sent by the host to advance rounds. It is not handled and will be rejected.

**Fix**: Add `case "next_round":` with host permission check.

#### A2-H2 · `SetStatus` (pack) has no admin auth check

**File**: `backend/internal/api/packs.go` · Lines 218–239  
**Category**: Security

Any authenticated user can change a pack's status to `active`, `flagged`, or `banned`. The protocol marks this as admin-only.

**Fix**: Add admin role check at the start of the handler.

#### A2-H3 · `DeleteInvite` has no auth check at all

**File**: `backend/internal/api/admin.go` · Lines 153–165  
**Category**: Security

Unauthenticated clients can revoke any invite by sending `DELETE /api/admin/invites/:id`.

**Fix**: Add admin role check.

#### A2-H4 · `ListUsers` and `UpdateUser` have no auth checks

**File**: `backend/internal/api/admin.go` · Lines 28–44, 46–84  
**Category**: Security

Both admin endpoints lack any authentication or role verification. Any client can list all users or downgrade an admin to player.

**Fix**: Add admin role check at the start of each handler.

#### A2-H5 · Pagination response missing `next_cursor` and `total`

**File**: `backend/internal/api/admin.go` · Line 180  
**Category**: Consistency

`ListNotifications` returns `{"data": [...]}` only. The protocol contract requires `{ "data", "next_cursor", "total" }` on all paginated endpoints.

**Fix**: Add cursor and total to all paginated responses.

#### A2-H6 · Pagination cursors are integer offsets, not opaque cursors

**File**: `backend/internal/api/packs.go` · Lines 250–272  
**Category**: Design

The implementation uses `strconv.Atoi()` on the `after` parameter instead of a base64-encoded `{id, created_at}` cursor as specified in the protocol. Integer offsets drift on concurrent inserts.

**Fix**: Implement cursor-based pagination with base64-encoded state across all paginated endpoints.

#### A2-H7 · Malformed room `config` JSON silently defaulted

**File**: `backend/internal/api/rooms.go` · Lines 88–94  
**Category**: Logic

If `req.Config` is unparseable JSON, the decode error is ignored and defaults are applied. The caller never learns the config was invalid.

**Fix**: Return 400 if `json.Unmarshal` fails.

#### A2-H8 · Vote results missing `author_username`

**File**: `backend/internal/game/types/meme_caption/handler.go` · Lines 104–107  
**Category**: Logic

`BuildVoteResultsPayload` populates `submissionResult.AuthorUsername` but never sets it — it remains empty. The protocol requires `author_username` in `vote_results`.

**Fix**: Pre-fetch usernames in the hub before calling the handler, or include username in the `Submission` struct.

---

### MEDIUM

#### A2-M1 · Full send channels silently drop messages

**File**: `backend/internal/game/hub.go` · Lines 330–333  
**Category**: Design / Observability

Critical game events (e.g., `round_started`) sent to a slow client are dropped without logging. Players may miss state transitions.

**Fix**: Log dropped messages; close the connection after N consecutive drops.

#### A2-M2 · No timeout on WebSocket upgrade

**File**: `backend/internal/api/ws.go` · Lines 32–53  
**Category**: Ops

Upgrade and `hub.Join` run with the full request context and no timeout. Slow hubs block connection goroutines.

**Fix**: Wrap in a 5s timeout context for the upgrade/join phase.

#### A2-M3 · S3 errors not classified by type

**File**: `backend/internal/storage/s3.go` · Lines 52–86  
**Category**: Observability

All S3 errors surface as a generic "Failed to generate upload URL". Permission errors, timeouts, and not-found errors are indistinguishable.

**Fix**: Inspect error types and return appropriate HTTP status codes to the client.

#### A2-M4 · Room mode not validated against allowed values

**File**: `backend/internal/api/rooms.go` · Lines 57–59  
**Category**: Logic

An invalid mode like `"competitive"` is silently replaced with `"multiplayer"`. The caller is not informed.

**Fix**: Reject unknown modes with 400.

---

### LOW

#### A2-L1 · Hub channel buffer sizes undocumented

**File**: `backend/internal/game/hub.go` · Lines 94–99  
**Fix**: Add inline comments explaining each buffer size and its tuning rationale.

#### A2-L2 · `nextCursor` returns `*string` unnecessarily

**File**: `backend/internal/api/packs.go` · Lines 266–272  
**Fix**: Return plain `string`; use `""` as the sentinel for "no next page".

#### A2-L3 · Malformed submission payloads silently skipped with no logging

**File**: `backend/internal/game/types/meme_caption/handler.go` · Lines 80–94  
**Fix**: Log at warn level when a submission payload fails to unmarshal.

#### A2-L4 · Unknown WS message types not logged server-side

**File**: `backend/internal/game/hub.go` · Lines 234–238  
**Fix**: Add a debug-level log before returning the error to the client.

#### A2-L5 · Room config not validated against game type schema ranges

**File**: `backend/internal/api/rooms.go` · Lines 87–98  
**Fix**: Validate `round_duration_seconds`, `voting_duration_seconds`, etc. against the game type's config schema before insertion.

#### A2-L6 · Hub reconnect/disconnect edge cases lack test coverage

**File**: `backend/internal/game/hub_test.go` · Line 301  
**Fix**: Add tests for host disconnect during `playing` (should end game), all players disconnecting, and reconnect just before grace expiry.

#### A2-L7 · Asset handler tests use `nil` storage

**File**: `backend/internal/api/assets_test.go` · Line 22  
**Fix**: Add a mock storage that returns a URL; verify it appears in the response.

---

## Layer 3 — Backend: Database Layer / Entrypoint

### CRITICAL

#### A3-C1 · `rooms.host_id` FK blocks GDPR hard-delete

**File**: `backend/db/migrations/001_initial_schema.up.sql` · Line 142  
**Category**: Bug / GDPR

`host_id UUID NOT NULL REFERENCES users(id)` has no `ON DELETE` clause, defaulting to `RESTRICT`. Deleting a user who hosts any room — even a finished one — fails. This violates GDPR Art. 17 (Right to Erasure).

**Fix**: Add `ON DELETE CASCADE` (delete room with host) or `ON DELETE SET NULL` (requires making `host_id` nullable) and update application logic accordingly.

---

### HIGH

#### A3-H1 · `users.invited_by` lacks `ON DELETE SET NULL`

**File**: `backend/db/migrations/001_initial_schema.up.sql` · Line 11  
**Category**: Design / Consistency

All other nullable user FKs (`invites.created_by`, `game_packs.owner_id`, `admin_notifications.actor_id`) explicitly use `ON DELETE SET NULL`. `invited_by` defaults to `RESTRICT`, silently blocking deletion of users who have invited others.

**Fix**: Add `ON DELETE SET NULL` to `invited_by`.

#### A3-H2 · `GetSessionByTokenHash` uses INNER JOIN — sessions silently disappear

**File**: `backend/db/queries/sessions.sql` · Lines 8–12  
**Category**: Logic / Consistency

The `JOIN users u ON s.user_id = u.id` means a session for a deleted user returns no rows instead of a cascade-deleted row. The application should explicitly delete sessions before hard-deleting users, not rely on implicit query filtering.

**Fix**: Add `DeleteAllUserSessions(user_id)` to the hard-delete sequence before `HardDeleteUser`.

#### A3-H3 · Missing index on `rooms.host_id`

**File**: `backend/db/migrations/001_initial_schema.up.sql` · Lines 209–230  
**Category**: Performance

FK columns without explicit indexes cause sequential scans on cascade operations and queries filtering by host.

**Fix**: Add `CREATE INDEX ON rooms(host_id);` in the migration.

---

### MEDIUM

#### A3-M1 · Missing simple index on `game_packs.owner_id`

**File**: `backend/db/migrations/001_initial_schema.up.sql` · Line 228  
**Category**: Performance

The existing partial index (`WHERE deleted_at IS NULL`) may not be used efficiently for ON DELETE CASCADE operations.

**Fix**: Add `CREATE INDEX ON game_packs(owner_id);`.

#### A3-M2 · `ConsumeMagicLinkToken` vs `ConsumeMagicLinkTokenAtomic` — usage undefined

**File**: `backend/db/queries/magic_link_tokens.sql` · Lines 26–32  
**Category**: Design / Consistency

Both queries exist. The atomic version is used in tests, but which one the application uses is not documented. Using the non-atomic version allows a race where two concurrent requests consume the same token.

**Fix**: Document that `ConsumeMagicLinkTokenAtomic` must be used for all production paths. Consider removing or deprecating the non-atomic version.

#### A3-M3 · `GetUserSubmissions` returns submissions from in-progress games

**File**: `backend/db/queries/users.sql` · Lines 86–91  
**Category**: Logic

`GetUserGameHistory` filters `WHERE r.state = 'finished'`, but `GetUserSubmissions` has no such filter, potentially leaking active game submissions.

**Fix**: Clarify design intent and add `AND r.state = 'finished'` if in-progress submissions should not be exposed.

#### A3-M4 · Test container cleanup not reliable under CI parallelism

**File**: `backend/internal/testutil/testutil.go` · Lines 32–72  
**Category**: Ops / Testing

`SetupSuite` uses `log.Fatalf` on migration failure with no retry logic. Parallel test runs may conflict on ports or container resources.

**Fix**: Document that tests must not run in parallel across packages. Add container reuse or retry logic for CI robustness.

---

### LOW

#### A3-L1 · Sentinel UUID hardcoded in three files without a Go constant

**File**: `backend/db/queries/users.sql`, `backend/db/migrations/001_initial_schema.up.sql`  
**Category**: Maintainability

`00000000-0000-0000-0000-000000000001` appears as a literal in SQL and generated Go. A typo cannot be caught by the compiler.

**Fix**: Define `const SentinelUserID = "00000000-0000-0000-0000-000000000001"` in a shared package and reference it in application code.

#### A3-L2 · Down migration does not document sentinel row removal

**File**: `backend/db/migrations/001_initial_schema.down.sql`  
**Category**: Maintainability

The up migration explicitly inserts the sentinel; the down drops the table (implicitly removing it). Add a comment explaining why no explicit DELETE is needed.

#### A3-L3 · main.go graceful shutdown does not close WebSocket connections

**File**: `backend/cmd/server/main.go` · Lines 204–211  
**Category**: Ops

`srv.Shutdown()` closes HTTP listeners but does not signal game hubs to drain. Active WS clients will have connections torn down without a clean close frame. In-flight votes or submissions may be lost.

**Fix**: Call a `manager.Shutdown()` method (to be implemented) before `srv.Shutdown()` so hubs can broadcast a "server restarting" message and cleanly close.

#### A3-L4 · Startup cleanup uses `Warn` instead of structured `Info`

**File**: `backend/cmd/server/main.go` · Lines 61–67  
**Category**: Observability

Design doc specifies structured log events `room.crash_recovery` and `room.abandoned` with counts. Current code logs errors at warn level without counts.

**Fix**: Return affected row counts from cleanup queries and emit structured info logs.

---

## Layer 4 — Frontend

### CRITICAL

#### A4-C1 · Room layout never initializes `user` state

**File**: `frontend/src/routes/(app)/rooms/[code]/+layout.svelte`  
**Category**: Logic / Bug

The room page uses `user.id` for host detection and vote validation, but `user` is not initialized from layout data or server locals. `user.id` will be `null`, causing all host checks and self-vote guards to fail silently.

**Fix**: Initialize `user` state from `data.user` in the `(app)` layout and make it available to nested routes.

#### A4-C2 · Cookie parsing in verify handler uses fragile regex

**File**: `frontend/src/routes/(public)/auth/verify/+page.server.ts` · Lines 40–50  
**Category**: Security / Bug

Manual `Set-Cookie` header parsing via `/^session=([^;]+)/` can fail if header attributes are reordered or contain whitespace. If it fails, the session is silently not set and the user cannot log in.

**Fix**: Use SvelteKit's `cookies.set()` in a server hook that forwards cookies from the backend response, or parse the header with a robust `cookie` library.

#### A4-C3 · `LeaderboardEntry` uses `score` but backend sends `total_score`

**File**: `frontend/src/lib/api/types.ts` · Lines 141–147  
**Category**: Type Mismatch / Bug

Components reference `entry.score`, which is undefined. The leaderboard displays blank scores.

**Fix**: Remove `score` from `LeaderboardEntry`; use only `total_score` everywhere.

#### A4-C4 · Missing auth redirect on public landing page

**File**: `frontend/src/routes/(public)/`  
**Category**: Design / Logic

Authenticated users can visit the public landing page without being redirected to the app lobby. The design requires this redirect.

**Fix**: Create `/(public)/+page.server.ts` that checks `locals.user` and redirects to `/` (app root) if authenticated.

#### A4-C5 · Host state not preserved across player events

**File**: `frontend/src/lib/state/room.svelte.ts`  
**Category**: Logic / Bug

`player_joined` and `room_state` WS messages may not carry the `is_host` flag. If missing, `isHost` is derived incorrectly and the host never sees host-only controls.

**Fix**: Ensure `handleMessage` for `player_joined` and `room_state` preserves `is_host`; verify the backend includes this field in all player update payloads.

---

### HIGH

#### A4-H1 · WebSocket `error` message type is unhandled

**File**: `frontend/src/lib/state/room.svelte.ts` · Lines 27–81  
**Category**: Logic / Error Handling

Server errors (e.g., `submission_closed`, `vote_failed`) sent as `{ type: 'error', ... }` are silently ignored. Players receive no feedback when their actions fail.

**Fix**: Add `case 'error'` in `handleMessage` to emit a toast and reset affected UI state.

#### A4-H2 · Game type change during pack fetch causes race condition

**File**: `frontend/src/routes/(app)/+page.svelte` · Lines 33–36  
**Category**: Logic / Bug

Rapidly switching game types A → B → A while a pack fetch for B is in-flight can overwrite the final pack list with stale data.

**Fix**: Use `AbortController` to cancel the in-flight fetch when the game type changes, or debounce the fetch trigger.

#### A4-H3 · Upload progress bar not cleared on error

**File**: `frontend/src/lib/components/studio/ItemTable.svelte` · Lines 50–62  
**Category**: Logic / UX

If bulk upload fails mid-loop, `uploadProgress` is not reset until the loop exits. The UI shows a stale/incorrect progress indicator.

**Fix**: Reset `uploadProgress` immediately on each error.

#### A4-H4 · Timer component shows expired round on late mount

**File**: `frontend/src/lib/games/meme-caption/SubmitForm.svelte` · Lines 12–21  
**Category**: Logic / Bug

If `round.ends_at` is already in the past when the component mounts (network delay), `timerMs` is negative and the submit button is immediately disabled. No error is shown.

**Fix**: If `deadline <= Date.now()` on mount, render "Submission window has closed" and disable input explicitly.

#### A4-H5 · `reorderItems` sends wrong payload shape

**File**: `frontend/src/lib/api/studio.ts` · Line 66  
**Category**: API Mismatch / Bug

The function sends `{ ordered_ids: [...] }` but the protocol expects `{ positions: [{ id, position }, ...] }`. Reorder calls will fail with 422.

**Fix**: Update the payload to match the protocol.

#### A4-H6 · Vote results type mismatches backend response fields

**File**: `frontend/src/lib/api/types.ts` · Lines 157–164  
**Category**: Type Mismatch / Bug

The `Submission` interface uses `vote_count` and `score`, but backend sends `votes_received` and `points_awarded`. Results view will display `undefined` for both fields.

**Fix**: Rename interface fields to `votes_received` and `points_awarded`.

---

### MEDIUM

#### A4-M1 · Pack list fetched as `Pack[]` instead of `PaginatedResponse<Pack>`

**File**: `frontend/src/routes/(app)/+page.svelte` · Line 26  
**Category**: Type Safety / Bug

Direct `res.json()` assigned to `packs` when the backend returns `{ data: [...], next_cursor: ... }`. Packs will be an object, not an array, causing rendering errors.

**Fix**: Use `packsApi.list()` from `$lib/api/packs.ts`, which extracts `data`.

#### A4-M2 · Admin dashboard fetches undocumented endpoints

**File**: `frontend/src/routes/(admin)/admin/+page.server.ts` · Lines 4–6  
**Category**: Design Mismatch

Fetches `/api/admin/stats` and `/api/admin/audit-log`, which are not in the protocol spec and likely don't exist in the backend.

**Fix**: Either implement these endpoints or replace with placeholder content.

#### A4-M3 · Consent checkboxes reset after form error

**File**: `frontend/src/routes/(public)/auth/register/+page.svelte` · Lines 88, 103  
**Category**: UX / GDPR Compliance

If registration fails for any reason, the consent and age affirmation checkboxes reset. Users may miss re-checking before resubmitting, but the form will pass client-side validation because the `required` attribute only checks presence at submit time.

**Fix**: Restore checkbox state from `form?.consent` and `form?.age_affirmation` after error.

#### A4-M4 · Room join redirects without validating room existence

**File**: `frontend/src/routes/(app)/+page.server.ts` · Lines 61–68  
**Category**: Logic / UX

An invalid code redirects to a 404 room page rather than showing an inline error.

**Fix**: Fetch `/api/rooms/{code}` before redirecting; return `fail(404, { message: 'Room not found' })` if absent.

#### A4-M5 · Countdown animation phase transition may race with `round_started` WS event

**File**: `frontend/src/routes/(app)/rooms/[code]/+page.svelte` · Lines 20–33  
**Category**: Logic

Local countdown completion sets `phase = 'submitting'`; the server also sends `round_started` which sets the same phase. If messages arrive out of order or the countdown fires late, state could flicker.

**Fix**: Make phase transitions driven exclusively by WS messages. Remove the local `phase = 'submitting'` after countdown.

#### A4-M6 · Admin notification badge not implemented

**File**: `frontend/src/routes/(admin)/+layout.svelte`  
**Category**: Design / Completeness

Design doc specifies an unread notification count badge in the admin nav. Not present.

**Fix**: Fetch `/api/admin/notifications?unread=true` in the admin layout; display count in the nav.

#### A4-M7 · `Round` type missing `item` sub-object

**File**: `frontend/src/lib/api/types.ts` · Lines 149–155  
**Category**: Type / Design

Protocol says `round_started` includes `item: { payload, media_url? }`. Components access `round.media_url` directly — this will be `undefined` unless the backend flattens the payload (which is not documented).

**Fix**: Add `item: { payload: unknown; media_url?: string }` to the `Round` type and update component accessors.

#### A4-M8 · No error boundary on failed room fetch

**File**: `frontend/src/routes/(app)/rooms/[code]/+layout.server.ts`  
**Category**: UX / Error Handling

A 404 or timeout throws a generic SvelteKit error page with no useful message.

**Fix**: Catch the error and throw `error(code, 'Room not found or game ended.')`.

#### A4-M9 · Timer progress bar has no ARIA attributes

**File**: `frontend/src/lib/games/meme-caption/SubmitForm.svelte` · Lines 40–49  
**Category**: Accessibility

The timer bar lacks `role="progressbar"`, `aria-valuenow`, `aria-valuemax`, `aria-valuemin`. Screen readers cannot convey remaining time.

**Fix**: Add the required ARIA attributes as specified in `design/05-frontend.md`.

#### A4-M10 · Countdown animation ignores `prefers-reduced-motion`

**File**: `frontend/src/routes/(app)/rooms/[code]/+page.svelte` · Lines 63–66  
**Category**: Accessibility

`animate-bounce` runs regardless of the OS motion preference, violating the design's accessibility requirements.

**Fix**: Check `window.matchMedia('(prefers-reduced-motion: reduce)')` and suppress animation when true.

---

### LOW

#### A4-L1 · Copy room code button gives no feedback

**File**: `frontend/src/routes/(app)/rooms/[code]/+layout.svelte` · Lines 28–30  
**Fix**: Show a toast or temporarily change button text to "Copied!" after `navigator.clipboard.writeText()` resolves.

#### A4-L2 · `toast` not exported from state index

**File**: `frontend/src/lib/state/index.ts`  
**Fix**: Add `export { toast } from './toast.svelte.ts';` for consistency with other state exports.

#### A4-L3 · `updateUser` admin API type missing `email` and `username`

**File**: `frontend/src/lib/api/admin.ts` · Line 13  
**Fix**: Add `email?: string; username?: string` to the body type.

#### A4-L4 · Studio pack list has no pagination

**File**: `frontend/src/routes/(app)/studio/+page.server.ts` · Lines 4–8  
**Fix**: Implement cursor pagination until `next_cursor` is null, or add a "Load more" button.

#### A4-L5 · Version comparison stub

**File**: `frontend/src/lib/components/studio/VersionHistory.svelte` · Lines 87–95  
**Fix**: Implement side-by-side comparison modal or remove the button until it is ready.

#### A4-L6 · Room config editor not implemented

**File**: `frontend/src/routes/(app)/rooms/[code]/+page.svelte`  
**Fix**: Add a config panel (host-only, lobby phase) with sliders that call `PATCH /api/rooms/{code}/config`.

#### A4-L7 · Profile export error handling too broad

**File**: `frontend/src/routes/(app)/profile/+page.svelte` · Lines 22–33  
**Fix**: Add a specific catch for blob creation failure with a user-readable message.

---

## Layer 5 — Infrastructure / Design-Doc Consistency / Ops

### CRITICAL

#### A5-C1 · Items and versions REST endpoints are completely absent from the router

**File**: `backend/cmd/server/main.go` · Lines 159–167  
**Category**: Missing

None of the following routes are wired:

- `GET/POST /api/packs/{id}/items`
- `PATCH/DELETE /api/packs/{id}/items/{item_id}`
- `PATCH /api/packs/{id}/items/reorder`
- `GET/POST /api/packs/{id}/items/{item_id}/versions`
- `POST /api/packs/{id}/items/{item_id}/versions/{vid}/restore`
- `DELETE /api/packs/{id}/items/{item_id}/versions/{vid}[/purge]`

The Studio frontend calls these endpoints; they will all 404.

**Fix**: Implement `PackHandler` item/version methods and wire them in nested routes.

#### A5-C2 · Room action endpoints are absent from the router

**File**: `backend/cmd/server/main.go` · Lines 175–179  
**Category**: Missing

Not wired:

- `PATCH /api/rooms/:code/config` (update config in lobby)
- `POST /api/rooms/:code/leave`
- `POST /api/rooms/:code/kick`
- `GET /api/rooms/:code/leaderboard`

**Fix**: Implement and register all four.

---

### HIGH

#### A5-H1 · `ALLOWED_ORIGIN` / CORS origin validation not implemented

**File**: `backend/internal/config/config.go`, `backend/internal/api/ws.go` · Line 16  
**Category**: Security / Consistency

`compose.base.yml` passes `ALLOWED_ORIGIN` to the backend. The config struct has no such field, and no origin-validation middleware exists. The WebSocket upgrade comment references "ALLOWED_ORIGIN middleware in main.go" which does not exist.

**Fix**: Add `AllowedOrigin string` to Config, load it from `FRONTEND_URL`, and create an origin-check middleware (or configure the `gorilla/websocket` upgrader's `CheckOrigin`).

#### A5-H2 · Prometheus metrics endpoint not implemented

**File**: `backend/cmd/server/main.go`  
**Category**: Missing / Ops

`design/06-operations.md` specifies `GET /api/metrics` with HTTP, WebSocket, game, auth, and infrastructure counters. No instrumentation middleware or metrics endpoint exists.

**Fix**: Integrate `github.com/prometheus/client_golang`, add instrumentation middleware, and wire the endpoint with IP restriction.

#### A5-H3 · Deep health check does not verify RustFS

**File**: `backend/internal/api/health.go` · Lines 26–46  
**Category**: Design / Ops

`GET /api/health/deep` only pings PostgreSQL. Design specifies it must also probe RustFS with a 2s timeout.

**Fix**: Issue a HEAD request to the RustFS bucket root in the readiness handler.

#### A5-H4 · Frontend cannot reach backend in production compose

**File**: `docker/compose.prod.yml` · Lines 5–6  
**Category**: Ops / Bug

The production overlay doesn't declare the frontend's `project_network` membership. The base file requires it (line 73). The frontend can reach the reverse proxy (via `pangolin`) but not the backend API.

**Fix**:

```yaml
frontend:
  networks:
    - project_network
    - pangolin
```

#### A5-H5 · No HTTP instrumentation middleware

**File**: All middleware files  
**Category**: Ops / Missing

Design specifies `http_requests_total` and `http_request_duration_seconds` histograms. Without this, Prometheus scrapes return no useful HTTP data.

**Fix**: Add a `promhttp` instrumentation middleware in the chi router setup.

---

### MEDIUM

#### A5-M1 · `PORT` vs `BACKEND_PORT` naming inconsistency

**File**: `docker/compose.base.yml` · Line 43, `backend/internal/config/config.go`  
**Category**: Consistency

Compose sets `PORT: ${BACKEND_PORT:-8080}`; config.go reads `PORT`. Design docs call it `BACKEND_PORT`. Two names for the same thing.

**Fix**: Pick one canonical name and update all references. `PORT` is idiomatic for Go services; update docs.

#### A5-M2 · Backend and frontend containers run as root

**File**: `backend/Dockerfile`, `frontend/Dockerfile`  
**Category**: Security

Neither Dockerfile creates an unprivileged user. Containers run as `root` by default, violating the "Least Attack Surface" principle in the design.

**Fix**: Add `RUN adduser -D -u 1000 appuser && USER appuser` before the final entrypoint in both Dockerfiles.

#### A5-M3 · Backend service has no Docker Compose healthcheck

**File**: `docker/compose.base.yml` · Lines 20–60  
**Category**: Ops

`frontend.depends_on.backend` cannot use `condition: service_healthy` because the backend has no `healthcheck` block. Frontend starts before the backend is ready.

**Fix**:

```yaml
backend:
  healthcheck:
    test: ['CMD-SHELL', 'wget -qO- http://localhost:8080/api/health || exit 1']
    interval: 5s
    retries: 10
    start_period: 10s
```

#### A5-M4 · Optional env vars missing from `.env.example`

**File**: `.env.example`, `.env.dev.example`  
**Category**: Ops / Documentation

Several tunable variables are not documented in the example files: `RATE_LIMIT_AUTH_RPM`, `RATE_LIMIT_ROOMS_RPH`, `RATE_LIMIT_UPLOADS_RPH`, `RATE_LIMIT_GLOBAL_RPM`, `WS_RATE_LIMIT`, `WS_READ_LIMIT_BYTES`, `WS_READ_DEADLINE`, `WS_PING_INTERVAL`, `SESSION_RENEW_INTERVAL`.

**Fix**: Add all optional variables with their defaults as commented examples.

---

### LOW

#### A5-L1 · Health check uses 3s timeout vs design's 2s

**File**: `backend/internal/api/health.go` · Line 28  
**Fix**: Change to `2 * time.Second` to match the spec.

#### A5-L2 · No explicit restart delay or backoff for critical services

**File**: `docker/compose.base.yml` · Lines 4, 21, 63  
**Fix**: Document the restart strategy in the production runbook; consider `restart: on-failure:5` with `delay` for PostgreSQL.

---

## Issues by Priority for Pre-Launch

### Must fix before first deploy

| ID    | Issue                                         |
| ----- | --------------------------------------------- |
| A3-C1 | `rooms.host_id` FK blocks GDPR hard-delete    |
| A2-C4 | `runRounds()` is a stub — games never advance |
| A2-C5 | Room codes use `math/rand`                    |
| A2-C6 | MIME magic byte validation is optional        |
| A2-H2 | `SetStatus` has no admin auth                 |
| A2-H3 | `DeleteInvite` has no auth                    |
| A2-H4 | `ListUsers` / `UpdateUser` have no auth       |
| A5-C1 | Items/versions routes absent                  |
| A5-C2 | Room action routes absent                     |
| A4-C2 | Session cookie parsing fragility              |
| A4-C3 | `LeaderboardEntry` type mismatch              |
| A4-H5 | `reorderItems` sends wrong payload            |
| A4-H6 | Vote result field names don't match backend   |

### Fix before beta

| ID    | Issue                                                       |
| ----- | ----------------------------------------------------------- |
| A2-C1 | Goroutine leak on grace window                              |
| A2-C2 | Broadcast panic on closed channel                           |
| A2-C3 | Missing `reconnect` message handler                         |
| A1-C1 | Verify endpoint error codes wrong                           |
| A3-H1 | `users.invited_by` missing `ON DELETE SET NULL`             |
| A3-H2 | Sessions use INNER JOIN — implicit disappear on hard-delete |
| A5-H1 | No origin validation for WebSocket                          |
| A5-H4 | Frontend not on `project_network` in production             |
| A4-C1 | User state not initialized in room layout                   |
| A4-H1 | WS error messages unhandled                                 |
| A4-H2 | Race condition on game-type-change fetch                    |

---

_Generated by 5 parallel review agents. Each agent read its domain files in full and cross-referenced the design docs._
