# Stage 0 — Inventory & Ground Truth

Date: 2026-04-10
Scope: read-only audit. No code was modified.

## 0.1 Repository layout vs. CLAUDE.md

| CLAUDE.md says…                                   | Reality                                                                                                                                            | Delta              |
| ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------ |
| `docker-compose.yml` at root                      | No. Compose files live under `docker/` (`compose.base.yml` + `compose.{dev,preprod,prod}.yml`).                                                    | 🟡 Docs drift      |
| `docker-compose.override.yml` at root             | No. Dev overrides are in `docker/compose.dev.yml`; chosen via the `Makefile`.                                                                      | 🟡 Docs drift      |
| `backend/internal/api/*` exists                   | Yes. Contains `rooms.go`, `ws.go`, `assets.go`, `packs.go`, `admin.go`, `items.go`, `versions.go`, `health.go`, `game_types.go`.                   | ✅                 |
| `backend/internal/game/registry.go` + hub + types | Yes. Plus `manager.go` (hub manager — not named in CLAUDE.md) and `message.go`.                                                                    | ✅ (+1 extra file) |
| `backend/internal/testutil/` for test helpers     | **Yes.** Uses `testcontainers-go` with a package-shared container + `WithTx` rollback. Exactly what we recommended for Stage 6 — already in place. | ✅✅               |
| Frontend `lib/games/meme-caption/`                | Yes: `SubmitForm`, `VoteForm`, `ResultsView`, `GameRules`.                                                                                         | ✅                 |
| `docs/` authoritative                             | Yes, 13 files present under `docs/` + `docs/reference/`.                                                                                           | ✅                 |
| `design/*` old plans                              | Deleted per `git status` (commit `5240160`). `CLAUDE.md` no longer references them.                                                                | ✅                 |

**Action for later**: refresh `CLAUDE.md` compose-file paths in Stage 7 punch list.

## 0.2 Backend package map

```
backend/
├── cmd/server/main.go                    ← wires everything; 269 lines
├── db/
│   ├── migrations/ (3 pairs up/down)     ← embedded via `migrations.FS`
│   │   ├── 001_initial_schema
│   │   ├── 002_seed_game_types
│   │   └── 003_schema_fixes
│   ├── queries/ (11 .sql files)          ← sqlc input
│   └── sqlc/ (11 .sql.go files)          ← sqlc output, generated
└── internal/
    ├── api/        9 handlers + ws + health       (+ 6 _test.go)
    ├── auth/       tokens, session, register, verify, magic_link, admin, profile, bootstrap, handler, sentinel, email  (+ 9 _test.go)
    ├── config/     env loader                                     (+ _test.go)
    ├── email/      go-mail + embedded HTML/text templates        (+ _test.go)
    ├── game/       registry, handler iface, hub, manager, message, types/meme_caption/  (+ 4 _test.go)
    ├── middleware/ auth, logging, metrics, request_id, rate_limit, ip_allowlist, context  (+ 5 _test.go)
    ├── storage/    Storage iface + S3 impl + MIME detection      (+ _test.go)
    └── testutil/   shared postgres testcontainer + WithTx helper
```

### 0.2.1 Test files already present

28 `_test.go` files spread across packages. Per-package `main_test.go` files in `db/`, `internal/api/`, `internal/auth/`, `internal/game/` call `testutil.SetupSuite(m)` → one container per package. The discipline is already in place.

## 0.3 Go module

- `go 1.25.0`
- **Already uses** `testcontainers-go v0.41.0` and `testcontainers-go/modules/postgres` — ✅ matches Stage 6 plan.
- `github.com/stretchr/testify v1.11.1` — present (as indirect). Tests mostly use stdlib `testing.T` assertions.
- No `ginkgo`, no `gomock`. Handlers use hand-written test helpers in `testutil` — consistent style.
- `tool github.com/golang-migrate/migrate/v4/cmd/migrate` — module tool directive, newer Go-1.24+ style. Good.

## 0.4 Frontend package

