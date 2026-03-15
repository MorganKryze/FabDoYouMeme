# FabDoYouMeme — Initial Design Reference

> Status: pre-implementation. This document is the authoritative architecture reference.
> Update it whenever a significant decision changes.

---

## 1. Project Overview

FabDoYouMeme is a self-hosted, invite-only **multi-game platform** that currently launches with meme-style games (caption, match, vote) but is architected to host any game type. Players join live multiplayer rooms or practice solo. All content (images, game packs) is managed by admins.

**License**: GPLv3
**Hosting**: Self-hosted on personal hardware, Docker Compose only
**Reverse proxy**: Pre-existing (not in scope here — assumed to route `/api/*` to backend and `/*` to frontend)

### Design Priorities

1. **Least attack surface** — no passwords, no public asset access, all secrets injected, minimal exposed ports
2. **Multi-game extensibility** — game types are registered units; adding a new game type should not require schema or protocol changes
3. **Simplicity** — self-hosted single-machine footprint, no distributed systems complexity

---

## 2. Tech Stack

| Layer          | Technology                 | Rationale                                                              |
| -------------- | -------------------------- | ---------------------------------------------------------------------- |
| Frontend       | SvelteKit (`adapter-node`) | Reactive, lightweight, SSR support, excellent DX                       |
| Backend API    | Go + `chi` router          | Simple, fast, tiny images, native goroutine concurrency for WebSockets |
| Database       | PostgreSQL 17              | Reliable, strongly typed, ideal for relational game/user data          |
| File storage   | Rustfs                     | S3-compatible, Rust-native, self-hostable                              |
| DB migrations  | `golang-migrate`           | CLI + library, supports up/down, integrates with CI                    |
| Query layer    | `sqlc`                     | Generates type-safe Go code from raw SQL — no ORM footprint            |
| WebSockets     | `gorilla/websocket`        | De-facto Go WS library, handles hub pattern well                       |
| Email          | `go-mail` (wneessen)       | Idiomatic Go SMTP library with TLS; used for magic link delivery       |
| Session tokens | Opaque tokens (DB-backed)  | Simpler and more revocable than JWT for a closed platform              |
| S3 client      | `aws-sdk-go-v2/s3`         | Works with any S3-compatible backend including Rustfs                  |
| Container      | Docker Compose             | Single-machine orchestration, straightforward service graph            |

### Why magic links instead of passwords?

Passwords are a large attack surface: credential stuffing, brute force, and storage breaches all require mitigation. On an invite-only platform where users are known, eliminating passwords removes that entire class of risk. Magic links are one-time-use, short-lived, and rely on email delivery as the second factor. The only credential stored is a SHA-256 token hash — no secret to crack if the DB leaks.

### Why not JWT?

For a closed, invite-only platform on personal hardware, DB-backed sessions are simpler, instantly revocable (logout = delete row), and eliminate token-replay edge cases. The performance overhead of a session lookup is negligible at this scale.

### Why `chi` over Gin/Echo?

Chi is idiomatic stdlib-compatible Go (uses `net/http` interfaces directly), has zero reflection, and is trivially auditable. Gin and Echo add abstraction that isn't needed here.

---

## 3. Repository Structure

```plain
FabDoYouMeme/
├── backend/
│   ├── cmd/
│   │   └── server/          # main.go entrypoint
│   ├── internal/
│   │   ├── auth/            # session management, magic link logic, invite logic
│   │   ├── game/            # game type registry, pack, room, round logic
│   │   ├── storage/         # Rustfs / S3 client wrapper (interface-backed)
│   │   ├── email/           # email rendering + sending
│   │   ├── middleware/      # auth, rate-limit, logging
│   │   └── config/          # env-based config loading
│   ├── db/
│   │   ├── migrations/      # golang-migrate SQL files
│   │   └── queries/         # sqlc .sql files → generated Go
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/             # shared components, stores, API client
│   │   ├── routes/          # SvelteKit file-based routing
│   │   └── app.html
│   ├── Dockerfile
│   ├── svelte.config.js
│   └── package.json
├── docker-compose.yml
├── docker-compose.override.yml   # local dev overrides (Mailpit, volume mounts, etc.)
├── initial-design.md
├── CLAUDE.md
├── LICENSE
└── README.md
```

---

## 4. Authentication & Invite System

### Principles

