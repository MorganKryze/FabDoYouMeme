# FabDoYouMeme

Self-hosted, invite-only party game platform. Launch meme caption rounds, vote on the funniest answers, and track scores — all running on your own hardware via Docker Compose.

---

## What it is

- **Invite-only**: no public registration; admins create invite tokens and control who joins
- **Multi-game**: starts with meme-caption; new game types plug in without schema or protocol changes
- **Self-hosted**: single Docker Compose stack on personal hardware; no third-party game servers
- **No passwords**: authentication via magic links only — one email click to log in

---

## Prerequisites

| Dependency              | Notes                                                                                          |
| ----------------------- | ---------------------------------------------------------------------------------------------- |
| Docker + Docker Compose | Runs all services                                                                              |
| Reverse proxy           | Pre-existing; must route `/api/*` → backend and `/*` → frontend                                |
| SMTP server             | For magic link delivery (Mailgun, AWS SES, self-hosted Postfix, etc.)                          |
| RustFS                  | S3-compatible object store deployed on the `pangolin` Docker network — see `docs/architecture.md` |

---

## Local development

This section covers running the full stack locally for testing and development.

### 1. Copy and configure the environment file

```bash
cp .env.dev.example .env.dev
```

Edit `.env.dev` with values for your setup. At minimum set:

```bash
POSTGRES_PASSWORD=dev_password
FRONTEND_URL=http://localhost:3000
BACKEND_URL=http://localhost:8080
SEED_ADMIN_EMAIL=you@example.com
```

SMTP is automatically replaced by Mailpit in dev mode — you don't need real SMTP credentials locally.

### 2. Set up object storage

The backend exits on startup if it can't reach `RUSTFS_ENDPOINT`. You have two options:

**Option A — Use your existing RustFS instance**

Fill in your credentials in `.env.dev`:

```bash
RUSTFS_ENDPOINT=https://rustfs.example.com
RUSTFS_ACCESS_KEY=your_access_key
RUSTFS_SECRET_KEY=your_secret_key
RUSTFS_BUCKET=fabyoumeme-assets
```

Make sure the bucket exists in your RustFS instance before starting.

**Option B — Run a local MinIO substitute**

MinIO is S3-compatible and works as a drop-in replacement for local testing. Add it to your dev stack:

```bash
# Start MinIO on the project_network so the backend can reach it
docker run -d --name minio \
  --network project_network \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  -p 9001:9001 \
  quay.io/minio/minio server /data --console-address ":9001"
```

Then create the bucket (open `http://localhost:9001`, log in with `minioadmin/minioadmin`, create bucket `fabyoumeme-assets`) or via CLI:

```bash
docker exec minio sh -c "
  mc alias set local http://localhost:9000 minioadmin minioadmin &&
  mc mb local/fabyoumeme-assets
"
```

Set these in `.env.dev`:

```bash
RUSTFS_ENDPOINT=http://minio:9000
RUSTFS_ACCESS_KEY=minioadmin
RUSTFS_SECRET_KEY=minioadmin
RUSTFS_BUCKET=fabyoumeme-assets
```

### 3. Start the full stack

```bash
make dev
```

This starts: PostgreSQL, backend (Go), frontend (SvelteKit), Mailpit (local email).

Services:

| Service   | URL                         |
| --------- | --------------------------- |
| Frontend  | `http://localhost:3000`     |
| Backend   | `http://localhost:8080`     |
| Mailpit   | `http://localhost:8025`     |

### 4. First boot

On first start the backend sends a magic link to `SEED_ADMIN_EMAIL`:

1. Open `http://localhost:8025` (Mailpit)
2. Click the magic link in the email
3. You are now logged in as admin

### 5. Create an invite and register a player

1. Navigate to `http://localhost:3000/admin` → **Invites** → **Create Invite**
2. Copy the invite token
3. In a new browser (or incognito), go to `http://localhost:3000/auth/register?invite=<token>`
4. Fill in username, email, check both consent boxes, submit
5. Check Mailpit for the magic link → click it → logged in as a regular player

### 6. Test the game flow

With two browser sessions (admin + player):

1. **Admin/host**: go to `http://localhost:3000` → Create Room (select game type + pack) → redirected to `/rooms/WXYZ`
2. **Player**: enter the room code on the join card → joins room
3. Both players visible in the sidebar panel
4. **Host** clicks **Start Game** → countdown overlay appears (3…2…1…GO!)
5. Both players write captions, submit → status pills turn green ✓
6. Voting phase: select a caption and vote
7. Results: see vote counts, leaderboard, **Next Round →**
8. After all rounds: Game Over screen with final leaderboard