- SvelteKit 2.57, Svelte 5.55, Tailwind 4.2, shadcn-svelte, adapter-node.
- `lucide-svelte` as the only prod dep.
- **No test framework**. `scripts`: `dev`, `build`, `preview`, `check`, `check:watch`, `prepare`. No `vitest`, no `@playwright/test`, no `@testing-library`. **This is a significant gap** — deferred to Stage 6.

## 0.5 Database schema (migration 001)

14 tables:

| Table                 | Notes                                                                                                       |
| --------------------- | ----------------------------------------------------------------------------------------------------------- |
| `users`               | UUID PK, unique username + email, `pending_email`, `role` check, `consent_at NOT NULL`                      |
| `users` sentinel row  | UUID `00000000-...-01` with `is_active=false`, `email=deleted@localhost`. For GDPR hard-delete per ADR-006. |
| `sessions`            | Opaque `token_hash`, `expires_at`, FK cascade                                                               |
| `magic_link_tokens`   | `token_hash`, `purpose` (login/email_change), `used_at` for one-time-use, `expires_at`                      |
| `invites`             | `token`, `max_uses`, `uses_count`, `restricted_email`                                                       |
| `game_types`          | `slug`, `config` JSONB with min/max/default for round/voting/players                                        |
| `game_packs`          | `visibility`, `status`, `deleted_at` (soft delete)                                                          |
| `game_items`          | `position` UNIQUE per pack, `current_version_id`, deferred FK to avoid cycle                                |
| `game_item_versions`  | `version_number` UNIQUE per item, `media_key`, `payload JSONB`, soft-delete                                 |
| `admin_notifications` | `pack_published` / `pack_modified`, read_at nullable                                                        |
| `rooms`               | `code` UNIQUE, `state`, JSONB `config` with CHECK constraints on durations and round_count                  |
| `room_players`        | Composite PK (room_id, user_id), `score`                                                                    |
| `rounds`              | `round_number` UNIQUE per room, timeline CHECK                                                              |
| `submissions`         | UNIQUE (round_id, user_id), `payload JSONB`                                                                 |
| `votes`               | UNIQUE (submission_id, voter_id), `value JSONB` (scoring points)                                            |
| `audit_logs`          | `admin_id`, `action`, `resource`, `changes`                                                                 |

Indexes are thorough — partial indexes on `deleted_at`, `used_at IS NULL`, `read_at IS NULL`.

**Observation for Stage 3**: `rooms.config` has CHECK constraints that match the defaults in `game_types.config`, but the min/max **bounds are duplicated**. Changing the allowed durations requires a schema migration _and_ a seed update. Noted.

## 0.6 HTTP routing surface (from `cmd/server/main.go`)

Middleware stack (applies to every request):

```
chi.Recoverer → RequestID → Logger → Session → Metrics → GlobalLimiter
```

**Observation**: `Session` middleware runs on every request including `/api/health`. That means every liveness probe hits Postgres for a session lookup. **Flagged for Stage 4 (perf / unnecessary DB load)**.

Routes (condensed):

- `GET /api/health`, `GET /api/health/deep`
- `GET /api/metrics` (behind `RequirePrivateIP` — ✅ matches CLAUDE.md promise)
- `/api/auth/*` — `register` (+ invite limiter), `magic-link`, `verify`, `logout`, `me`
- `/api/users/me` — `PATCH`, `/history`, `/export`
- `/api/admin/*` — users, invites, notifications (RequireAdmin)
- `/api/game-types/*` — list, getBySlug
- `/api/packs/*` — CRUD + items + versions (CRUD + reorder + restore + soft-delete + purge)
- `/api/assets/*` — `upload-url` (rate limited), `download-url`
- `/api/rooms/*` — create (rate limited), get, config, leave, kick, leaderboard
- `/api/ws/rooms/{code}` — WebSocket upgrade

Rate limiters: auth (RPM), invite (RPH), rooms (RPH), uploads (RPH), global (RPM). In-memory per-process — documented.

## 0.7 Existing CI (`.github/workflows/`)

### `backend.yml`