- No passwords — email + magic link only
- Email is the user's identity anchor; no PII beyond username and email
- Sessions stored in DB: instant revocation, simple audit trail; **TTL: 30 days**, renewed on activity — players stay logged in across game nights without re-authenticating
- All session cookies: `HttpOnly`, `Secure`, `SameSite=Strict`
- Magic link tokens: one-time use, 15-minute TTL, SHA-256 hash stored (never the raw token)
- Email enumeration protection: magic link endpoint always returns `200` regardless of whether the email exists
- Session tokens are random 32-byte values from `crypto/rand`, hex-encoded; the SHA-256 is stored in the DB — no HMAC signing key required

### Invite Token Model

Admin creates an invite with:

- `token` — random 12-char alphanumeric string (human-typeable or included in a URL)
- `max_uses` — `0` = unlimited, `N` = exactly N registrations allowed
- `restricted_email` — optional; if set, only that email address may use this invite
- `expires_at` — nullable; null = never expires
- `label` — optional human note (e.g. "gaming night 2026-03")

Registration validates token (not expired, uses < max_uses, email matches if restricted), creates user, increments use count atomically.

**Streamlined onboarding when `restricted_email` is set**: after the account is created, the backend immediately sends the first magic link to the registered email without the user needing to make a separate request. The player receives one email, clicks once, and is in. This collapses the 4-step flow into 2 (register → click link).

### Auth Flow

```plain
POST /api/auth/register      { invite_token, username, email }  → creates account; if invite.restricted_email matches, auto-sends magic link
POST /api/auth/magic-link    { email }                          → sends magic link email (always 200)
POST /api/auth/verify        { token }                          → validates token, creates session or confirms email change
POST /api/auth/logout        session cookie                     → deletes session row
GET  /api/auth/me            session cookie                     → current user info
```

**Why `POST /api/auth/verify` instead of `GET`**: magic link emails are scanned by security tools and email clients that pre-fetch URLs. A `GET` would consume the one-time token before the user clicks. Instead, the email link targets a frontend route (`/auth/verify?token=xxx`) which renders an intermediate "Log in" button. That button `POST`s the token to the backend. One extra user click eliminates the pre-fetch risk entirely.

### Email Change Flow

Users and admins can change the email address on an account:

- **Admin path**: `PATCH /api/admin/users/:id { email }` — updates `email` immediately, no verification required. **Always sends a notification to the old email address** informing the user the change occurred (prevents silent lockout).
- **Self-service path**: `PATCH /api/users/me { email }` — stores the new address in `pending_email`, sends a magic link with `purpose = email_change` to that address. When the user verifies, `POST /api/auth/verify` detects `purpose = email_change`, swaps `email ← pending_email`, clears `pending_email`, invalidates all existing sessions, and notifies the old address.

Username is changeable freely via `PATCH /api/users/me { username }` by the user, or `PATCH /api/admin/users/:id { username }` by an admin, with no verification step (it is a display handle, not an auth credential).

On `POST /api/auth/verify`:

1. Hash the incoming token with SHA-256
2. Look up the hash in `magic_link_tokens`
3. Check: not expired, not already used
4. Mark token as used (`used_at = now()`)
5. If `purpose = login`: create a new session row, set `HttpOnly` cookie (30-day TTL), redirect to lobby
6. If `purpose = email_change`: update `users.email ← pending_email`, clear `pending_email`, delete all existing sessions for the user, notify old email, set new session cookie

---

## 5. Database Schema

