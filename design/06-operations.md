# 06 — Operations

Docker Compose orchestration, database migrations, backups, CI pipeline, structured logging, health checks, and Prometheus metrics.

Environment variables are documented in `ref-env-vars.md`.

---

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
      WS_READ_LIMIT_BYTES: ${WS_READ_LIMIT_BYTES:-4096}
      WS_READ_DEADLINE: ${WS_READ_DEADLINE:-60s}
      WS_PING_INTERVAL: ${WS_PING_INTERVAL:-25s}
      SESSION_RENEW_INTERVAL: ${SESSION_RENEW_INTERVAL:-60m}
      MAX_UPLOAD_SIZE_BYTES: ${MAX_UPLOAD_SIZE_BYTES:-2097152}
      RATE_LIMIT_AUTH_RPM: ${RATE_LIMIT_AUTH_RPM:-10}
      RATE_LIMIT_INVITE_VALIDATION_RPH: ${RATE_LIMIT_INVITE_VALIDATION_RPH:-20}
      RATE_LIMIT_ROOMS_RPH: ${RATE_LIMIT_ROOMS_RPH:-10}
      RATE_LIMIT_UPLOADS_RPH: ${RATE_LIMIT_UPLOADS_RPH:-50}
      RATE_LIMIT_GLOBAL_RPM: ${RATE_LIMIT_GLOBAL_RPM:-100}
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
      - pangolin

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

Postgres and the backend are reachable only on the internal `project_network`. The frontend joins both: `project_network` to reach the backend, and `pangolin` to be reachable by the reverse proxy. RustFS lives on `pangolin` in a separate stack.

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

# Force a specific version (after manually fixing a broken migration)
migrate -path ./backend/db/migrations -database "$DATABASE_URL" force {version}
```

### Migration Conventions

- Each migration has an `up` and `down` file
- `down` files must exactly reverse their `up` counterpart — test rollback before merging
- Destructive `down` migrations (DROP TABLE, DROP COLUMN) are acceptable — the `down` file is for dev rollback, not production recovery
- Never delete or modify a migration applied to production; add a new migration instead

### Regenerating sqlc Types

After modifying any `.sql` query file in `backend/db/queries/`:

```bash
cd backend && sqlc generate
```

---

## Backup Strategy

### PostgreSQL

Run nightly via host cron:

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
gunzip -c /var/backups/fabyoumeme/postgres_TIMESTAMP.sql.gz \
  | docker compose exec -T postgres \
    psql -U fabyoumeme fabyoumeme
```

**Test the restore procedure before inviting the first users.**

### RustFS Assets

Asset backup is the responsibility of the external RustFS stack. The `postgres_data` Docker volume must be backed up separately from RustFS objects.

---

## Startup Cleanup

On backend startup, before accepting traffic, two idempotent SQL statements run. Each emits a structured log event with the count of affected rows:

```sql
-- Emits log event "room.crash_recovery" with affected row count
-- Rooms left in 'playing' by a prior crash are unrecoverable (no active hub).
UPDATE rooms
SET state = 'finished', finished_at = now()
WHERE state = 'playing';

-- Emits log event "room.abandoned" with affected row count
-- Lobby rooms older than 24h that were never started.
UPDATE rooms
SET state = 'finished', finished_at = now()
WHERE state = 'lobby' AND created_at < now() - interval '24 hours';
```

The frontend handles reconnection to a `finished` room gracefully — see `04-protocol.md` for the recovery UX.

---

## Development Workflow

