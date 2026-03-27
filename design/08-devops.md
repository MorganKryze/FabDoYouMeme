# 08 — DevOps & Operations

## Docker Compose

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: fabyoumeme
      POSTGRES_USER: fabyoumeme
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U fabyoumeme']
      interval: 5s
      retries: 5
    expose:
      - 5432
    networks:
      - project_network

  backend:
    build: ./backend
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://fabyoumeme:${POSTGRES_PASSWORD}@postgres:5432/fabyoumeme
      RUSTFS_ENDPOINT: ${RUSTFS_ENDPOINT}
      RUSTFS_ACCESS_KEY: ${RUSTFS_ACCESS_KEY}
      RUSTFS_SECRET_KEY: ${RUSTFS_SECRET_KEY}
      RUSTFS_BUCKET: ${RUSTFS_BUCKET:-fabyoumeme-assets}
      ALLOWED_ORIGIN: ${FRONTEND_URL}
      SMTP_HOST: ${SMTP_HOST}
      SMTP_PORT: ${SMTP_PORT:-587}
      SMTP_USERNAME: ${SMTP_USERNAME}
      SMTP_PASSWORD: ${SMTP_PASSWORD}
      SMTP_FROM: ${SMTP_FROM}
      MAGIC_LINK_BASE_URL: ${FRONTEND_URL}
      MAGIC_LINK_TTL: ${MAGIC_LINK_TTL:-15m}
      SESSION_TTL: ${SESSION_TTL:-720h}
      SEED_ADMIN_EMAIL: ${SEED_ADMIN_EMAIL}
      PORT: ${BACKEND_PORT:-8080}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      RECONNECT_GRACE_WINDOW: ${RECONNECT_GRACE_WINDOW:-30s}
      WS_RATE_LIMIT: ${WS_RATE_LIMIT:-20}
      MAX_UPLOAD_SIZE_BYTES: ${MAX_UPLOAD_SIZE_BYTES:-2097152}
    expose:
      - 8080
    networks:
      - project_network

  frontend:
    build: ./frontend
    restart: unless-stopped
    depends_on:
      - backend
    environment:
      PUBLIC_API_URL: ${BACKEND_URL}
    expose:
      - 3000
    networks:
      - project_network
      - pangolin # shared with reverse proxy

volumes:
  postgres_data:

networks:
  project_network:
    driver: bridge
  pangolin:
    external: true
```

### Network Topology

| Service  | project_network | pangolin |
| -------- | --------------- | -------- |
| postgres | ✓               |          |
| backend  | ✓               |          |
| frontend | ✓               | ✓        |

Postgres and the backend are reachable only on the internal `project_network`. The frontend joins both networks: `project_network` to reach the backend internally, and `pangolin` to be reachable by the reverse proxy. RustFS lives on the `pangolin` network in a separate stack; the backend and frontend access it over the reverse proxy (HTTPS) like any other external service.

### Dev Overrides

`docker-compose.override.yml` adds **Mailpit** for local email catching and mounts source for live reload:

```yaml
# docker-compose.override.yml (dev only — never commit with secrets)
services:
  mailpit:
    image: axllent/mailpit:latest
    expose:
      - 8025 # web UI
      - 1025 # SMTP
    networks:
      - project_network

  backend:
    environment:
      SMTP_HOST: mailpit
      SMTP_PORT: '1025'
      SMTP_USERNAME: ''
      SMTP_PASSWORD: ''
      SMTP_FROM: noreply@fabyoumeme.local
      LOG_LEVEL: debug
    volumes:
      - ./backend:/app

  frontend:
    volumes:
      - ./frontend:/app

networks:
  project_network:
    external: true # defined in docker-compose.yml
```

View captured dev emails: `http://localhost:8025`

---

## Environment Variables Reference

All variables loaded from `.env` (never committed). Copy `.env.example` and fill in values.