```sql
-- Users
-- username is the unique human-facing handle (used in-game and as display identity).
-- email is the magic link delivery channel and is changeable (see pending_email).
-- Admin can update both email and username directly; user can update both self-service
-- (email change requires verification via magic link to the new address).
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username      TEXT NOT NULL UNIQUE,
  email         TEXT NOT NULL UNIQUE,
  pending_email TEXT,                            -- set when user requests email change; cleared on confirm
  role          TEXT NOT NULL DEFAULT 'player',  -- 'player' | 'admin'
  is_active     BOOLEAN NOT NULL DEFAULT true,   -- false = deactivated by admin; cannot log in
  invited_by    UUID REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sessions (30-day TTL; renewed on activity to keep players logged in across game nights)
CREATE TABLE sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of the random 32-byte cookie value
  expires_at  TIMESTAMPTZ NOT NULL,   -- now() + 30 days; refreshed on each authenticated request
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Magic link tokens (login and email-change confirmation)
CREATE TABLE magic_link_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of the raw token sent by email
  purpose     TEXT NOT NULL DEFAULT 'login',  -- 'login' | 'email_change'
  expires_at  TIMESTAMPTZ NOT NULL,   -- short TTL: 15 minutes
  used_at     TIMESTAMPTZ,            -- null = not yet used
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invites
CREATE TABLE invites (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token             TEXT NOT NULL UNIQUE,
  created_by        UUID NOT NULL REFERENCES users(id),
  label             TEXT,
  restricted_email  TEXT,              -- null = any email may use this invite
  max_uses          INT NOT NULL DEFAULT 1,   -- 0 = unlimited
  uses_count        INT NOT NULL DEFAULT 0,
  expires_at        TIMESTAMPTZ,       -- null = never
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Game type registry
-- Each game type defines a discrete set of mechanics (e.g. meme-caption, trivia).
-- Seeded via migration; new game types added by new migrations.
CREATE TABLE game_types (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        TEXT NOT NULL UNIQUE,   -- e.g. 'meme-caption', 'trivia', 'drawing'
  name        TEXT NOT NULL,
  description TEXT,
  version     TEXT NOT NULL DEFAULT '1.0.0',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Game packs (portable content libraries; not tied to a specific game type)
-- A pack of meme images can be used for 'meme-caption', 'meme-vote', or any future type.
-- The room configuration is where game type and pack are brought together.
-- Soft-deleted packs (deleted_at IS NOT NULL) are hidden from the UI but historical game data is preserved.
CREATE TABLE game_packs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  description TEXT,
  created_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ                        -- null = active; set to soft-delete
);

-- Game items (generic content units)
-- payload JSONB holds all item data. Game type handlers read the fields they need;
-- unknown fields are ignored. A single item can be valid for multiple game types.
CREATE TABLE game_items (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  pack_id    UUID NOT NULL REFERENCES game_packs(id) ON DELETE CASCADE,
  position   INT NOT NULL DEFAULT 0,         -- ordering within pack
  media_key  TEXT,                           -- Rustfs object key, nullable if text-only
  payload    JSONB NOT NULL DEFAULT '{}',    -- open content bag; see payload convention below
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Rooms (live game sessions; supports multiplayer and solo)
CREATE TABLE rooms (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code         TEXT NOT NULL UNIQUE,   -- human-readable join code e.g. "MEME-4829"
  game_type_id UUID NOT NULL REFERENCES game_types(id),
  pack_id      UUID NOT NULL REFERENCES game_packs(id),
  host_id      UUID NOT NULL REFERENCES users(id),
  mode         TEXT NOT NULL DEFAULT 'multiplayer',  -- 'multiplayer' | 'solo'
  state        TEXT NOT NULL DEFAULT 'lobby',        -- 'lobby' | 'playing' | 'finished'
  config       JSONB NOT NULL DEFAULT '{}',          -- round count, time limits, etc.
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_players (
  room_id   UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  user_id   UUID NOT NULL REFERENCES users(id),
  score     INT NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (room_id, user_id)
);

CREATE TABLE rounds (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id      UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  item_id      UUID NOT NULL REFERENCES game_items(id),
  round_number INT NOT NULL,
  started_at   TIMESTAMPTZ,
  ended_at     TIMESTAMPTZ,
  UNIQUE (room_id, round_number)   -- prevents duplicate round numbers in the same room
);

-- payload JSONB matches game_items.payload convention — flexible for all game types.
-- e.g. text answer: {"text": "..."}, image pick: {"item_id": "..."}, drawing: {"strokes": [...]}
CREATE TABLE submissions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  round_id   UUID NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id),
  payload    JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (round_id, user_id)
);

-- value holds the vote weight/rank; JSONB to support varied game type scoring models.
-- e.g. simple upvote: {"points": 1}, ranked choice: {"rank": 2}, star rating: {"stars": 4}
CREATE TABLE votes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  voter_id      UUID NOT NULL REFERENCES users(id),
  value         JSONB NOT NULL DEFAULT '{"points": 1}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (submission_id, voter_id)
);
```

### Game item payload convention

Packs and items are **game-type-agnostic**. The room configuration is where a game type and a pack are paired — the same pack of meme images can be used for `meme-caption`, `meme-vote`, or any future type without duplicating content.

Each game type's backend handler reads the `payload` fields it needs and ignores the rest. Admins populate items with whichever fields are relevant; a well-authored item can serve multiple game types simultaneously.

```jsonc
// An item that works for both 'meme-caption' (uses "prompt") and 'meme-vote' (uses media_key only)
{ "prompt": "When the CI passes on the first try" }

// A text-only trivia item (no media_key)
{ "question": "What year was Go released?", "answers": ["2007", "2009", "2012", "2015"], "correct": 1 }
```