- Triggers on `backend/**` push + PR.
- Runs on `ubuntu-latest`, Go 1.25.
- **Starts a `postgres:17-alpine` service container on the runner** and sets `DATABASE_URL` env var… **but `testutil.SetupSuite` always starts its own testcontainer and ignores `DATABASE_URL`**, so the service block is wasted runtime (~5–10 s per run) and wasted memory. **Finding for Stage 1.**
- Steps: `go mod download` → `go build ./...` → `go vet ./...` → `migrate up` → `go test -race -count=1 ./...` → `govulncheck` → `docker build ./backend`.
- **`migrate up` runs against the CI service postgres — not against the testcontainer** — so the migrate step is also essentially unused by the tests. It only verifies "migrations don't explode against an empty DB", which is weak assurance.
- No coverage reporting. No test result artifacts.

### `frontend.yml`

- Triggers on `frontend/**` push + PR.
- `npm ci` → `npm run check` → `npm run build` → `npm audit --audit-level=high` → `docker build ./frontend`.
- **No frontend tests run**. Nothing beyond type-check.

## 0.8 Configuration surface

`config.Load()` is a single monolithic function that:

- Hard-requires: `DATABASE_URL`, `RUSTFS_{ENDPOINT,ACCESS_KEY,SECRET_KEY}`, `FRONTEND_URL`, `BACKEND_URL`, `SMTP_HOST`, `SMTP_FROM`.
- Derives `CookieDomain` from `FrontendURL` via `url.Parse` — silently drops errors (the `if u, err := url.Parse(...); err == nil` discards `err`, making malformed `FRONTEND_URL` fall back to an empty domain, which is invisible). **Flagged for Stage 1.**
- 19 optional duration/int env vars with sensible defaults. Duration parsing uses `time.ParseDuration`. No bounds validation (e.g. `RECONNECT_GRACE_WINDOW=-10s` would be accepted).

## 0.9 Critical seams already in place

Good:

- `storage.Storage` interface + `S3Storage` impl → **testable via fake** ✅
- `auth.EmailSender` interface + `email.Service` impl (compile-time check: `var _ auth.EmailSender = (*Service)(nil)`) ✅
- `db.Queries` (sqlc) takes a `DBTX` interface → can be backed by a `pgxpool.Pool` or a `pgx.Tx` — this is what `testutil.WithTx` exploits ✅
- Game type registry is already a `Register`/`Get` pattern ✅

Missing:

- **No `Clock` seam.** `time.Now()` is called directly in `hub.go` (e.g. `time.Now().Add(roundDuration)`), in `auth/tokens.go`, and presumably in session expiry. Time-sensitive tests (grace window expiry, session renew, magic-link TTL) will need sleeps or polling. **Flagged for Stage 2.**
- **No RNG seam.** `generateRoomCode` in `rooms.go` uses `crypto/rand` directly. Tests cannot assert on a deterministic code. Not fatal, but awkward.
- **`email.Service.send` constructs `gomail.NewClient` every call.** Testable only by substituting the whole `auth.EmailSender` interface — which is already exported, so doable. ✅

## 0.10 CRITICAL finding from Stage 0: hubs are never created in production

The **biggest** finding of Stage 0, and one I did not expect to surface in inventory:

**`game.Manager.GetOrCreate` is only called from `backend/internal/game/hub_test.go:103`.** It is not called from any `_ = cmd/server`, `internal/api/rooms.go`, `internal/api/ws.go`, or `internal/api/room_actions.go`.

The production WebSocket path is:

1. Client calls `POST /api/rooms` → `rooms.go:Create` → writes a row in `rooms` table. **No hub is created.**
2. Client opens `GET /api/ws/rooms/{code}` → `ws.go:ServeHTTP` calls `h.manager.Get(roomCode)` (line 36) → returns `(nil, false)` because no hub exists → responds `404 room_not_found` and never upgrades.

**End-to-end impact**: WebSocket connections cannot succeed in production. Every player attempting to join any room will receive `404`. The entire game-play path is broken.

The test at `hub_test.go:103` works because it calls `GetOrCreate` directly — i.e. **the test exercises a path the production code does not take**. This is a textbook example of why "tests pass but feature is broken": the unit of behavior being tested is not what users exercise.

### Why I'm confident

