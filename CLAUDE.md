# CLAUDE.md

Guidance for Claude Code in this repository.

## Project

**FabDoYouMeme** — GPLv3, self-hosted, invite-only multi-game platform (meme caption, match, vote). Single-machine Docker Compose. Reverse proxy routes `/api/*` → backend, `/*` → frontend.

GitHub: `github.com/MorganKryze/FabDoYouMeme`
Status: **implemented** — source complete. `docs/` is the authoritative reference.

---

## Engineering Standards

Work like a senior engineer. These are not suggestions.

- **Root-cause, not symptoms.** Fix the actual bug, not its downstream effects. If a test fails, fix the code — don't weaken the test. If a type is wrong, correct it — don't cast around it.
- **No temporary fixes.** No `TODO`, `FIXME`, `// hack`, no "we'll fix this later" commits. If the real fix is out of scope, say so explicitly and stop — don't smuggle a patch in.
- **Elegance at every size.** A three-line function deserves the same thought as a three-hundred-line one. No throwaway code in shared paths.
- **No tech debt accrual.** Every change holds the line or pays debt down. No dead code, no duplicated logic, no "I'll abstract this next time" copies.
- **Verify, don't assume.** After a change: build, vet, test. For UI, open the browser and exercise the feature. Never claim "should work" — it passes or it doesn't.
- **Minimal blast radius.** Touch only what the task requires. No drive-by refactors, no unrelated formatting, no speculative abstractions.
- **Read before writing.** Match existing patterns in this repo (state classes, API wrappers, handler registration, sqlc usage). Consistency beats cleverness.
- **Ask when uncertain.** If the right answer depends on a product or UX decision, ask — don't guess and hope.

**Project invariants:** single-machine simplicity (no distributed systems), least attack surface (no passwords, no public asset access, secrets via env), game types as pluggable handlers (no schema or protocol changes to add one).

---

## Tech Stack

| Layer        | Technology                                       |
| ------------ | ------------------------------------------------ |
| Frontend     | SvelteKit (`adapter-node`) + Svelte 5            |
| Styling      | Tailwind v4 + shadcn-svelte                      |
| Backend      | Go + `chi`, `sqlc`, `gorilla/websocket`          |
| Database     | PostgreSQL 17 (migrated via `golang-migrate`)    |
| File storage | RustFS (S3-compatible, external, `pangolin` net) |
| Email        | `go-mail` (wneessen) via SMTP                    |
| Sessions     | Opaque DB-backed tokens (not JWT)                |
| Runtime      | Docker Compose                                   |

Layout: `backend/` (`cmd/server`, `internal/{auth,game,storage,email,middleware,config}`, `db/{migrations,queries}`), `frontend/src/` (`lib/{components,state,api,games}`, `routes/`), `docs/`, `docker/`, `Makefile`.

---

## Docs (in `docs/`)

- Core: `overview.md`, `architecture.md`, `auth-and-identity.md`, `game-engine.md`, `api.md`, `frontend.md`
- Ops: `self-hosting.md`, `operations.md`, `brand.md`
- Reference: `reference/error-codes.md`, `reference/decisions.md` (ADR-001–010), `reference/gdpr.md`, `reference/privacy-policy.md`

---

## Quick Reference

- **Auth**: register (invite + consent + age) → magic-link → `/auth/verify` → session cookie
- **Game**: create room → WS `join` → host `start` → `round_started` → `{slug}:submit` → `submissions_closed` → `{slug}:vote` → `vote_results` → `game_ended`
- **Assets**: create item → `POST /api/assets/upload-url` (MIME + magic-byte validated) → PUT to RustFS → `PATCH item { media_key }`
- **First boot**: `SEED_ADMIN_EMAIL` → backend creates admin + sends magic link (idempotent)
- **Reconnect**: `RECONNECT_GRACE_WINDOW` (default 30s) before removal

---

## Commands

```bash
# Dev stack — isolated per stage (project name fabyoumeme-{dev,preprod,prod})
make dev            # up --build -d
make dev-down       # keep DB volume
make dev-clean      # wipe DB volume (destructive)
# same targets exist for preprod, prod

# Env drift — never overwrites existing values
make env-check      # summary, exit 1 on drift
make env-diff       # per-variable diff
make env-migrate    # interactive append + bootstrap

# Logs
docker compose -p fabyoumeme-dev -f docker/compose.base.yml -f docker/compose.dev.yml logs -f backend

# DB
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1
cd backend && sqlc generate   # after any change in db/queries/

# Backend
cd backend && go build ./... && go vet ./... && go test -race -count=1 ./...

# Frontend
cd frontend && npm ci && npm run check && npm run build

# Dev mail (Mailpit)
open http://localhost:8025
```

---

## Non-Obvious Patterns

- **Sessions, not JWT**: logout = `DELETE session`; instantly revocable, negligible overhead at this scale.
- **Magic links**: DB stores only a SHA-256 hash; token is one-time-use + short-lived.
- **`sqlc`**: never hand-write Go DB code — author SQL in `backend/db/queries/`, regenerate, use the generated types.
- **Game handlers**: new game = implement `GameTypeHandler` + `Register()` in `main.go`. No schema or protocol change.
- **RustFS is external**: runs on `pangolin` docker network; backend talks to it via `RUSTFS_ENDPOINT`.
- **Rate limits are in-memory**: per-process only; externalize to Redis if multi-instance (ADR-005).
- **`/api/metrics` must be IP-restricted**: never expose Prometheus to the internet.
- **Startup cleanup**: backend marks stale `playing` rooms `finished` (crash recovery) + closes `lobby` rooms >24h old. Both idempotent.
- **GDPR**: registration requires `consent: true` + `age_affirmation: true`; `users.consent_at` set once, never changed. Hard-delete replaces `submissions.user_id` AND `votes.voter_id` with the sentinel UUID before deleting the user. Game data purged at 2y, audit PII anonymized at 3y. See `docs/reference/gdpr.md`.
- **Privacy page is env-driven**: `frontend/src/routes/(public)/privacy/+page.svelte` reads `PUBLIC_OPERATOR_*` via `$env/dynamic/public`. Never hardcode operator identity. Missing required var → visible red warning banner. See `docs/self-hosting.md → Legal / privacy policy`.

---

## Plan Deviation Policy (`docs/superpowers/plans/`)

When a plan snippet is wrong (bad signature, invalid syntax, stdlib preferred over custom helper, …):

1. Edit the plan to match what was implemented.
2. If the reason is non-obvious, add `> **Deviation (implemented):** …` immediately below the snippet.
3. Plan must never contradict the code.

---

## Git Workflow

Read-only. No commit, push, branch, or destructive git from Claude. `git log` / `diff` / `status` are fine.

---

## graphify

Knowledge graph at `graphify-out/`.

- Before architecture / codebase questions, read `graphify-out/GRAPH_REPORT.md` for god nodes + communities.
- If `graphify-out/wiki/index.md` exists, navigate it instead of raw files.
- After modifying code in this session: `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"`.
