# Self-Hosting Guide

## Prerequisites

- Docker and Docker Compose installed on the host
- A pre-existing reverse proxy (e.g. Nginx, Traefik, Pangolin) routing:
  - `/api/*` → backend (port 8080)
  - `/*` → frontend (port 3000)
- An SMTP server or relay for magic link delivery
- A running RustFS instance accessible from the Docker network (the project uses an external `pangolin` network to reach it)
- DNS pointing your domain to the host

---

## First boot

1. **Copy `.env.example` to `.env`** and fill in all required values (see [Environment variables](#environment-variables) below).

2. **Set `SEED_ADMIN_EMAIL`** to your email address. On first startup, the backend detects that no admin exists and automatically creates the admin user + sends a magic link to that address. Subsequent restarts with the same env var are no-ops.

3. **Start the stack:**

   ```plain
   docker compose up --build
   ```

4. **Check your inbox.** Click the magic link to log in as admin. The link expires in 15 minutes. If it expires before you click, request a new one from the login page.

5. **Create invites** from the Admin panel so other players can register.

---

## Development vs production

The repository ships a `docker-compose.override.yml` that activates automatically in development. It:

- Starts **Mailpit** (a local SMTP catcher) on port `8025` — captured emails are viewable at `http://localhost:8025`
- Mounts source directories as volumes for hot-reload

In production, `docker-compose.override.yml` should not be present. Use `docker compose -f docker-compose.yml up` if you want to be explicit.

---

## Applying database migrations

Migrations run automatically at backend startup via `golang-migrate`. To run them manually:

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" up
```

To roll back one migration:

```bash
migrate -path ./backend/db/migrations -database "$DATABASE_URL" down 1
```

---

## Environment variables

All variables come from `.env`. Never commit this file.

### Core infrastructure

| Variable            | Required | Default             | Description                                                                 |
| ------------------- | -------- | ------------------- | --------------------------------------------------------------------------- |
| `POSTGRES_PASSWORD` | yes      | —                   | PostgreSQL password                                                         |
| `DATABASE_URL`      | auto     | —                   | Constructed by Docker Compose from `POSTGRES_PASSWORD`; do not set manually |
| `RUSTFS_ENDPOINT`   | yes      | —                   | RustFS base URL, e.g. `https://rustfs.example.com`                          |
| `RUSTFS_ACCESS_KEY` | yes      | —                   | RustFS access key                                                           |
| `RUSTFS_SECRET_KEY` | yes      | —                   | RustFS secret key                                                           |
| `RUSTFS_BUCKET`     | no       | `fabyoumeme-assets` | S3 bucket name                                                              |

### URLs & routing

| Variable       | Required | Default | Description                                                                                              |
| -------------- | -------- | ------- | -------------------------------------------------------------------------------------------------------- |
| `FRONTEND_URL` | yes      | —       | Full public URL, e.g. `https://meme.example.com` — used for CORS, cookie domain, and magic link base URL |
| `BACKEND_URL`  | yes      | —       | Full API URL, e.g. `https://meme.example.com/api`                                                        |
| `BACKEND_PORT` | no       | `8080`  | Backend HTTP listen port                                                                                 |

### Email / SMTP

| Variable        | Required | Default | Description                                                        |
| --------------- | -------- | ------- | ------------------------------------------------------------------ |
| `SMTP_HOST`     | yes      | —       | SMTP server hostname                                               |
| `SMTP_PORT`     | no       | `587`   | SMTP port (587 = STARTTLS)                                         |
| `SMTP_USERNAME` | no       | —       | SMTP auth username (omit for unauthenticated servers like Mailpit) |
| `SMTP_PASSWORD` | no       | —       | SMTP auth password (omit for unauthenticated servers like Mailpit) |
| `SMTP_FROM`     | yes      | —       | From address, e.g. `noreply@example.com`                           |

> **GDPR note:** your SMTP provider is a data processor under GDPR Art. 28(3). A Data Processing Agreement must be in place before production use.

### Auth & sessions

| Variable                 | Required | Default | Description                                                                          |
| ------------------------ | -------- | ------- | ------------------------------------------------------------------------------------ |
| `SEED_ADMIN_EMAIL`       | no       | —       | Email for first-boot admin bootstrap. Idempotent: skipped if an admin already exists |
| `MAGIC_LINK_TTL`         | no       | `15m`   | How long a magic link is valid                                                       |
| `SESSION_TTL`            | no       | `720h`  | Session expiry (30 days); renewed on each authenticated request                      |
| `SESSION_RENEW_INTERVAL` | no       | `60m`   | How often the hub renews sessions for long-running WebSocket connections             |

### WebSocket

| Variable                 | Required | Default | Description                                                                     |
| ------------------------ | -------- | ------- | ------------------------------------------------------------------------------- |
| `RECONNECT_GRACE_WINDOW` | no       | `30s`   | How long a disconnected player can be absent before being removed from the room |
| `WS_RATE_LIMIT`          | no       | `20`    | Maximum messages per second per WebSocket connection                            |
| `WS_READ_LIMIT_BYTES`    | no       | `4096`  | Maximum WebSocket frame size; exceeding this disconnects the client             |
| `WS_PING_INTERVAL`       | no       | `25s`   | Client-side ping interval                                                       |
| `WS_READ_DEADLINE`       | no       | `60s`   | Server-side read deadline per connection; reset on each pong                    |

### Rate limits

All rate limits are enforced in-memory per backend process. This is correct for a single-host deployment. For multi-instance deployments, rate limit state must be externalized to Redis.

| Variable                           | Required | Default | Description                                                         |
| ---------------------------------- | -------- | ------- | ------------------------------------------------------------------- |
| `RATE_LIMIT_AUTH_RPM`              | no       | `10`    | Max requests/minute per IP on all `POST /api/auth/*` endpoints      |
| `RATE_LIMIT_INVITE_VALIDATION_RPH` | no       | `20`    | Max registration attempts/hour per IP (prevents invite brute-force) |
| `RATE_LIMIT_ROOMS_RPH`             | no       | `10`    | Max room creations/hour per user                                    |
| `RATE_LIMIT_UPLOADS_RPH`           | no       | `50`    | Max upload URL requests/hour per admin                              |
| `RATE_LIMIT_GLOBAL_RPM`            | no       | `100`   | Max requests/minute per IP on all other `GET /api/*` endpoints      |

### Uploads

| Variable                | Required | Default   | Description                        |
| ----------------------- | -------- | --------- | ---------------------------------- |
| `MAX_UPLOAD_SIZE_BYTES` | no       | `2097152` | Maximum upload size (default 2 MB) |

### Logging

| Variable    | Required | Default | Description                                    |
| ----------- | -------- | ------- | ---------------------------------------------- |
| `LOG_LEVEL` | no       | `info`  | Verbosity: `debug` / `info` / `warn` / `error` |

---

## Secrets rotation

| Secret                                    | Impact when rotated                         | Procedure                                                           |
| ----------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------- |
| `POSTGRES_PASSWORD`                       | All backend connections drop                | Update `.env`, restart backend + postgres                           |
| `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` | All asset operations fail                   | Rotate in RustFS admin console, update `.env`, restart backend      |
| `SMTP_PASSWORD`                           | Email delivery fails (magic links broken)   | Update `.env`, restart backend                                      |
| Session tokens (in DB)                    | None — tokens are random and self-contained | Invalidate individual sessions by deleting rows in `sessions` table |

---

## Network topology

| Service  | Internal network | Pangolin (external) |
| -------- | ---------------- | ------------------- |
| postgres | ✓                |                     |
| backend  | ✓                |                     |
| frontend | ✓                | ✓                   |

PostgreSQL and the backend are not reachable outside the Docker internal network. The frontend sits on the `pangolin` external network so the reverse proxy can route traffic to it. RustFS lives in a separate Docker stack on the same `pangolin` network; the backend reaches it via `RUSTFS_ENDPOINT`.