### 7. Test the profile page

1. Navigate to `http://localhost:3000/profile`
2. Edit username → Save → green toast "Username updated."
3. Click **Download My Data** → JSON file downloads

---

## Pre-production deployment

Use this when you want to deploy a locally-built image to the server (e.g. testing a branch before merging). Requires the `pangolin` Docker network to exist on the host.

**1. Copy and fill the environment file**

```bash
cp .env.preprod.example .env.preprod
```

Fill in real values — see `docs/self-hosting.md` for the full list. Real SMTP credentials are required (no Mailpit in preprod).

**2. Start all services**

```bash
make preprod
```

Services are not port-forwarded; they are exposed via the pangolin reverse proxy.

**3. Check your email**

The backend sends a magic link to `SEED_ADMIN_EMAIL` on first boot (idempotent — no-op if admin already exists).

---

## Production deployment

Use this when deploying pre-built images from GitHub Container Registry. Requires the `pangolin` Docker network to exist on the host.

**1. Copy and fill the environment file**

```bash
cp .env.prod.example .env.prod
```

Required variables (see `docs/self-hosting.md` for the full list):

```bash
POSTGRES_PASSWORD=strong_random_password
RUSTFS_ENDPOINT=https://rustfs.example.com
RUSTFS_ACCESS_KEY=...
RUSTFS_SECRET_KEY=...
RUSTFS_BUCKET=fabyoumeme-assets
FRONTEND_URL=https://meme.example.com
BACKEND_URL=https://meme.example.com/api
SMTP_HOST=smtp.example.com
SMTP_USERNAME=...
SMTP_PASSWORD=...
SMTP_FROM=noreply@example.com
SEED_ADMIN_EMAIL=you@example.com
```

**2. Start all services**

```bash
make prod
```

Images are pulled from `ghcr.io/morgankryze/fabyoumeme-{backend,frontend}:latest` — no local build needed.

**3. Check your email**

The backend sends a magic link to `SEED_ADMIN_EMAIL` on first boot (idempotent — no-op if admin already exists). Click it to log in, then create invite tokens for your players.

---

## Development commands

```bash
# Rebuild and restart all services (dev-local)
make dev

# Tear down dev stack
make dev-down

# Watch backend logs
docker compose -f docker/compose.base.yml -f docker/compose.dev.yml logs -f backend

# Roll back one migration (requires golang-migrate CLI: brew install golang-migrate)
migrate -path ./backend/db/migrations \
  -database "postgres://fabyoumeme:${POSTGRES_PASSWORD}@localhost:5432/fabyoumeme?sslmode=disable" down 1

# Regenerate sqlc types after editing backend/db/queries/*.sql
cd backend && sqlc generate

# Backend: build, vet, test
cd backend && go build ./...
cd backend && go vet ./...
cd backend && go test -race -count=1 ./...

# Frontend: type-check
cd frontend && npm run check
```

---

## Architecture

Full design documentation lives in `docs/`:

| Doc                                | Contents                                                            |
| ---------------------------------- | ------------------------------------------------------------------- |
| `docs/overview.md`                 | Goals, tech stack rationale, key design decisions                   |
| `docs/architecture.md`             | System components, DB schema, storage, middleware, startup          |
| `docs/auth-and-identity.md`        | Auth flow, invite system, session management, security controls    |
| `docs/game-engine.md`              | Room/round lifecycle, WebSocket hub, game type handler interface    |
| `docs/api.md`                      | REST endpoints, WebSocket protocol, error model                     |
| `docs/frontend.md`                 | SvelteKit routing, Svelte 5 state singletons, game plugin arch      |
| `docs/self-hosting.md`             | Prerequisites, first boot, all environment variables                |
| `docs/operations.md`               | Monitoring, logs, backups, CI, production checklist                 |
| `docs/reference/error-codes.md`    | Canonical `snake_case` error code table (REST + WebSocket)          |
| `docs/reference/decisions.md`      | ADR-001–ADR-010 architectural decisions                             |
| `docs/reference/gdpr.md`           | GDPR compliance: lawful basis, rights, DPA, breach procedure        |
| `docs/reference/privacy-policy.md` | Art. 13(1) Privacy Policy stub template for operator to complete    |

---

## License

[GPLv3](LICENSE)
