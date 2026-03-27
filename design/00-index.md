# FabDoYouMeme — Design Index

> **Status**: pre-implementation. These documents are the authoritative architecture reference.
> Update the relevant file whenever a significant decision changes.

---

## Documents

| File | Contents |
| ---- | -------- |
| [01-overview.md](01-overview.md) | Project goals, tech stack rationale, repository structure |
| [02-auth.md](02-auth.md) | Magic link auth, invite system, session management, bootstrap |
| [03-database.md](03-database.md) | Full PostgreSQL schema, indexes, constraints, cleanup strategy |
| [04-api.md](04-api.md) | REST endpoints, WebSocket protocol, error conventions |
| [05-storage.md](05-storage.md) | Rustfs file storage, upload/download flows, access model |
| [06-game-engine.md](06-game-engine.md) | Game type handler interface, extensibility, payload versioning |
| [07-frontend.md](07-frontend.md) | SvelteKit route structure, pages, layouts, UX flows |
| [08-devops.md](08-devops.md) | Docker Compose, environment variables, migrations, backup |
| [09-security.md](09-security.md) | Security checklist and policies |
| [10-observability.md](10-observability.md) | Structured logging, health checks, Prometheus metrics |

---

## Quick Reference

**Auth flow**: `POST /api/auth/register` → `POST /api/auth/magic-link` → `GET /auth/verify?token=` (frontend) → `POST /api/auth/verify` (backend) → session cookie set

**Game flow**: create room → WebSocket `join` → host sends `start` → `round_started` events → `{slug}:submit` → `submissions_closed` → `{slug}:vote` → `vote_results` → repeat → `game_ended`

**Asset flow**: create item record → `POST /api/assets/upload-url` → PUT directly to Rustfs → `PATCH item { media_key }` to confirm

**First boot**: set `SEED_ADMIN_EMAIL` env var → backend auto-creates admin + sends magic link on startup
