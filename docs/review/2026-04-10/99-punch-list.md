# Stage 7 — Synthesis & Prioritized Punch List

Date: 2026-04-10
Scope: executive synthesis of the full review. No new findings — this consolidates Stages 0–6 into a single prioritized action list and an executive summary suitable for a status update.

## 7.1 TL;DR

**FabDoYouMeme builds cleanly, has a surprisingly mature test infrastructure, and solves several hard problems well** (single-goroutine hub state, testcontainers-based tests, sentinel-UUID GDPR model, opaque session architecture). **But** it has three genuinely critical defects that prevent the product from functioning as described:

1. **The WebSocket gameplay flow is end-to-end broken.** No production code path creates a `Hub`, so every player joining any room receives a 404. The unit tests miss this because they construct hubs directly.
2. **Rate limiting and `RequirePrivateIP` are both broken because the app is proxy-blind.** Behind the reverse proxy mandated by the architecture, every request looks like it comes from the proxy's IP, so all clients share one rate bucket and `/api/metrics` is either fully open or fully closed.
3. **`/api/assets/download-url` has no authorization.** Any logged-in user can download any media_key they can see or guess.

All three are CI-invisible today — they would be caught by a single end-to-end test each. The test infrastructure to write those tests already exists; what's missing is a `Clock` seam and a handful of `testutil` helpers.

A fourth issue — `SessionLookupFn` writes to the DB on every authenticated request, ignoring `SessionRenewInterval` — is a silent performance degradation rather than a correctness bug, but fixes are small and high-value.

**Estimated time to "production-ready with CI-enforced quality gates": ~17 focused engineering days.**

---

## 7.2 What the audit found well

Recording the good news so that Stage 7 work does not accidentally regress any of it:

| Area                                    | Why it's good                                                                                                                                                                                  |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Test foundation**                     | `testcontainers-go` already in place, shared pool, `WithTx` rollback, 28 test files, 10/10 packages pass under `-race`. Stage 6 is additive, not replacement.                                  |
| **Hub design**                          | Single-goroutine state ownership in `Run()` loop, no mutexes on internal state, careful reconnect-path channel close in `handleRegister`. The concurrency model is clearly thought through.    |
| **Auth architecture**                   | Opaque tokens (ADR-001), SHA-256 hashed storage, HttpOnly+Secure+SameSite=Strict cookies, magic-link-only (no passwords), atomic one-time-use consumption query. Matches modern best practice. |
| **GDPR implementation**                 | Sentinel UUID for hard-delete reassignment in both `submissions` and `votes`, consent_at set once, soft-delete cascades with partial indexes. Well-executed.                                   |
| **Seams already in place**              | `storage.Storage`, `auth.EmailSender`, `sqlc DBTX`, `game.GameTypeHandler`, `middleware.SessionLookupFn`. Five interfaces that make handler tests possible without mocks.                      |
| **Schema rigor**                        | 14 tables with CHECK constraints, partial indexes, foreign key cascades, deferred FKs to avoid cycles. No casual schema work.                                                                  |
| **Startup cleanup**                     | `FinishCrashedRooms` + `FinishAbandonedLobbies` on every boot — idempotent crash recovery without manual ops.                                                                                  |
| **Registration enumeration resistance** | Existing email returns 201 silently, MagicLink always returns 200, restricted-email mismatch reuses the invalid-invite error code. Account enumeration is properly mitigated.                  |
| **MIME validation**                     | Magic-byte check via `storage.ValidateMIME` before pre-signing upload URL. Correct defense against disguised uploads.                                                                          |
| **Docs ↔ code alignment**               | 13 docs files in `docs/`, all in git, architecture and ADRs match the code in ~95% of spot-checks. Drifts are small and fixable.                                                               |

This is a codebase written by someone who read the right books. The findings below are not "this is a mess"; they are "this is a good codebase that needs its last 5% of rigor".

---

## 7.3 Findings roll-up

Every finding from Stages 1–5, in one table.

