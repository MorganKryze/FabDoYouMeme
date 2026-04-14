# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project

**FabDoYouMeme** — GPLv3, self-hosted, invite-only multi-game platform (meme caption, match, vote). Runs on a single machine via Docker Compose. Reverse proxy is pre-existing and assumed to route `/api/*` to backend, `/*` to frontend.

GitHub: `github.com/MorganKryze/FabDoYouMeme`
Status: **implemented** — source code is complete. `docs/` is the authoritative documentation reference.

---

## Tech Stack

| Layer          | Technology                            |
| -------------- | ------------------------------------- |
| Frontend       | SvelteKit (`adapter-node`) + Svelte 5 |
| Styling        | Tailwind CSS v4 + shadcn-svelte       |
| Backend API    | Go + `chi` router                     |
| Database       | PostgreSQL 17                         |
| File storage   | RustFS (S3-compatible, self-hosted)   |
| DB migrations  | `golang-migrate`                      |
| Query layer    | `sqlc` (type-safe Go from raw SQL)    |
| WebSockets     | `gorilla/websocket`                   |
| Email          | `go-mail` (wneessen) via SMTP         |
| Session tokens | Opaque tokens, DB-backed (not JWT)    |
| Container      | Docker Compose                        |

---

## Repository Structure

```plain
FabDoYouMeme/
├── backend/
│   ├── cmd/server/              # main.go — wires everything, registers game handlers
│   ├── internal/
│   │   ├── auth/                # session management, magic link, invite logic
│   │   ├── game/                # game type registry, hub, room/round lifecycle
│   │   │   ├── registry.go      # Register() + Dispatch()
│   │   │   ├── hub.go           # WebSocket hub (per-room goroutine)
│   │   │   └── types/meme_caption/  # implements GameTypeHandler
│   │   ├── storage/             # RustFS/S3 client wrapper (interface-backed)
│   │   ├── email/               # template rendering + SMTP sending
│   │   ├── middleware/          # auth, rate-limit, structured logging, request ID
│   │   └── config/              # env-based config loading
│   ├── db/
│   │   ├── migrations/          # golang-migrate .sql up/down pairs
│   │   └── queries/             # sqlc .sql files → generated Go in db/sqlc/
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/      # shared UI (Button, Avatar, Timer, etc.)
│   │   │   ├── state/           # Svelte 5 reactive state classes (ws, room, user)
│   │   │   ├── api/             # typed fetch wrappers per REST endpoint
│   │   │   └── games/meme-caption/  # SubmitForm, VoteForm, ResultsView, GameRules
│   │   └── routes/              # SvelteKit file-based routing
│   ├── Dockerfile
│   └── package.json
├── docs/                        # documentation (see below)
├── docker/
│   ├── compose.base.yml         # shared services
│   ├── compose.dev.yml          # dev overrides (Mailpit, volume mounts)
│   ├── compose.preprod.yml
│   └── compose.prod.yml
├── Makefile                     # dev/preprod/prod up+down targets
└── CLAUDE.md
```

---

## Docs

| File                               | Contents                                                                |
| ---------------------------------- | ----------------------------------------------------------------------- |
| `docs/overview.md`                 | Goals, tech stack rationale, key design decisions                       |
| `docs/architecture.md`             | System components, DB schema, storage, middleware, startup behaviour    |
| `docs/auth-and-identity.md`        | Auth flow, invite system, session management, security controls         |
| `docs/game-engine.md`              | Room/round lifecycle, WebSocket hub, game type handler interface        |
| `docs/api.md`                      | REST endpoints, WebSocket protocol, error model                         |
| `docs/frontend.md`                 | SvelteKit routing, Svelte 5 state singletons, game plugin architecture  |
| `docs/self-hosting.md`             | Prerequisites, first boot, all environment variables                    |
| `docs/operations.md`               | Monitoring, logs, backups, CI, production checklist                     |
| `docs/brand.md`                    | Brand & identity: name rationale, voice, vocabulary, namespace audit    |
| `docs/reference/error-codes.md`    | Canonical `snake_case` error code table (REST + WebSocket)              |
| `docs/reference/decisions.md`      | ADR-001–ADR-010: why no JWT, why chi, why sentinel UUID, etc.           |
| `docs/reference/gdpr.md`           | GDPR compliance: lawful basis, ROPA-lite, rights, DPA, breach procedure |
| `docs/reference/privacy-policy.md` | Art. 13(1) Privacy Policy stub template for operator to complete        |

---

## Quick Reference

**Auth flow**: `POST /api/auth/register { invite_token, username, email, consent: true, age_affirmation: true }` → `POST /api/auth/magic-link` → `GET /auth/verify?token=` (frontend page) → `POST /api/auth/verify` (backend) → session cookie set

**Game flow**: create room → WS `join` → host `start` → `round_started` (includes `ends_at` + `duration_seconds`) → `{slug}:submit` → `submissions_closed` → `{slug}:vote` → `vote_results` → repeat → `game_ended`

**Asset flow**: create item record → `POST /api/assets/upload-url` (MIME + magic byte validation) → PUT directly to RustFS → `PATCH item { media_key }` to confirm

