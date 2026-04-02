# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project

**FabDoYouMeme** — GPLv3, self-hosted, invite-only multi-game platform (meme caption, match, vote). Runs on a single machine via Docker Compose. Reverse proxy is pre-existing and assumed to route `/api/*` to backend, `/*` to frontend.

GitHub: `github.com/MorganKryze/FabDoYouMeme`
Status: **pre-implementation** — no source code yet. `design/` is the authoritative architecture reference.

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

```
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
├── design/                      # architecture reference (see below)
├── docker-compose.yml
├── docker-compose.override.yml  # dev overrides (Mailpit, volume mounts)
└── CLAUDE.md
```

---

## Design Docs

| File                           | Contents                                                                |
| ------------------------------ | ----------------------------------------------------------------------- |
| `design/00-index.md`           | Index + quick-reference flows                                           |
| `design/01-overview.md`        | Goals, tech stack rationale, repo structure                             |
| `design/02-identity.md`        | Auth flow, invite system, session management, rate limits               |
| `design/03-data.md`            | PostgreSQL schema, indexes, RustFS storage, asset lifecycle             |
| `design/04-protocol.md`        | REST endpoints, WebSocket protocol, game type handler interface         |
| `design/05-frontend.md`        | SvelteKit routing, state architecture, UX flows, accessibility          |
| `design/06-operations.md`      | Docker Compose, migrations, backups, CI, logging, Prometheus metrics    |
| `design/ref-error-codes.md`    | Canonical `snake_case` error code table (REST + WebSocket)              |
| `design/ref-env-vars.md`       | All environment variables: name, default, required, description         |
| `design/ref-decisions.md`      | ADR-style decisions: why no JWT, why chi, why sentinel UUID, etc.       |
| `design/ref-gdpr.md`           | GDPR compliance: lawful basis, ROPA-lite, rights, DPA, breach procedure |
| `design/ref-privacy-policy.md` | Art. 13(1) Privacy Policy stub template for operator to complete        |

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
# Start all services (includes Mailpit via docker-compose.override.yml)
docker compose up --build

# Watch backend logs
docker compose logs -f backend

# Apply all pending DB migrations
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1

# Regenerate sqlc types after modifying any query in backend/db/queries/
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
- **Rate limits are in-memory**: per-process only; if multi-instance is ever added, externalize to Redis (see ADR-005 in `ref-decisions.md`)
- **`/api/metrics` must be IP-restricted**: never expose Prometheus endpoint to the internet
- **Startup cleanup**: on every start, the backend marks `playing` rooms as `finished` (crash recovery) and closes stale `lobby` rooms older than 24h — both are idempotent
- **GDPR**: registration requires `consent: true` + `age_affirmation: true`; `users.consent_at` is set once and never changed; hard-delete replaces both `submissions.user_id` AND `votes.voter_id` with the sentinel UUID before deleting the user row; game data purged after 2 years; audit log PII anonymized after 3 years — see `ref-gdpr.md`

---

## Core Principles

- **Simplicity First**: single-machine Docker Compose; no distributed systems complexity
- **Least Attack Surface**: no passwords, no public asset access, secrets via env only
- **Multi-game Extensibility**: game types are registered handler units; adding one requires no schema or protocol changes
- **Minimal Impact**: changes should touch only what's necessary