- `Grep` on `GetOrCreate|NewHub` across the whole codebase returns only the 2 call sites noted.
- `ws.go` deliberately uses `manager.Get` (not `GetOrCreate`) and has a code path that returns `room_not_found` when absent — this is not a typo, it's a design choice that expects the hub to exist _before_ WS connect.
- Neither `rooms.Create` nor any startup cleanup creates hubs for existing lobby rooms.

### Secondary concern: context lifetime

Even if a missing `GetOrCreate` call were added to `ws.go`, the current signature is:

```go
func (m *Manager) GetOrCreate(ctx context.Context, ...) *Hub {
    ...
    go func() { h.Run(ctx); ... }()
    return h
}
```

If the caller passes `r.Context()` from the HTTP request, the hub's lifetime is tied to **the first WS connection's HTTP upgrade request**, which is completed once the connection is upgraded and handed off. The hub would die immediately. The fix is to use a server-scoped context — typically one created in `main()` and cancelled on shutdown signal. That context needs to be threaded through `Manager` or stored on it.

**Both issues go in Stage 3 (correctness) as must-fix.**

## 0.11 Miscellaneous observations harvested during inventory

Captured here for the later stages so we don't re-discover them:

- **`hub.handleRegister`** (`hub.go:232`) does not enforce a max-players limit even though `game_types.config.max_players` exists in the schema. → Stage 3.
- **`hub.runRounds`** is launched as a goroutine but has **no way to be cancelled outside the room's context** — if `finishRoom` is called (e.g. host disconnect), `runRounds` keeps sleeping in `time.After(roundDuration)` and will send further `roundCtrl` messages after the room is finished. → Stage 4.
- **`hub.KickPlayer`** sends on `h.incoming` (buffered 64) from outside the Run goroutine with no `select`/`default` — an HTTP handler could block indefinitely if the queue is saturated. → Stage 4.
- **`time.AfterFunc` grace callback** (`hub.go:254`) captures `h.graceExpired` in a closure — if the hub has already exited `Run()` (ctx cancel), the AfterFunc goroutine will block forever trying to send on a channel no-one is reading. → Stage 4 (goroutine leak on shutdown).
- **Votes hardcode `{"points":1}`** in `hub.go:645`. The `CalculateRoundScores` handler is passed the votes separately and is free to use its own scheme, but the DB stores `1`, not the handler's value. → Stage 3.
- **`json.Unmarshal` errors swallowed** at `hub.go:308` (`system:kick` payload) and `hub.go:440` (room config parse). → Stage 4.
- **WebSocket send channel is 64-deep**, `safeSend` drops on overflow and logs. Good design, but on a slow laptop with 10 players every message could drop during `vote_results`. → Stage 4 (observability).
- **CORS / origin** in `ws.go:44`: `r.Header.Get("Origin") == h.allowedOrigin` — exact string match. Any trailing slash mismatch → silent failure. → Stage 5.

## 0.12 Docs inventory

Present under `docs/`:

```
docs/README.md
docs/overview.md
docs/architecture.md
docs/auth-and-identity.md
docs/game-engine.md
docs/api.md
docs/frontend.md
docs/self-hosting.md
docs/operations.md
docs/reference/decisions.md        (ADR-001…010)
docs/reference/error-codes.md
docs/reference/gdpr.md
docs/reference/privacy-policy.md
```

All are in git (not among the deleted files). Consistency with code was **not** cross-checked during Stage 0 — that is a task for Stage 3 (correctness) for the files that document contracts (`api.md`, `game-engine.md`, `error-codes.md`).

---

## Stage 0 summary

- Repo structure largely matches CLAUDE.md, with three small drifts (compose paths).
- Test infrastructure (`testutil` + testcontainers-go) is already excellent — Stage 6 has less work than expected.
- **1 must-fix architectural bug found**: no production path creates `Hub` instances → WS flow is end-to-end broken. This alone justifies the audit.
- **Several goroutine / concurrency hazards** queued for Stages 3 & 4.
- **No frontend tests at all** — largest single test-suite gap.
- **CI backend workflow starts a Postgres service that tests never use** — wasted time + weak migration check.
- **Session middleware runs on `/health`** — unnecessary DB load.