**First boot**: set `SEED_ADMIN_EMAIL` → backend auto-creates admin + sends magic link on startup (idempotent)

**Reconnect grace**: disconnected players have `RECONNECT_GRACE_WINDOW` (default 30s) to rejoin before being removed from the room

---

## Commands

```bash
# Start the dev stack (includes Mailpit via docker/compose.dev.yml).
# Each stage runs under its own Compose project name (fabyoumeme-{dev,preprod,prod})
# so Postgres volumes, containers, and networks are fully isolated per stage.
make dev           # or: docker compose -p fabyoumeme-dev -f docker/compose.base.yml -f docker/compose.dev.yml --env-file .env.dev up --build -d
make dev-down      # tear it down (keeps the database volume)
make dev-clean     # tear it down AND wipe postgres_data — destructive
# Analogous targets exist for preprod and prod: {preprod,prod}-{,down,clean}

# Env var drift detection & migration — run after pulling upstream updates.
# Compares .env.{dev,preprod,prod} against their .env.*.example templates.
# Never overwrites existing values; only appends missing defaults.
make env-check     # brief "am I out of sync?" summary; exits 1 on drift
make env-diff      # detailed per-variable diff with reasons
make env-migrate   # interactively append missing defaults + bootstrap missing live files

# Watch backend logs (remember the -p flag when running compose directly)
docker compose -p fabyoumeme-dev -f docker/compose.base.yml -f docker/compose.dev.yml logs -f backend

# Apply all pending DB migrations
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1

# Regenerate sqlc types after modifying any query in backend/db/queries/
# (config lives at backend/sqlc.yaml)
cd backend && sqlc generate

# Backend: build, vet, test
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test -race -count=1 ./...

# Frontend: install, type-check, build
cd frontend && npm ci
cd frontend && npm run check
cd frontend && npm run build

# View captured dev emails (Mailpit)
open http://localhost:8025
```

---

## Non-Obvious Patterns

- **Sessions over JWT**: logout = `DELETE session` row; instantly revocable, no signing key, negligible overhead at this scale
- **Magic links**: only a SHA-256 hash is stored — nothing crackable if DB leaks; token is one-time-use + short-lived
- **`sqlc` workflow**: never write Go DB code by hand; write SQL in `backend/db/queries/`, run `sqlc generate`, use the generated types
- **Game handler registry**: adding a new game type requires only implementing `GameTypeHandler` and calling `Register()` in `main.go` — no schema or protocol changes
- **RustFS is external**: it lives in a separate Docker stack on `pangolin` network; the backend communicates via `RUSTFS_ENDPOINT`
- **Rate limits are in-memory**: per-process only; if multi-instance is ever added, externalize to Redis (see ADR-005 in `docs/reference/decisions.md`)
- **`/api/metrics` must be IP-restricted**: never expose Prometheus endpoint to the internet
- **Startup cleanup**: on every start, the backend marks `playing` rooms as `finished` (crash recovery) and closes stale `lobby` rooms older than 24h — both are idempotent
- **GDPR**: registration requires `consent: true` + `age_affirmation: true`; `users.consent_at` is set once and never changed; hard-delete replaces both `submissions.user_id` AND `votes.voter_id` with the sentinel UUID before deleting the user row; game data purged after 2 years; audit log PII anonymized after 3 years — see `docs/reference/gdpr.md`
- **Privacy policy is env-driven**: the `/privacy` page at `frontend/src/routes/(public)/privacy/+page.svelte` is a generic template — operator-specific fields (controller name, contact email, deployment URL, SMTP provider) are injected at runtime from `PUBLIC_OPERATOR_*` env vars via `$env/dynamic/public`. Never hardcode a real operator name or email into the template; to change the displayed values, update the env var, not the file. An unset required var triggers a visible red warning banner on the page. Full variable reference: `docs/self-hosting.md → Legal / privacy policy`.

---

## Plan Deviation Policy

When implementation requires a justified deviation from a plan (wrong type signature, invalid syntax in the plan, stdlib preferred over custom helper, etc.):

1. **Update the plan file** — edit the relevant code snippet to match what was actually implemented.
2. **Add an inline note** — if the reason is non-obvious, add a `> **Deviation (implemented):** ...` blockquote immediately after the snippet explaining what changed and why.
3. **Never leave the plan inconsistent with the code** — a future reader (or agent) doing a consistency check must be able to trust that plan = code.

This applies to all files under `docs/superpowers/plans/`.

---

## Git Workflow

- **No git modifications allowed via Claude:** Do not commit, push, create branches, or run any destructive git commands. Read-only git access only (e.g. `git log`, `git diff`, `git status`).

---

## Core Principles

- **Simplicity First**: single-machine Docker Compose; no distributed systems complexity
- **Least Attack Surface**: no passwords, no public asset access, secrets via env only
- **Multi-game Extensibility**: game types are registered handler units; adding one requires no schema or protocol changes
- **Minimal Impact**: changes should touch only what's necessary

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:

- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- After modifying code files in this session, run `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"` to keep the graph current