| Stage | #    | Severity    | Finding                                                                         |
| ----- | ---- | ----------- | ------------------------------------------------------------------------------- |
| 1     | 1.1  | 🟡 med      | 9 Svelte 5 `state_referenced_locally` warnings — potential stale-data bugs      |
| 1     | 1.2  | 🟡 med      | 7 a11y warnings (5 autofocus + 2 unassociated labels)                           |
| 1     | 1.3  | 🟡 low      | 1 potential sqlc drift on `users.sql` (mtime heuristic)                         |
| 1     | 1.4  | 🟡 med      | CI has no `sqlc diff` / `sqlc verify` gate                                      |
| 1     | 1.5  | 🔵 info     | 2 govulncheck vulns — test-path only                                            |
| 1     | 1.6  | 🔵 info     | 3 low-severity npm audit (cookie <0.7.0 via SvelteKit)                          |
| 1     | 1.7  | 🟡 low      | `config.Load()` swallows `url.Parse` error for `CookieDomain` (promoted to 4.D) |
| 1     | 1.8  | 🟡 low      | `config.Load()` has no bounds validation (promoted to 4.E)                      |
| 1     | 1.9  | 🟡 med      | CI `backend.yml` starts a Postgres service container the tests never use        |
| 1     | 1.10 | 🟡 low      | CI `frontend.yml` runs zero tests                                               |
| 1     | 1.11 | 🟡 low      | `sqlc.yaml` path not mentioned in CLAUDE.md                                     |
| 3     | 3.A  | 🔴 **CRIT** | Hub never created in production → WS returns 404 on every room                  |
| 3     | 3.B  | 🔴 HIGH     | `finishRoom` doesn't cancel `runRounds` → ghost rounds                          |
| 3     | 3.C  | 🟠 HIGH     | `SessionLookupFn` renews on every request; `SessionRenewInterval` dead config   |
| 3     | 3.D  | 🟡 med      | `handleRegister` doesn't enforce `max_players`                                  |
| 3     | 3.E  | 🟡 med      | `Leave` handler no state/host check — docs drift                                |
| 3     | 3.F  | 🟡 med      | `Leaderboard` handler no finished-state check — docs drift                      |
| 3     | 3.G  | 🔵 low      | `votes.value` hardcoded — design smell                                          |
| 3     | 3.H  | 🔵 low      | `system:kick` unmarshal errors swallowed                                        |
| 4     | 4.A  | 🔴 HIGH     | `RateLimiter.evictLoop` no stop — 5 leaked goroutines                           |
| 4     | 4.B  | 🔴 HIGH     | `KickPlayer` bare-send can block HTTP goroutine                                 |
| 4     | 4.C  | 🟠 HIGH     | `manager.Shutdown()` doesn't cancel hubs, magic sleep                           |
| 4     | 4.D  | 🟠 med      | `CookieDomain` `url.Parse` error swallowed                                      |
| 4     | 4.E  | 🟡 med      | No bounds validation on duration/int env vars                                   |
| 4     | 4.F  | 🟡 med      | `runRounds` goroutine leak on `finishRoom` (= 3.B)                              |
| 4     | 4.G  | 🟡 med      | `graceExpired` buffer exhaustion edge case                                      |
| 4     | 4.H  | 🔵 low      | 3x silent `json.Unmarshal` errors                                               |
| 4     | 4.I  | 🔵 low      | `safeSend` drops messages silently                                              |
| 5     | 5.A  | 🔴 **CRIT** | `/api/assets/download-url` no authorization                                     |
| 5     | 5.B  | 🔴 **CRIT** | Proxy-blind IP handling breaks rate limiting & `RequirePrivateIP`               |
| 5     | 5.C  | 🟠 HIGH     | WS `CheckOrigin` exact-match is fragile                                         |
| 5     | 5.D  | 🟠 HIGH     | No CSRF token — SameSite=Strict is sole defense                                 |
| 5     | 5.E  | 🟡 med      | No username/email validation in Go layer                                        |
| 5     | 5.F  | 🟡 med      | govulncheck CI behavior inconsistency                                           |
| 5     | 5.G  | 🔵 low      | Audit logging consistency needs spot-check                                      |
| 5     | 5.H  | 🔵 low      | `/api/users/me/export` no per-user rate limiter                                 |

**Totals**: 3 critical, 7 high, 14 medium, 10 low, 3 info.

---

## 7.4 Prioritized punch list

Grouped by the order they should be tackled. Each item lists: finding ID(s), effort, files touched, acceptance test.

### P0 — Foundation (unblocks everything else)

| #    | Done | Title                                                                                     | Effort | Files                                                                  | Acceptance                                              |
| ---- | ---- | ----------------------------------------------------------------------------------------- | ------ | ---------------------------------------------------------------------- | ------------------------------------------------------- |
| P0.1 | ✅   | Add `clock.Clock` interface with `Real` + `Fake` implementations                          | 0.5d   | `backend/internal/clock/{clock,real,fake}.go` (new)                    | `FakeClock` unit tests pass; `Advance` is deterministic |
| P0.2 | ✅   | Plumb clock into `hub.go`, `auth/handler.go`, `middleware/rate_limit.go`                  | 1d     | 3 files + all call sites of `time.Now`, `time.After`, `time.AfterFunc` | `go build ./...` passes; existing tests still green     |
| P0.3 | ✅   | Add `testutil` helpers: `FakeStorage`, `FakeEmail`, `WSTestClient`, `HTTPTest`, factories | 2d     | `backend/internal/testutil/*.go` (new files)                           | Each helper has its own unit test                       |