Adding a new game type requires: a new migration seeding `game_types`, a new backend handler package under `internal/game/`, and new frontend route components. No schema changes needed.

---

## 6. API Surface (REST + WebSocket)

### REST (Go + chi)

| Method | Path                       | Auth    | Description                                      |
| ------ | -------------------------- | ------- | ------------------------------------------------ |
| POST   | `/api/auth/register`       | —       | Register with invite token                       |
| POST   | `/api/auth/magic-link`     | —       | Request magic link (always 200)                  |
| POST   | `/api/auth/verify`         | —       | Verify magic link token, create session          |
| POST   | `/api/auth/logout`         | session | Logout                                           |
| GET    | `/api/auth/me`             | session | Current user                                     |
| PATCH  | `/api/users/me`            | session | Update own username or request email change      |
| GET    | `/api/admin/invites`       | admin   | List invites                                     |
| POST   | `/api/admin/invites`       | admin   | Create invite                                    |
| DELETE | `/api/admin/invites/:id`   | admin   | Revoke invite                                    |
| GET    | `/api/admin/users`         | admin   | List users                                       |
| PATCH  | `/api/admin/users/:id`     | admin   | Update role or is_active (deactivate/reactivate) |
| GET    | `/api/game-types`          | session | List available game types                        |
| GET    | `/api/packs`               | session | List game packs                                  |
| POST   | `/api/packs`               | admin   | Create pack                                      |
| POST   | `/api/packs/:id/items`     | admin   | Add item (with optional image upload)            |
| DELETE | `/api/packs/:id`           | admin   | Delete pack                                      |
| POST   | `/api/rooms`               | session | Create room                                      |
| GET    | `/api/rooms/:code`         | session | Get room info                                    |
| POST   | `/api/assets/upload-url`   | admin   | Get pre-signed upload URL                        |
| POST   | `/api/assets/download-url` | session | Get pre-signed download URL for an item          |

### WebSocket

```plain
WS /api/ws/rooms/:code
```

All messages are JSON with a `type` field and an optional `data` object. The `type` namespace is split:

- **Lifecycle** types (`join`, `start`, `game_ended`, …) are shared across all game types
- **Game-specific** types are prefixed with the game type slug (e.g. `meme-caption:submit`, `trivia:answer`)

This allows a single WebSocket hub to serve multiple game types without protocol conflicts.

### WebSocket Security & Reliability

**Authentication**: the WS upgrade handler extracts the session cookie from the HTTP upgrade request and validates it against the `sessions` table before the connection is accepted. Unauthenticated upgrade requests are rejected with `401`.

**Message size limit**: `gorilla/websocket` `SetReadLimit` is set to **4 KB** per message. Clients exceeding this are disconnected. Game-type payloads are small (text answers, vote values) and should never approach this limit.

**Per-connection rate limit**: the hub tracks message rate per connection. Clients exceeding **20 messages/second** are disconnected. This prevents a misbehaving or compromised client from flooding the room hub.

**Reconnection**: WebSocket drops are common on mobile networks. Behaviour on disconnect:

- Player is marked `reconnecting` (not removed) for a **30-second grace window**
- Other players see a "reconnecting…" indicator rather than `player_left`
- If the player reconnects within the window, they receive a `room_state` snapshot and resume seamlessly
- If the grace window expires, `player_left` is broadcast and their turn is skipped

#### Client → Server

| Type            | When                                    |
| --------------- | --------------------------------------- |
| `join`          | Player joins lobby                      |
| `start`         | Host starts the game                    |
| `next_round`    | Host advances to next round             |
| `{slug}:submit` | Player submits answer (game-type event) |
| `{slug}:vote`   | Player casts vote (game-type event)     |

#### Server → Client (broadcast to room)

| Type                 | When                                                                                              |
| -------------------- | ------------------------------------------------------------------------------------------------- |
| `player_joined`      | A player joins the lobby                                                                          |
| `player_left`        | A player disconnects (grace window expired)                                                       |
| `game_started`       | Host starts the game                                                                              |
| `round_started`      | New round; payload includes item data **and pre-signed asset URLs** (no extra round-trips needed) |
| `submissions_closed` | Submission phase ends, voting begins                                                              |
| `vote_results`       | Round scores revealed                                                                             |
| `game_ended`         | Final leaderboard                                                                                 |
| `room_state`         | Full room snapshot sent on reconnect                                                              |

---

## 7. File Storage (Rustfs)

