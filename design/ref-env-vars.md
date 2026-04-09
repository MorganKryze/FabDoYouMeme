# ref — Environment Variables

All variables are loaded from `.env` (never committed to version control). Copy `.env.example` and fill in values before first boot.

> **Rate-limit variables** are enforced in-memory per backend process. This is correct for the single-host Docker Compose deployment. If multi-instance deployment is ever added, rate-limit state must be externalized to Redis — see ADR-005 in `ref-decisions.md`.

---

## Core Infrastructure

| Variable            | Required | Default             | Description                                                                 |
| ------------------- | -------- | ------------------- | --------------------------------------------------------------------------- |
| `POSTGRES_PASSWORD` | yes      | —                   | PostgreSQL password                                                         |
| `DATABASE_URL`      | auto     | —                   | Constructed in Docker Compose from `POSTGRES_PASSWORD`; do not set manually |
| `RUSTFS_ENDPOINT`   | yes      | —                   | RustFS base URL, e.g. `https://rustfs.example.com`                          |
| `RUSTFS_ACCESS_KEY` | yes      | —                   | RustFS access key (create in RustFS admin console)                          |
| `RUSTFS_SECRET_KEY` | yes      | —                   | RustFS secret key                                                           |
| `RUSTFS_BUCKET`     | no       | `fabyoumeme-assets` | S3 bucket name                                                              |

## URLs & Routing

| Variable         | Required       | Default | Description                                                                                              |
| ---------------- | -------------- | ------- | -------------------------------------------------------------------------------------------------------- |
| `FRONTEND_URL`   | yes            | —       | Full public URL, e.g. `https://meme.example.com` — used for CORS, cookie domain, and magic link base URL |
| `BACKEND_URL`    | yes            | —       | Full API URL, e.g. `https://meme.example.com/api` — sent to frontend as `PUBLIC_API_URL`                 |
| `PUBLIC_API_URL` | yes (frontend) | —       | Frontend-side env var pointing to backend; set automatically in Docker Compose from `BACKEND_URL`        |
| `BACKEND_PORT`   | no             | `8080`  | Backend HTTP listen port                                                                                 |

## Email / SMTP

| Variable        | Required | Default | Description                              |
| --------------- | -------- | ------- | ---------------------------------------- |
| `SMTP_HOST`     | yes      | —       | SMTP server hostname                     |
| `SMTP_PORT`     | no       | `587`   | SMTP port (587 = STARTTLS)               |
| `SMTP_USERNAME` | no       | —       | SMTP auth username (omit for unauthenticated servers like Mailpit) |
| `SMTP_PASSWORD` | no       | —       | SMTP auth password (omit for unauthenticated servers like Mailpit) |
| `SMTP_FROM`     | yes      | —       | From address, e.g. `noreply@example.com` |

## Auth & Session

| Variable                 | Required | Default | Description                                                                        |
| ------------------------ | -------- | ------- | ---------------------------------------------------------------------------------- |
| `MAGIC_LINK_TTL`         | no       | `15m`   | Magic link expiry duration                                                         |
| `SESSION_TTL`            | no       | `720h`  | Session expiry (30 days)                                                           |
| `SEED_ADMIN_EMAIL`       | no       | —       | Email for first-boot admin bootstrap. Idempotent: no-op if an admin already exists |
| `SESSION_RENEW_INTERVAL` | no       | `60m`   | How often the hub renews sessions for long-running WebSocket connections           |

## WebSocket

| Variable                 | Required | Default | Description                                                                          |
| ------------------------ | -------- | ------- | ------------------------------------------------------------------------------------ |
| `RECONNECT_GRACE_WINDOW` | no       | `30s`   | How long a disconnected player has to reconnect before being removed from the room   |
| `WS_RATE_LIMIT`          | no       | `20`    | Maximum WebSocket messages per second per connection                                 |
| `WS_READ_LIMIT_BYTES`    | no       | `4096`  | Maximum WebSocket frame size in bytes; exceeding this disconnects the client         |
| `WS_PING_INTERVAL`       | no       | `25s`   | Client-side ping interval (documented here for reference; enforced by frontend code) |
| `WS_READ_DEADLINE`       | no       | `60s`   | Server-side read deadline per connection; reset on each pong                         |

## Rate Limits

All rate limits are in-memory per backend process. See ADR-005 in `ref-decisions.md` for the multi-instance caveat.

| Variable                           | Required | Default | Description                                                                                 |
| ---------------------------------- | -------- | ------- | ------------------------------------------------------------------------------------------- |
| `RATE_LIMIT_AUTH_RPM`              | no       | `10`    | Max requests/minute per IP on all `POST /api/auth/*` endpoints                              |
| `RATE_LIMIT_INVITE_VALIDATION_RPH` | no       | `20`    | Max registration attempts/hour per IP (separate, lower limit to prevent invite brute-force) |
| `RATE_LIMIT_ROOMS_RPH`             | no       | `10`    | Max room creations/hour per user                                                            |
| `RATE_LIMIT_UPLOADS_RPH`           | no       | `50`    | Max upload URL requests/hour per admin                                                      |
| `RATE_LIMIT_GLOBAL_RPM`            | no       | `100`   | Max requests/minute per IP on all other `GET /api/*` endpoints                              |

## Uploads

| Variable                | Required | Default   | Description                              |
| ----------------------- | -------- | --------- | ---------------------------------------- |
| `MAX_UPLOAD_SIZE_BYTES` | no       | `2097152` | Maximum file upload size (default: 2 MB) |

## Logging

| Variable    | Required | Default | Description                                        |
| ----------- | -------- | ------- | -------------------------------------------------- |
| `LOG_LEVEL` | no       | `info`  | Log verbosity: `debug` / `info` / `warn` / `error` |
