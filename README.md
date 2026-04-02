# FabDoYouMeme

Self-hosted, invite-only party game platform. Launch meme caption rounds, vote on the funniest answers, and track scores — all running on your own hardware via Docker Compose.

> **Status**: pre-implementation. The `design/` directory is the authoritative architecture reference.

---

## What it is

- **Invite-only**: no public registration; admins create invite tokens and control who joins
- **Multi-game**: starts with meme-caption; new game types plug in without schema or protocol changes
- **Self-hosted**: single Docker Compose stack on personal hardware; no third-party game servers
- **No passwords**: authentication via magic links only — one email click to log in

---

## Prerequisites

Before starting:

| Dependency              | Notes                                                                          |
| ----------------------- | ------------------------------------------------------------------------------ |
| Docker + Docker Compose | Runs all services                                                              |
| Reverse proxy           | Pre-existing; must route `/api/*` → backend and `/*` → frontend                |
| SMTP server             | For magic link delivery (Mailgun, AWS SES, self-hosted Postfix, etc.)          |
| RustFS                  | Deployed separately on the `pangolin` Docker network — see `design/03-data.md` |

---

## First boot

**1. Copy and fill the environment file**

```bash
cp .env.example .env
```

Required variables (see `design/ref-env-vars.md` for the full list):

```bash
POSTGRES_PASSWORD=change_me
RUSTFS_ENDPOINT=https://rustfs.example.com
RUSTFS_ACCESS_KEY=...
RUSTFS_SECRET_KEY=...
FRONTEND_URL=https://meme.example.com
BACKEND_URL=https://meme.example.com/api
SMTP_HOST=smtp.example.com
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_FROM=noreply@example.com
SEED_ADMIN_EMAIL=you@example.com   # triggers admin creation on first start
```

**2. Apply database migrations**

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up
```

**3. Start all services**

```bash
docker compose up --build -d
```

**4. Check your email**

The backend sends a magic link to `SEED_ADMIN_EMAIL` on first boot. Click it to log in as admin, then create invite tokens for your players.

---

## Development

```bash
# Start with Mailpit for local email catching
docker compose up --build

# View captured emails
open http://localhost:8025

# Watch backend logs
docker compose logs -f backend

# Backend checks
cd backend && go build ./... && go vet ./... && go test -race -count=1 ./...

# Frontend checks
cd frontend && npm ci && npm run check && npm run build
```

---

## Architecture

Full design documentation lives in `design/`:

| Doc                       | Contents                                  |
| ------------------------- | ----------------------------------------- |
| `design/00-index.md`      | Index + quick-reference flows             |
| `design/02-identity.md`   | Auth, invite system, session management   |
| `design/03-data.md`       | PostgreSQL schema, RustFS storage         |
| `design/04-protocol.md`   | REST API, WebSocket protocol, game engine |
| `design/05-frontend.md`   | SvelteKit routing, state, UX flows        |
| `design/06-operations.md` | Docker Compose, migrations, CI, backups   |
| `design/ref-gdpr.md`      | GDPR compliance reference                 |

---

## License

[GPLv3](LICENSE)
