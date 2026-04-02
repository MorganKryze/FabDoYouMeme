# FabDoYouMeme — Design Index

> **Status**: pre-implementation. These documents are the authoritative architecture reference.
> Update the relevant file whenever a significant decision changes.
>
> **Redesigned 2026-04**: previously 10 files (00–10). Reorganized into 6 thematic docs + 3 reference files to eliminate split ownership and cross-doc contradictions.

---

## Documents

| File                                     | Contents                                                                                      |
| ---------------------------------------- | --------------------------------------------------------------------------------------------- |
| [01-overview.md](01-overview.md)         | Project goals, tech stack rationale, repository structure                                     |
| [02-identity.md](02-identity.md)         | Auth flow, invite system, session management, email templates, rate limits, security policies |
| [03-data.md](03-data.md)                 | PostgreSQL schema, indexes, constraints, cleanup strategy, RustFS storage, asset lifecycle    |
| [04-protocol.md](04-protocol.md)         | REST endpoints, WebSocket protocol, game type handler interface, game engine extensibility    |
| [05-frontend.md](05-frontend.md)         | SvelteKit routing, state architecture, UX flows, toast notifications, accessibility           |
| [06-operations.md](06-operations.md)     | Docker Compose, migrations, backups, CI, structured logging, Prometheus metrics, alerting     |
| [ref-error-codes.md](ref-error-codes.md) | Canonical `snake_case` error code table (REST + WebSocket)                                    |
| [ref-env-vars.md](ref-env-vars.md)       | All environment variables: name, default, required, description                               |
| [ref-decisions.md](ref-decisions.md)     | ADR-style decisions: why no JWT, why chi, why sentinel UUID, etc.                             |

---

## Quick Reference

**Auth flow**: `POST /api/auth/register` → `POST /api/auth/magic-link` → `GET /auth/verify?token=` (frontend intermediate page) → `POST /api/auth/verify` (backend) → session cookie set

**Game flow**: create room → WebSocket `join` → host sends `start` → `round_started` (includes `ends_at` + `duration_seconds`) → `{slug}:submit` → `submissions_closed` → `{slug}:vote` → `vote_results` → repeat → `game_ended`

**Asset flow**: create item record → `POST /api/assets/upload-url` (MIME + magic byte validation) → PUT directly to RustFS → `PATCH item { media_key }` to confirm

**First boot**: set `SEED_ADMIN_EMAIL` env var → backend auto-creates admin + sends magic link on startup (idempotent)

**Error codes**: all `snake_case` codes → `ref-error-codes.md`

**Environment variables**: all vars with defaults → `ref-env-vars.md`

**Architecture decisions**: why each non-obvious choice was made → `ref-decisions.md`