The storage layer is wrapped behind a `Storage` interface in `internal/storage/`. The concrete implementation uses `aws-sdk-go-v2/s3` pointed at Rustfs. Swapping to MinIO or any other S3-compatible store requires changing only the concrete implementation, not call sites.

### Access model

- All game assets are stored in a single private bucket (e.g. `fabyoumeme-assets`)
- **No public bucket access** — every read goes through a short-lived pre-signed URL
- Pre-signed download URLs: **15-minute TTL** (reduced from 1h to minimise window for shared/leaked URLs)
- Pre-signed upload URLs: generated only for authenticated admin sessions

### Object key convention

```plain
packs/{pack_id}/items/{item_id}/{original_filename}
```

### Upload flow

1. Admin requests upload URL: `POST /api/assets/upload-url { pack_id, item_id, filename, mime_type, size_bytes }`
2. Backend validates: MIME type (JPEG, PNG, WebP only) and `size_bytes ≤ 5 MB`; rejects otherwise
3. Backend generates a pre-signed PUT URL and returns it
4. Frontend PUTs the file directly to Rustfs
5. Backend stores the resulting object key in `game_items.media_key`

### Download flow — embedded in WebSocket events

Clients **do not** request download URLs individually. Instead, the backend generates pre-signed GET URLs (15-minute TTL) for all assets in the round **at the moment it broadcasts `round_started`**, and embeds them directly in the event payload. The client receives everything needed to render the round in a single server push with no extra round-trips.

`POST /api/assets/download-url` is retained only for admin preview purposes (viewing items outside a game).

---

## 8. Docker Compose

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

  rustfs:
    image: rustfs/rustfs:latest
    restart: unless-stopped
    environment:
      RUSTFS_ACCESS_KEY: ${RUSTFS_ACCESS_KEY}
      RUSTFS_SECRET_KEY: ${RUSTFS_SECRET_KEY}
    volumes:
      - rustfs_data:/data
    healthcheck:
      test: ['CMD-SHELL', 'wget -qO- http://localhost:9000/health || exit 1']
      interval: 5s
      retries: 5
    # Not exposed externally — backend connects on internal Docker network

  backend:
    build: ./backend
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      rustfs:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://fabyoumeme:${POSTGRES_PASSWORD}@postgres:5432/fabyoumeme
      RUSTFS_ENDPOINT: http://rustfs:9000
      RUSTFS_ACCESS_KEY: ${RUSTFS_ACCESS_KEY}
      RUSTFS_SECRET_KEY: ${RUSTFS_SECRET_KEY}
      RUSTFS_BUCKET: fabyoumeme-assets
      ALLOWED_ORIGIN: ${FRONTEND_URL}
      SMTP_HOST: ${SMTP_HOST}
      SMTP_PORT: ${SMTP_PORT}
      SMTP_USERNAME: ${SMTP_USERNAME}
      SMTP_PASSWORD: ${SMTP_PASSWORD}
      SMTP_FROM: ${SMTP_FROM}
      MAGIC_LINK_BASE_URL: ${FRONTEND_URL}
      MAGIC_LINK_TTL: 15m
    ports:
      - '127.0.0.1:8080:8080' # exposed only to reverse proxy

  frontend:
    build: ./frontend
    restart: unless-stopped
    depends_on:
      - backend
    environment:
      PUBLIC_API_URL: ${BACKEND_URL}
    ports:
      - '127.0.0.1:3000:3000' # exposed only to reverse proxy

volumes:
  postgres_data:
  rustfs_data:
```

`docker-compose.override.yml` adds **Mailpit** for local email catching and mounts source volumes for live reload:

```yaml
# docker-compose.override.yml (dev only, never committed with secrets)
services:
  mailpit:
    image: axllent/mailpit:latest
    ports:
      - '127.0.0.1:8025:8025' # web UI
      - '127.0.0.1:1025:1025' # SMTP

  backend:
    environment:
      SMTP_HOST: mailpit
      SMTP_PORT: '1025'
      SMTP_USERNAME: ''
      SMTP_PASSWORD: ''
      SMTP_FROM: noreply@fabyoumeme.local
    volumes:
      - ./backend:/app