| Variable                 | Required | Default             | Description                                                                    |
| ------------------------ | -------- | ------------------- | ------------------------------------------------------------------------------ |
| `POSTGRES_PASSWORD`      | yes      | —                   | PostgreSQL password                                                            |
| `DATABASE_URL`           | auto     | —                   | Constructed in compose from `POSTGRES_PASSWORD`                                |
| `RUSTFS_ENDPOINT`        | yes      | —                   | e.g. `https://rustfs.example.com`                                              |
| `RUSTFS_ACCESS_KEY`      | yes      | —                   | RustFS access key                                                              |
| `RUSTFS_SECRET_KEY`      | yes      | —                   | RustFS secret key                                                              |
| `RUSTFS_BUCKET`          | no       | `fabyoumeme-assets` | S3 bucket name                                                                 |
| `FRONTEND_URL`           | yes      | —                   | Full URL e.g. `https://meme.example.com` — used for CORS, cookies, magic links |
| `BACKEND_URL`            | yes      | —                   | Full URL e.g. `https://meme.example.com/api` — sent to frontend                |
| `SMTP_HOST`              | yes      | —                   | SMTP server hostname                                                           |
| `SMTP_PORT`              | no       | `587`               | SMTP port                                                                      |
| `SMTP_USERNAME`          | yes      | —                   | SMTP username                                                                  |
| `SMTP_PASSWORD`          | yes      | —                   | SMTP password                                                                  |
| `SMTP_FROM`              | yes      | —                   | From address e.g. `noreply@example.com`                                        |
| `MAGIC_LINK_TTL`         | no       | `15m`               | Magic link expiry duration                                                     |
| `SESSION_TTL`            | no       | `720h`              | Session expiry (30 days)                                                       |
| `SEED_ADMIN_EMAIL`       | no       | —                   | Email for first-boot admin creation (idempotent)                               |
| `BACKEND_PORT`           | no       | `8080`              | Backend HTTP port                                                              |
| `LOG_LEVEL`              | no       | `info`              | `debug` / `info` / `warn` / `error`                                            |
| `RECONNECT_GRACE_WINDOW` | no       | `30s`               | WS reconnect grace period                                                      |
| `WS_RATE_LIMIT`          | no       | `20`                | WebSocket messages/second per connection                                       |
| `MAX_UPLOAD_SIZE_BYTES`  | no       | `2097152`           | Max file upload size (2 MB)                                                    |
| `PUBLIC_API_URL`         | yes      | —                   | Frontend env var pointing to backend                                           |

---

## Database Migrations

Migrations live in `backend/db/migrations/` as `{version}_{description}.up.sql` / `.down.sql` pairs.

### Common Commands

```bash
# Apply all pending migrations
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1

# Check current migration version
migrate -path ./backend/db/migrations -database "$DATABASE_URL" version

# Force a specific version (use after manually fixing a broken migration)
migrate -path ./backend/db/migrations -database "$DATABASE_URL" force {version}
```

### Migration Conventions

- Each migration has an `up` and `down` file
- `down` files must reverse their `up` counterpart exactly — test rollback before merging
- Destructive `down` migrations (DROP TABLE, DROP COLUMN) are acceptable — the `down` file is for development rollback, not production recovery
- Never delete or modify a migration that has been applied to production; add a new migration instead

### Regenerating sqlc Types

After modifying any `.sql` query file in `backend/db/queries/`:

```bash
cd backend && sqlc generate
```

---

## Backup Strategy

### PostgreSQL

Run nightly via host cron. Adjust paths to match your system:

```bash
#!/bin/bash
# /etc/cron.daily/fabyoumeme-backup
BACKUP_DIR=/var/backups/fabyoumeme
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

docker compose -f /path/to/FabDoYouMeme/docker-compose.yml \
  exec -T postgres \
  pg_dump -U fabyoumeme fabyoumeme \
  | gzip > "$BACKUP_DIR/postgres_$TIMESTAMP.sql.gz"

# Retain only the last 7 daily backups
find "$BACKUP_DIR" -name 'postgres_*.sql.gz' -mtime +7 -delete
```

### Restore Procedure

```bash
# Restore from a backup file
gunzip -c /var/backups/fabyoumeme/postgres_TIMESTAMP.sql.gz \
  | docker compose exec -T postgres \
    psql -U fabyoumeme fabyoumeme
```

**Test the restore procedure before inviting the first users.** A backup you have never tested is not a backup.

### RustFS Assets

Asset backup is the responsibility of the external RustFS stack. The `postgres_data` Docker volume must be backed up separately from RustFS objects.

---

## Startup Cleanup

On backend startup, before accepting traffic, the server runs two idempotent SQL statements:

```sql
-- Mark any rooms that were left in 'playing' state by a prior crash as 'finished'.
-- These rooms have no active hub and are unrecoverable.
UPDATE rooms
SET state = 'finished', finished_at = now()
WHERE state = 'playing';

-- Mark any rooms that were left in 'lobby' state for more than 24 hours.
-- These are abandoned rooms that were never started.
UPDATE rooms
SET state = 'finished', finished_at = now()
WHERE state = 'lobby' AND created_at < now() - interval '24 hours';
```

This is logged as a `startup_cleanup` event with the count of affected rows. The frontend handles reconnection to a `finished` room gracefully — see [06-game-engine.md](06-game-engine.md) for the recovery UX.

---

## Development Workflow

```bash
# Start all services (includes Mailpit via override)
docker compose up --build

# Watch backend logs
docker compose logs -f backend

# Run DB migrations (requires migrate CLI or run inside backend container)
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up

# Regenerate sqlc types after query changes
cd backend && sqlc generate

# Frontend dev server (hot reload, bypasses Docker)
cd frontend && npm run dev

# View captured dev emails
open http://localhost:8025
```

---

## CI Pipeline (GitHub Actions)

Two workflows: one for the backend, one for the frontend. Both run on every push and pull request to `main`.

### Backend (`.github/workflows/backend.yml`)

```yaml
name: Backend CI
on:
  push:
    paths: ['backend/**', '.github/workflows/backend.yml']
  pull_request:
    paths: ['backend/**']

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:17-alpine
        env:
          POSTGRES_DB: fabyoumeme_test
          POSTGRES_USER: fabyoumeme
          POSTGRES_PASSWORD: testpassword
        ports: ['5432:5432']
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23' }

      - name: Download modules
        working-directory: backend
        run: go mod download

      - name: Build
        working-directory: backend
        run: go build ./...

      - name: Vet
        working-directory: backend
        run: go vet ./...

      - name: Run migrations
        working-directory: backend
        env:
          DATABASE_URL: postgres://fabyoumeme:testpassword@localhost:5432/fabyoumeme_test
        run: |
          go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
          migrate -path ./db/migrations -database "$DATABASE_URL" up

      - name: Test
        working-directory: backend
        env:
          DATABASE_URL: postgres://fabyoumeme:testpassword@localhost:5432/fabyoumeme_test
        run: go test -race -count=1 ./...

      - name: Vulnerability check
        working-directory: backend
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

      - name: Docker build smoke test
        run: docker build -t fabyoumeme-backend:ci ./backend
```

### Frontend (`.github/workflows/frontend.yml`)

```yaml
name: Frontend CI
on:
  push:
    paths: ['frontend/**', '.github/workflows/frontend.yml']
  pull_request:
    paths: ['frontend/**']

jobs:
  build-and-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          {
            node-version: '22',
            cache: 'npm',
            cache-dependency-path: frontend/package-lock.json
          }

      - name: Install
        working-directory: frontend
        run: npm ci

      - name: Type check
        working-directory: frontend
        run: npm run check

      - name: Build
        working-directory: frontend
        env:
          PUBLIC_API_URL: http://localhost:8080
        run: npm run build

      - name: Audit dependencies
        working-directory: frontend
        run: npm audit --audit-level=high

      - name: Docker build smoke test
        run: docker build -t fabyoumeme-frontend:ci ./frontend
```

### What is Not in CI (intentional)

- **No automatic deployment** — this is a self-hosted personal project; deployments are manual (`git pull && docker compose up --build -d`)
- **No staging environment** — single production instance; test thoroughly locally before pushing to `main`
- **No end-to-end tests** (yet) — deferred until the initial implementation is stable

---

## Production Checklist

Before inviting the first users:

- [ ] `.env` file created from `.env.example` with all required vars filled
- [ ] `SEED_ADMIN_EMAIL` set on first boot; cleared or left in place (idempotent) afterwards
- [ ] Reverse proxy configured to route `/api/*` to backend and `/*` to frontend
- [ ] TLS certificate in place on reverse proxy (magic links require HTTPS)
- [ ] PostgreSQL backup cron configured and tested (restore verified)
- [ ] RustFS backup strategy in place for the external stack
- [ ] `go test ./...` passes (once test suite exists)
- [ ] `govulncheck ./...` run on backend dependencies
- [ ] `npm audit` run on frontend dependencies