**Gate**: these three items must complete before any P1 item starts. They unblock 16 regression tests. — **cleared 2026-04-10**.

### P1 — Must-fix critical defects

| #    | Done | Title                                                                                               | Finding  | Effort | Files                                                                                                    | Acceptance test                            |
| ---- | ---- | --------------------------------------------------------------------------------------------------- | -------- | ------ | -------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| P1.1 | ✅   | Create hubs in WS upgrade path with server-scoped context                                           | 3.A, 4.C | 1.5d   | `api/ws.go`, `game/manager.go`, `cmd/server/main.go`                                                     | `TestE2E_WebSocketHappyPath`               |
| P1.2 | ✅   | Cancel `runRounds` from `finishRoom` via dedicated roundsCtx                                        | 3.B, 4.F | 0.5d   | `game/hub.go`                                                                                            | `TestE2E_HostDisconnectFinishesGame`       |
| P1.3 | ✅   | Respect `SessionRenewInterval` in `SessionLookupFn`                                                 | 3.C      | 0.5d   | `auth/handler.go`                                                                                        | `TestSession_RenewAtMostOncePerInterval`   |
| P1.4 | ✅   | Authorize `DownloadURL` via new `CanUserDownloadMedia` query                                        | 5.A      | 1d     | `api/assets.go`, `db/queries/game_packs.sql`, regen sqlc                                                 | `TestAPI_DownloadURLAuthzMatrix` (9 cases) |
| P1.5 | ✅   | Add `middleware.ClientIP` trusted-proxy walker; replace `r.RemoteAddr` in rate_limit + ip_allowlist | 5.B      | 1d     | `middleware/real_ip.go` (new), `rate_limit.go`, `ip_allowlist.go`, `config.go` (TRUSTED_PROXIES env var) | `TestMiddleware_ClientIPTrustedProxyWalk`  |

**Gate**: all five P1 items must pass their acceptance tests before any deployment claiming "beta-ready".

### P2 — Should-fix high-severity

| #    | Done | Title                                                                  | Finding  | Effort | Files                                                                   | Acceptance                                  |
| ---- | ---- | ---------------------------------------------------------------------- | -------- | ------ | ----------------------------------------------------------------------- | ------------------------------------------- |
| P2.1 | ☐    | Add `Stop()` to `RateLimiter` and call on shutdown                     | 4.A      | 0.25d  | `middleware/rate_limit.go`, `cmd/server/main.go`                        | `goleak` check in CI passes                 |
| P2.2 | ☐    | `KickPlayer(ctx)` with select/timeout, update call site                | 4.B      | 0.25d  | `game/hub.go`, `api/room_actions.go`                                    | `TestHub_KickPlayerRespectsContext`         |
| P2.3 | ☐    | Enforce `max_players` in `handleRegister` via handler interface        | 3.D      | 0.5d   | `game/hub.go`, `game/registry.go`, `game/types/meme_caption/handler.go` | `TestHub_RejectJoinWhenFull`                |
| P2.4 | ☐    | Add state/host guards to `Leave`; host-leaves-closes-room              | 3.E      | 0.5d   | `api/room_actions.go`                                                   | `TestAPI_LeaveRejectsPlaying`               |
| P2.5 | ☐    | Add finished-state guard to `Leaderboard`                              | 3.F      | 0.1d   | `api/room_actions.go`                                                   | `TestAPI_LeaderboardRejectsUnfinished`      |
| P2.6 | ☐    | Normalize WS `CheckOrigin` — trim trailing slash, support list         | 5.C      | 0.25d  | `api/ws.go`, `config.go`                                                | `TestWS_CheckOriginNormalizesTrailingSlash` |
| P2.7 | ☐    | Add ADR-011 for SameSite CSRF stance + lint rule for `SameSite=Strict` | 5.D      | 0.5d   | `docs/reference/decisions.md`, `.github/workflows/backend.yml`          | lint fails if SameSite relaxed              |
| P2.8 | ☐    | Fail-loud on bad `FRONTEND_URL`; bounds-check all duration/int env     | 4.D, 4.E | 0.5d   | `config/config.go`                                                      | `TestConfig_LoadRejects*` table             |
| P2.9 | ☐    | Validate username + email in Go before DB write                        | 5.E      | 0.25d  | `auth/register.go`, `auth/profile.go`                                   | `TestAuth_ValidateUsernameTable`            |