```

Environment variables are loaded from a `.env` file (never committed).

### Backup Strategy

Both data volumes (`postgres_data`, `rustfs_data`) live on the host machine. Recommended minimum backup approach:

- **PostgreSQL**: nightly `pg_dump` via a cron job on the host, compressed and written to a separate directory (ideally a different physical drive or remote mount)
- **Rustfs assets**: periodic `rclone sync` of the volume mount path to an off-host location
- Retention: keep 7 daily backups; test restore procedure before first public use

This is not in scope for the Docker Compose definition but must be in place before inviting users.

---

## 9. Security Checklist

| Concern              | Mitigation                                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------------------------- |
| Credential attacks   | No passwords — magic link only; eliminates brute force and credential stuffing                             |
| Magic link prefetch  | Verify endpoint is `POST`; email link routes through intermediate frontend page before POSTing token       |
| Magic link replay    | One-time use (`used_at` set on consume); 15-minute TTL                                                     |
| Magic link leak      | Token stored as SHA-256 hash only; raw token sent exclusively by email                                     |
| Email enumeration    | Magic link endpoint always returns `200`, never reveals account existence                                  |
| Silent email change  | Notification sent to old email address on any email change, regardless of who initiated it                 |
| Session hijacking    | `HttpOnly` + `Secure` + `SameSite=Strict` cookies; 30-day TTL renewed on activity                          |
| CSRF                 | `SameSite=Strict` + origin check on WS handshake                                                           |
| WS authentication    | Session cookie validated during HTTP upgrade; unauthenticated upgrades rejected with `401`                 |
| WS message flooding  | Per-connection rate limit (20 msg/s); connections exceeding limit are dropped                              |
| WS payload bombs     | `SetReadLimit(4096)` on every connection; oversized frames disconnect the client                           |
| SQL injection        | `sqlc` parameterised queries — no string concatenation                                                     |
| File upload abuse    | Server validates MIME type and `size_bytes ≤ 5 MB` before issuing pre-signed upload URL                    |
| Asset leakage        | No public bucket; signed URLs embedded in WS events (15-min TTL); session required                         |
| Brute force          | Per-IP rate limiting on `/api/auth/*` endpoints                                                            |
| Privilege escalation | Role checked server-side in middleware on every admin route; `is_active` checked on every session lookup   |
| Secrets in repo      | All secrets via `.env`; `.env` in `.gitignore`; no signing keys needed (raw random tokens, SHA-256 stored) |
| SMTP in transit      | `go-mail` enforces STARTTLS/TLS; plaintext SMTP disallowed in production config                            |
| Internal services    | Rustfs and Postgres not reachable outside Docker internal network                                          |
| Dependency drift     | Go module checksums + `govulncheck` in CI                                                                  |
| CSP headers          | Backend sets `Content-Security-Policy` on all responses                                                    |

---

## 10. Development Workflow

```bash
# Start all services (includes Mailpit via override)
docker compose up --build

# Run DB migrations (inside backend container or locally with golang-migrate)
migrate -path ./db/migrations -database $DATABASE_URL up

# Generate sqlc types after editing queries
sqlc generate

# Frontend dev server (with HMR)
cd frontend && npm run dev

# View captured dev emails
open http://localhost:8025   # Mailpit UI
```

`docker-compose.override.yml` mounts source directories for live reload during development.

---

## 11. Open Decisions (resolve before implementing each area)

### Resolved

- [x] **Admin bootstrap**: `./server create-admin --email <addr>` CLI command. Reads `DATABASE_URL` from env, creates the first user with `role = admin`, then immediately sends them a magic link. No env-var secrets or seed migrations needed; the command is idempotent (errors if an admin already exists).
- [x] **Session TTL**: 30 days, renewed on every authenticated request.
- [x] **Magic link delivery channel**: email only in production. For local/dev installs without SMTP configured, `MAGIC_LINK_DEV_LOG=true` env var causes the backend to log the raw token to stdout instead of sending email (Mailpit handles this in the override file automatically).

### Pending

- [ ] **Room code format**: `MEME-XXXX` (readable) vs UUID short vs custom? Finished rooms' codes should be reclaimable or the pool must be large enough.
- [ ] **Round timer**: configurable per-room (default 60s) — stored in `rooms.config` JSONB.
- [ ] **Image resize/optimisation**: serve raw (simplest) or process on upload (Sharp sidecar)? Raw is fine initially; revisit if storage or bandwidth becomes a concern.
- [ ] **Solo mode data model**: current design reuses `rooms` with `mode = 'solo'`. Confirm this is sufficient before implementing solo flow.
- [ ] **Leaderboard scope**: per-room only to start; global leaderboard deferred until there is enough game history to make it meaningful.
- [ ] **Game type versioning**: when a `payload` schema evolves, how are old items handled? Options: migration script, `payload_version` field on items, or graceful degradation in handlers.