```bash
# Start all services (includes Mailpit via override)
docker compose up --build

# Watch backend logs
docker compose logs -f backend

# Run DB migrations
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

Two workflows: backend and frontend. Both run on every push and pull request to `main`.

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
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json
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

### What Is Not in CI (intentional)

- **No automatic deployment** — self-hosted personal project; deployments are manual (`git pull && docker compose up --build -d`)
- **No staging environment** — single production instance; test thoroughly locally
- **No end-to-end tests** (yet) — deferred until initial implementation is stable

---

## Production Checklist

Before inviting the first users:

- [ ] `.env` file created from `.env.example` with all required vars filled (see `ref-env-vars.md`)
- [ ] `SEED_ADMIN_EMAIL` set on first boot; idempotent on subsequent restarts
- [ ] Reverse proxy configured to route `/api/*` to backend and `/*` to frontend
- [ ] TLS certificate in place on reverse proxy (magic links require HTTPS)
- [ ] PostgreSQL backup cron configured and **restore verified**
- [ ] RustFS backup strategy in place for the external stack
- [ ] `go test ./...` passes
- [ ] `govulncheck ./...` run on backend dependencies
- [ ] `npm audit` run on frontend dependencies
- [ ] `GET /api/metrics` is IP-restricted and not publicly reachable

---

## Structured Logging

The backend uses Go's `log/slog` (stdlib) for structured JSON logs. Every entry follows this schema:

```json
{
  "time": "2026-03-27T12:34:56.789Z",
  "level": "INFO",
  "msg": "auth_verify_success",
  "service": "backend",
  "request_id": "req_01jq3abc...",
  "user_id": "uuid",
  "duration_ms": 12,
  "error": null
}
```

`request_id` is generated in logging middleware and returned as `X-Request-ID` so frontend error reports can be correlated with backend logs.

### Key Log Events

| `msg` value               | Level | When                                                                                            |
| ------------------------- | ----- | ----------------------------------------------------------------------------------------------- |
| `auth_register_success`   | INFO  | User created via invite                                                                         |
| `auth_magic_link_sent`    | INFO  | Magic link email dispatched                                                                     |
| `auth_verify_success`     | INFO  | Token verified, session created                                                                 |
| `auth_verify_failure`     | WARN  | Invalid / expired / used token                                                                  |
| `auth_logout`             | INFO  | Session deleted                                                                                 |
| `session_created`         | INFO  | Session row inserted                                                                            |
| `session_expired`         | INFO  | Session lookup rejected (expired)                                                               |
| `user_deactivated`        | WARN  | `is_active` set to false by admin                                                               |
| `admin_action`            | INFO  | Any admin route that modifies data — includes `action`, `resource`, `changes` fields            |
| `room_created`            | INFO  | Room row inserted                                                                               |
| `room_started`            | INFO  | Game state transition `lobby → playing`                                                         |
| `room_finished`           | INFO  | Game state transition `→ finished`                                                              |
| `round_started`           | INFO  | Round row inserted + `round_started` event broadcast                                            |
| `round_ended`             | INFO  | `submissions_closed` broadcast                                                                  |
| `vote_results_broadcast`  | INFO  | `vote_results` event sent                                                                       |
| `game_ended`              | INFO  | `game_ended` event broadcast + reason                                                           |
| `ws_connect`              | INFO  | WebSocket upgrade accepted                                                                      |
| `ws_disconnect`           | INFO  | WebSocket closed (includes reason)                                                              |
| `ws_rate_limited`         | WARN  | Connection dropped for exceeding rate limit                                                     |
| `ws_reconnect`            | INFO  | Player reconnected within grace window                                                          |
| `ws_grace_expired`        | WARN  | Player's grace window expired, removed from room                                                |
| `asset_upload_url_issued` | INFO  | Pre-signed upload URL generated                                                                 |
| `asset_upload_confirmed`  | INFO  | `media_key` stored on item                                                                      |
| `asset_purge`             | INFO  | Before each RustFS object deletion in admin purge (includes `media_key`, `pack_id`, `admin_id`) |
| `asset_purge_failed`      | ERROR | RustFS DELETE failed during admin purge (includes `error`)                                      |
| `email_send_failure`      | ERROR | SMTP delivery failed                                                                            |
| `db_query_slow`           | WARN  | Query exceeded 500ms (includes `query_name`, `duration_ms`)                                     |
| `startup`                 | INFO  | Server ready; includes version, port, env                                                       |
| `shutdown`                | INFO  | Graceful shutdown initiated                                                                     |
| `room.crash_recovery`     | INFO  | Startup: playing rooms marked finished after crash (includes `count`)                           |
| `room.abandoned`          | INFO  | Startup: lobby rooms older than 24h marked finished (includes `count`)                          |

---

## Health Endpoints

### `GET /api/health`

Liveness check. Returns immediately — indicates only that the process is running.

```json
200 OK
{ "status": "ok" }
```

### `GET /api/health/deep`

Readiness check. Performs active dependency checks before responding.

1. **PostgreSQL**: `SELECT 1` with a 2-second timeout
2. **RustFS**: HEAD request to the bucket root with a 2-second timeout

```json
200 OK
{ "status": "ok", "checks": { "postgres": "ok", "rustfs": "ok" } }
```

```json
503 Service Unavailable
{ "status": "degraded", "checks": { "postgres": "ok", "rustfs": "error: connection refused" } }
```

Use `GET /api/health` for container liveness probes. Use `GET /api/health/deep` for readiness probes after startup.

---

## Prometheus Metrics

Exposed at `GET /api/metrics` in Prometheus text format. **Bind to a non-public port or IP-restrict — never expose to the internet.**

### HTTP Metrics

```plain
http_requests_total{method, status_class, path_pattern}
  Counter. path_pattern uses templated paths (e.g. "/api/rooms/{code}") to avoid cardinality explosion.

http_request_duration_seconds{method, path_pattern}
  Histogram. Buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5
```

### WebSocket Metrics

```plain
websocket_connections_active
  Gauge. Current open WebSocket connections.

websocket_messages_total{direction, type}
  Counter. direction = "client_to_server" | "server_to_client"

websocket_reconnects_total
  Counter. Successful reconnections within grace window.

websocket_grace_expirations_total
  Counter. Players whose grace window expired.
```

### Game Metrics

```plain
game_rooms_total{state}
  Gauge. Rooms grouped by current state (lobby, playing, finished).

game_rounds_total{game_type, outcome}
  Counter. outcome = "completed" | "skipped" | "pack_exhausted"

game_submissions_total{game_type}
  Counter.

game_votes_total{game_type}
  Counter.
```

### Auth Metrics

```plain
magic_links_sent_total
  Counter.

magic_links_verified_total{result}
  Counter. result = "success" | "expired" | "used" | "not_found"

sessions_created_total
  Counter.

sessions_active
  Gauge. Sessions with expires_at > now() (sampled, not exact).
```

### Infrastructure Metrics

```plain
db_query_duration_seconds{query_name}
  Histogram. Labelled with sqlc query name.

db_connections_active
  Gauge. PostgreSQL connection pool in-use count.

email_sends_total{result}
  Counter. result = "success" | "failure"
```

### Alerting Thresholds

Recommended starting points; adjust once baseline is observed in production:

| Metric                                                                  | Condition                                     | Severity | Meaning                                       |
| ----------------------------------------------------------------------- | --------------------------------------------- | -------- | --------------------------------------------- |
| `magic_links_verified_total{result="expired"}` + `{result="not_found"}` | Rate > 10% of total verify attempts over 5min | WARNING  | Potential brute-force or phishing campaign    |
| `http_requests_total{status_class="5xx", path_pattern="/api/rooms"}`    | > 5% of room creation requests over 5min      | WARNING  | DB or validation issue on room creation       |
| `db_query_duration_seconds` p95                                         | > 500ms sustained over 5min                   | WARNING  | Slow query or connection pool exhaustion      |
| `websocket_grace_expirations_total`                                     | Spike > 50 in 1min                            | WARNING  | Network instability or mass host disconnect   |
| `email_sends_total{result="failure"}`                                   | Any failure                                   | ERROR    | SMTP is down; magic links cannot be delivered |

---

## Log Retention

Logs are written to stdout/stderr and captured by Docker. Recommended log rotation policy:

```json
// /etc/docker/daemon.json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "10"
  }
}
```

Retains up to 500 MB of compressed logs per container. Adjust based on available disk space.