### P3 — Nice-to-have robustness & observability

| #    | Done | Title                                                                    | Finding   | Effort | Files                                                                      |
| ---- | ---- | ------------------------------------------------------------------------ | --------- | ------ | -------------------------------------------------------------------------- |
| P3.1 | ☐    | Fix Svelte 5 `state_referenced_locally` warnings via `$derived` audit    | 1.1       | 1d     | 9 `.svelte` files                                                          |
| P3.2 | ☐    | Fix a11y warnings (autofocus via onMount, label `for=`)                  | 1.2       | 0.5d   | 6 `.svelte` files                                                          |
| P3.3 | ☐    | Log + return early on `json.Unmarshal` errors in hub and runRounds       | 4.H, 3.H  | 0.25d  | `game/hub.go`                                                              |
| P3.4 | ☐    | Close slow WS consumers on drop; add `ws_messages_dropped_total` counter | 4.I       | 0.5d   | `game/hub.go`, `middleware/metrics.go`                                     |
| P3.5 | ☐    | Non-blocking `graceExpired` send with default-drop                       | 4.G       | 0.1d   | `game/hub.go`                                                              |
| P3.6 | ☐    | Per-user rate limiter on `/api/users/me/export`                          | 5.H       | 0.25d  | `cmd/server/main.go`, `middleware/rate_limit.go` (per-user bucket variant) |
| P3.7 | ☐    | Audit logging consistency spot-check, add missing entries                | 5.G       | 0.5d   | `api/admin.go`, `auth/*.go`                                                |
| P3.8 | ☐    | Investigate govulncheck CI behavior; add `.govulncheck.yaml` suppression | 5.F, 1.5  | 0.5d   | `.github/workflows/backend.yml`, `backend/.govulncheck.yaml` (new)         |
| P3.9 | ☐    | Update CLAUDE.md compose paths + sqlc.yaml location                      | 0.2, 1.11 | 0.1d   | `CLAUDE.md`                                                                |

### P4 — CI overhaul

| #    | Done | Title                                                                                                  | Effort | Files                                                          |
| ---- | ---- | ------------------------------------------------------------------------------------------------------ | ------ | -------------------------------------------------------------- |
| P4.1 | ☐    | New `backend.yml`: parallel jobs, drop wasted postgres service, add sqlc-verify, coverage, e2e, goleak | 0.5d   | `.github/workflows/backend.yml`                                |
| P4.2 | ☐    | New `frontend.yml`: add vitest step, bump to Node 22                                                   | 0.25d  | `.github/workflows/frontend.yml`                               |
| P4.3 | ☐    | Add `CODEOWNERS` requiring review on auth/middleware/main                                              | 0.1d   | `.github/CODEOWNERS` (new)                                     |
| P4.4 | ☐    | Enable Dependabot + CodeQL                                                                             | 0.1d   | `.github/dependabot.yml`, `.github/workflows/codeql.yml` (new) |

### P5 — Frontend test bootstrap

| #    | Done | Title                                                           | Effort | Files                                                                |
| ---- | ---- | --------------------------------------------------------------- | ------ | -------------------------------------------------------------------- |
| P5.1 | ☐    | Add vitest + @testing-library/svelte; ~10 component/state tests | 1.5d   | `frontend/vitest.config.ts` (new), `frontend/src/**/*.test.ts` (new) |
| P5.2 | ☐    | Add Playwright E2E smoke test (register → play → results)       | 0.5d   | `frontend/tests/e2e/*.spec.ts` (new)                                 |

---

## 7.5 Recommended sequencing

If the user wants to ship incrementally, here is the suggested order of PRs:

| PR  | Contents                                                                   | Time  |
| --- | -------------------------------------------------------------------------- | ----- |
| 1   | P0.1 — Clock interface                                                     | 0.5d  |
| 2   | P0.2 — Plumb clock into 3 packages                                         | 1d    |
| 3   | P0.3 — testutil helpers + factories                                        | 2d    |
| 4   | P1.3 — Session renewal fix + test (smallest P1, validates helpers)         | 0.5d  |
| 5   | P1.2 — runRounds cancellation + test                                       | 0.5d  |
| 6   | **P1.1 — Hub creation in WS path** (the big one) + test                    | 1.5d  |
| 7   | P1.5 — ClientIP + rate limit + allowlist + tests                           | 1d    |
| 8   | P1.4 — DownloadURL authz matrix + test                                     | 1d    |
| 9   | P2.1 + P2.2 — RateLimiter.Stop + KickPlayer ctx                            | 0.5d  |
| 10  | P2.3 + P2.4 + P2.5 — max_players, Leave, Leaderboard guards                | 1.25d |
| 11  | P2.6 + P2.8 + P2.9 — CheckOrigin + config validation + username validation | 1d    |
| 12  | P2.7 — ADR-011 + SameSite lint                                             | 0.5d  |
| 13  | P3.1 + P3.2 — Svelte warnings + a11y                                       | 1.5d  |
| 14  | P3.x batch — small robustness fixes                                        | 1d    |
| 15  | **P4.1 + P4.2 — CI overhaul** (turn the lights on for regressions)         | 0.75d |
| 16  | P4.3 + P4.4 — CODEOWNERS, Dependabot, CodeQL                               | 0.2d  |
| 17  | P5.x — Frontend tests                                                      | 2d    |

