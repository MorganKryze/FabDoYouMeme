# Operations

## Health checks

The backend exposes two health endpoints:

| Endpoint               | Purpose                                                  | Use for                        |
| ---------------------- | -------------------------------------------------------- | ------------------------------ |
| `GET /api/health`      | Liveness — returns `200` if the process is running       | Container liveness probes      |
| `GET /api/health/deep` | Readiness — checks DB connection and RustFS reachability | Readiness probes after startup |

`/api/health/deep` performs a `SELECT 1` against PostgreSQL and a HEAD request to the RustFS bucket, both with 2-second timeouts. It returns `503` with a `checks` object showing which dependency failed.

---

## Structured logging

The backend uses Go's `slog` for structured JSON logs. Every entry includes:

- `time` — ISO 8601 timestamp
- `level` — `DEBUG` / `INFO` / `WARN` / `ERROR`
- `msg` — a stable snake_case event identifier (see table below)
- `request_id` — correlates with the `X-Request-ID` response header
- Additional context fields (`user_id`, `duration_ms`, `error`, etc.)

Log verbosity is controlled by `LOG_LEVEL` (default `info`). At `info`, IP addresses are not logged.

### Key log events

| Event                     | Level | When                                                                          |
| ------------------------- | ----- | ----------------------------------------------------------------------------- |
| `startup`                 | INFO  | Server ready — includes version, port, and environment                        |
| `room.crash_recovery`     | INFO  | Startup: playing rooms marked finished after crash                            |
| `room.abandoned`          | INFO  | Startup: lobby rooms older than 24h marked finished                           |
| `auth_register_success`   | INFO  | User created via invite                                                       |
| `auth_magic_link_sent`    | INFO  | Magic link email dispatched                                                   |
| `auth_verify_success`     | INFO  | Token verified, session created                                               |
| `auth_verify_failure`     | WARN  | Invalid, expired, or already-used token                                       |
| `auth_logout`             | INFO  | Session deleted                                                               |
| `user_deactivated`        | WARN  | Admin set `is_active = false`                                                 |
| `admin_action`            | INFO  | Any admin route that modifies data (includes `action`, `resource`, `changes`) |
| `room_created`            | INFO  | Room row inserted                                                             |
| `room_started`            | INFO  | Lobby → playing transition                                                    |
| `room_finished`           | INFO  | Room moved to finished                                                        |
| `ws_connect`              | INFO  | WebSocket upgrade accepted                                                    |
| `ws_disconnect`           | INFO  | WebSocket closed (includes reason)                                            |
| `ws_rate_limited`         | WARN  | Connection dropped for exceeding message rate                                 |
| `ws_grace_expired`        | WARN  | Player's reconnect grace window expired                                       |
| `asset_upload_url_issued` | INFO  | Pre-signed upload URL generated                                               |
| `asset_purge`             | INFO  | Before each RustFS object deletion in admin purge                             |
| `asset_purge_failed`      | ERROR | RustFS deletion failed                                                        |
| `email_send_failure`      | ERROR | SMTP delivery failed                                                          |
| `db_query_slow`           | WARN  | Query exceeded 500ms                                                          |

---

## Prometheus metrics

Exposed at `GET /api/metrics` in Prometheus text format. This endpoint is IP-restricted by the `METRICS_ALLOWED_IPS` allowlist — never expose it publicly.

| Metric                              | Type      | Description                                                                                                |
| ----------------------------------- | --------- | ---------------------------------------------------------------------------------------------------------- |
| `http_requests_total`               | Counter   | Labelled by `method`, `status_class`, `path_pattern` — uses templated paths to avoid cardinality explosion |
| `http_request_duration_seconds`     | Histogram | Labelled by `method`, `path_pattern`                                                                       |
| `websocket_connections_active`      | Gauge     | Current open WebSocket connections                                                                         |
| `websocket_messages_total`          | Counter   | Labelled by `direction` (client_to_server / server_to_client) and `type`                                   |
| `websocket_reconnects_total`        | Counter   | Successful reconnections within grace window                                                               |
| `websocket_grace_expirations_total` | Counter   | Players whose grace window expired                                                                         |
| `game_rooms_total`                  | Gauge     | Rooms grouped by current state (lobby / playing / finished)                                                |
| `game_rounds_total`                 | Counter   | Labelled by `game_type` and `outcome` (completed / skipped / pack_exhausted)                               |
| `game_submissions_total`            | Counter   | Labelled by `game_type`                                                                                    |
| `game_votes_total`                  | Counter   | Labelled by `game_type`                                                                                    |
| `magic_links_sent_total`            | Counter   | Magic link emails dispatched                                                                               |
| `magic_links_verified_total`        | Counter   | Labelled by `result` (success / expired / used / not_found)                                                |
| `sessions_created_total`            | Counter   | Session rows inserted                                                                                      |
| `sessions_active`                   | Gauge     | Current non-expired sessions                                                                               |

---

## Backup strategy

### PostgreSQL

A nightly cron job on the host runs `pg_dump` inside the postgres container and compresses the output. Only the last 7 daily backups are kept.

**Verify the restore procedure before inviting the first users.** Restore is: decompress the dump file and pipe it into `psql` inside the running postgres container.

### GDPR and backup lag

When a user is hard-deleted, the live database is updated immediately. Daily backups created within the 7-day retention window still contain that user's data until those backups age out. This 7-day lag is acceptable under GDPR Art. 17(3)(b) for legitimate backup purposes, but must be disclosed in the privacy policy. No manual backup purge is required — the rolling 7-day deletion handles it automatically.

### RustFS assets

Asset backup is the responsibility of the external RustFS stack. The PostgreSQL volume and RustFS objects must be backed up independently and restored together for a consistent state.

---

## CI pipeline

Two GitHub Actions workflows run on every push and pull request targeting paths they care about:

**Backend (`backend/**`):\*\*

1. Go module download
2. `go build ./...`
3. `go vet ./...`
4. Run migrations against a fresh PostgreSQL 17 test container
5. `go test -race -count=1 ./...`
6. `govulncheck ./...` — dependency vulnerability scan
7. Docker build smoke test

**Frontend (`frontend/**`):\*\*

1. `npm ci`
2. `npm run check` — TypeScript type check
3. `npm run build`
4. `npm audit --audit-level=high`
5. Docker build smoke test

**What is not in CI (intentional):**

- No automatic deployment — deployments are manual (`git pull && docker compose up --build -d`)
- No staging environment — single production instance; test locally
- No end-to-end tests — deferred until the initial implementation is stable

---

## Production checklist

Before inviting the first users:

- `.env` filled in from `.env.example` — all required vars present
- `SEED_ADMIN_EMAIL` set on first boot; verify admin magic link arrives
- Reverse proxy routing: `/api/*` → backend (8080), `/*` → frontend (3000)
- TLS certificate in place — magic links will not work over plain HTTP
- PostgreSQL backup cron configured and **restore verified**
- RustFS backup strategy confirmed for the external stack
- `GET /api/metrics` IP-restricted and not reachable from the internet
- `go test ./...` passes on the deployed revision
- `govulncheck ./...` clean
- `npm audit` clean
- Docker log driver configured with `max-size` / `max-file` rotation

---

## Common maintenance tasks

**View live backend logs:**

```bash
docker compose logs -f backend
```

**Apply pending DB migrations:**

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up
```

**Roll back one migration:**

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1
```

**Regenerate sqlc types after query changes:**

```bash
cd backend && sqlc generate
```

**Check captured dev emails (Mailpit):**

```bash
open http://localhost:8025
```

**Rebuild and restart after code changes:**

```bash
docker compose up --build -d
```