PRs 1–3 unblock everything. PRs 4–8 are the critical-fix train. PR 15 is the moment where CI becomes a gate instead of decoration.

---

## 7.6 What to skip if time is short

If the user has only ~1 week instead of ~3:

**Do**: P0 (foundation) + P1 (5 must-fixes) + P4.1 (backend CI overhaul with the new regression tests as a gate). That's ~8.5 days, compressible to ~7 with tight scope.

**Skip**: P2.6 (CheckOrigin — rare in practice), P2.7 (ADR is documentation), P3 (all — nice-to-haves), P5 (frontend tests can wait one cycle).

**Do NOT skip**: any of P0 or P1. Skipping P0 means the P1 fixes cannot be locked in with tests, and the bugs will regress.

---

## 7.7 Assumptions and unknowns

Items that should be verified before starting:

1. **Does CI actually pass today on `main`?** Stage 5.F flagged govulncheck as an inconsistency — either the workflow has been silently red, or something suppresses the exit code. Check before touching CI.
2. **Is the reverse proxy architecture definitely pre-existing?** CLAUDE.md says so, but if the proxy is actually operator-configured, then 5.B becomes "operator-responsibility" for the XFF handling. Still want the fix, but the urgency shifts.
3. **Are there users already registered?** If yes, the GDPR audit-log spot-check (5.G) is more urgent; if no, it's cleanup.
4. **Does the operator use `DownloadURL` anywhere in current flows?** If the frontend never calls it, 5.A is theoretical until it's wired up. Grep the frontend.
5. **Is Svelte 5 `state_referenced_locally` actually producing wrong UI today?** The compiler warns but each case needs audit — some may be intentional snapshots.

---

## 7.8 What would change my recommendation

- **If `DownloadURL` is not currently used by any frontend code**: downgrade 5.A from CRIT to HIGH (still a latent vulnerability for future flows, but not a live leak).
- **If the reverse proxy enforces rate limiting itself**: downgrade 5.B's rate limit component (but not `RequirePrivateIP`).
- **If the operator has never configured `TRUSTED_PROXIES` (because the env var doesn't exist)**: accelerate P1.5 — the fix introduces a _new_ env var that operators must set.
- **If frontend tests show `state_referenced_locally` was intentional snapshots**: drop P3.1.

---

## 7.9 Full file map of review artifacts

```
docs/review/2026-04-10/
├── 00-inventory.md              Stage 0 — ground truth
├── 01-build-health.md           Stage 1 — build/vet/test/lint
├── 02-seams-and-testability.md  Stage 2 — seams analysis
├── 03-correctness.md            Stage 3 — correctness per domain
├── 04-robustness.md             Stage 4 — errors/concurrency/resources
├── 05-security.md               Stage 5 — security
├── 06-test-plan.md              Stage 6 — test suite & CI design
└── 99-punch-list.md             Stage 7 — this file
```

Each file is self-contained and cross-references siblings by section number. Stages 0–2 are read-only inventory; stages 3–5 surface findings; stages 6–7 prescribe fixes.

---

## 7.10 Closing note

The most important sentence in this review: **every critical bug we found is invisible to the current CI because no end-to-end test exists that exercises the broken path.** 3.A (hub creation) passes unit tests because the tests call `GetOrCreate` directly; 3.B (ghost rounds) passes unit tests because no test waits for grace expiry; 3.C (session renewal) passes because nothing times the DB writes; 5.A (download authz) passes because no test checks "user B downloads user A's media"; 5.B (proxy IPs) passes because all tests run with `RemoteAddr=127.0.0.1`.

The single highest-leverage change in this review is **adding the E2E test layer** (§6.5). Every other fix is scaffolding that enables E2E tests to exist. Without them, any Stage 7 fix will be reviewed by eye, merged in good faith, and regressed by the next Claude or contributor who doesn't have this document on hand. With them, the codebase defends itself.
