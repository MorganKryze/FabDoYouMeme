# Architecture

## System overview

```plain
                     ┌─────────────────────────────────────────┐
                     │          Docker Compose host             │
                     │                                          │
  Browser ──────────►│  Reverse proxy (pre-existing)           │
                     │    /api/* ──► backend :8080              │
                     │    /*     ──► frontend :3000             │
                     │                                          │
                     │  ┌──────────┐   ┌──────────────────┐    │
                     │  │ frontend │   │     backend       │    │
                     │  │SvelteKit │   │  Go + chi router  │    │
                     │  └──────────┘   └────────┬─────────┘    │
                     │                          │               │
                     │                 ┌────────┴────────┐      │
                     │                 │   PostgreSQL 17  │      │
                     │                 └─────────────────┘      │
                     └─────────────────────────────────────────┘
                                        │
                     ┌──────────────────▼──────────────────┐
                     │  Pangolin network (external stack)   │
                     │  RustFS — S3-compatible file storage │
                     └─────────────────────────────────────┘
```

The reverse proxy is assumed to exist and is not managed by this project. The backend and PostgreSQL are internal-only — not reachable from outside the Docker network. The frontend joins both the internal network and the external `pangolin` network so the reverse proxy can reach it.

---

## Backend

The backend is a single Go binary. `cmd/server/main.go` wires all components together at startup: config loading, DB connection, service initialisation, game handler registration, and route mounting.

### Internal packages

| Package       | Responsibility                                                                                                                                  |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `config/`     | Loads and validates all environment variables at startup; fails fast if required vars are absent                                                |
| `middleware/` | Request ID injection, structured logging, authentication session lookup, rate limiting, admin guard, IP allowlist for metrics endpoint          |
| `auth/`       | Registration, magic link generation/verification, session creation/deletion, profile management, admin user/invite management, GDPR hard-delete |
| `game/`       | Game type registry, WebSocket hub per room, round lifecycle state machine                                                                       |
| `api/`        | REST handlers for rooms, packs, items, versions, assets                                                                                         |
| `storage/`    | Interface-backed RustFS/S3 client; pre-signed URL generation                                                                                    |
| `email/`      | Template rendering and SMTP delivery for magic links and notifications                                                                          |

### Database layer

**Migrations** live in `backend/db/migrations/` as `golang-migrate` up/down SQL pairs. They run automatically at startup.

**Queries** live in `backend/db/queries/` as `.sql` files. `sqlc` generates type-safe Go structs and query functions in `backend/db/sqlc/`. No Go database code is written by hand — all DB access goes through the generated layer.

Current migrations:

- `001_initial_schema` — full table definitions, sentinel user row
- `002_seed_game_types` — seeds the `meme-caption` game type row
- `003_schema_fixes` — FK constraint corrections, performance indexes

### Startup behaviour

On every start the backend:

1. Applies any pending DB migrations
2. Checks for `SEED_ADMIN_EMAIL` and creates the admin user if no admin exists
3. Marks any rooms with `state = playing` as `finished` (crash recovery — rooms can't be resumed)
4. Closes any `lobby` rooms older than 24 hours

All four operations are idempotent.

---

## Frontend

The frontend is a SvelteKit application compiled with `adapter-node` to a Node.js server. It performs SSR for initial page loads and hydrates to a reactive client-side app.

### Route structure

Routes are divided into layout groups:

| Group      | Path prefix                         | Auth required                         |
| ---------- | ----------------------------------- | ------------------------------------- |
| `(public)` | `/auth/*`                           | No                                    |
| `(app)`    | `/` (lobby, rooms, profile, studio) | Yes — redirects to `/auth/magic-link` |
| `(admin)`  | `/admin/*`                          | Yes + admin role                      |

### State management

Global state is held in three Svelte 5 reactive singleton classes in `lib/state/`:

| Class       | File             | What it holds                                                                    |
| ----------- | ---------------- | -------------------------------------------------------------------------------- |
| `UserState` | `user.svelte.ts` | Authenticated user (`id`, `username`, `email`, `role`)                           |
| `WsState`   | `ws.svelte.ts`   | WebSocket connection, status (`connected` / `reconnecting` / `error` / `closed`) |
| `RoomState` | `room.svelte.ts` | Current room state, player list, round phase, scores                             |

Components import singleton instances directly and read reactive properties with Svelte's `$derived` rune. No subscription boilerplate.

### API client

Typed fetch wrappers for every REST endpoint live in `lib/api/`. They share error handling and session cookie passing. Server-side loads use the same wrappers via SvelteKit's `fetch` (which automatically includes cookies during SSR).

### Game plugins

Each game type has a self-contained UI plugin in `lib/games/<slug>/`. The meme-caption plugin exports four components: `SubmitForm`, `VoteForm`, `ResultsView`, and `GameRules`. The room page dynamically loads the plugin matching the room's `game_type_slug`. Adding a new game type requires adding a new directory here — no changes to the room shell.

---

## Storage

Files (game item images) are stored in RustFS, an S3-compatible self-hosted object store. The backend never proxies file bytes. The flow:

1. Admin requests a pre-signed upload URL from the backend (`POST /api/assets/upload-url`), passing the MIME type and first bytes of the file for magic byte validation.
2. The backend validates MIME type (allowlist: JPEG, PNG, WebP) and magic bytes, then issues a pre-signed S3 PUT URL directly to RustFS.
3. The client uploads directly to RustFS using the pre-signed URL — no file bytes pass through the Go server.
4. The client creates a version record (`POST /api/packs/:id/items/:itemId/versions`) to save the storage key and confirm the upload.

Pre-signed download URLs are embedded in WebSocket events during game rounds. They expire after 15 minutes and require no session — the time-limited URL is the access control.

---

## Email

The email layer uses Go's `html/template` for rendering paired HTML + plain-text templates. All templates live in `internal/email/templates/`. SMTP delivery uses `go-mail` with STARTTLS enforced.

Three email types are sent:

| Trigger                      | Template                     | Purpose                                           |
| ---------------------------- | ---------------------------- | ------------------------------------------------- |
| Registration / login request | `magic_link_login`           | Delivers the one-time login link                  |
| Email change request         | `magic_link_email_change`    | Delivers the confirmation link to the new address |
| Email change confirmed       | `notification_email_changed` | Notifies the old address that the change happened |

---

## Middleware stack

Every request passes through, in order:

1. **Request ID** — injects a unique `X-Request-ID` header and stores it in context; included in all error responses
2. **Structured logger** — logs method, path, status, duration, and request ID as JSON
3. **Rate limiter** — enforces per-IP limits (varies by route group); entries evicted after 1 hour of inactivity
4. **Auth** (`RequireAuth`) — reads session cookie, looks up hash in `sessions` table, re-fetches user `role` and `is_active` from DB; rejects if absent, expired, or user is deactivated
5. **Admin guard** (`RequireAdmin`) — checks `role = 'admin'`; applied on top of `RequireAuth` for admin routes
6. **IP allowlist** — applied only to `GET /api/metrics`; rejects any IP not in `METRICS_ALLOWED_IPS`

---

## Database schema

Full annotated schema. All migrations live in `backend/db/migrations/`. All queries are generated by `sqlc` from `backend/db/queries/` — no Go DB code is written by hand.

### users

Holds all registered accounts. `role` and `is_active` are re-checked on every session lookup — never cached. `consent_at` is set once at registration and never updated (GDPR Art. 7 record).

| Column          | Type          | Notes                                                   |
| --------------- | ------------- | ------------------------------------------------------- |
| `id`            | UUID PK       |                                                         |
| `username`      | TEXT UNIQUE   | Human-facing display handle                             |
| `email`         | TEXT UNIQUE   | Magic link delivery channel                             |
| `pending_email` | TEXT nullable | Set when user requests email change; cleared on confirm |
| `role`          | TEXT          | `'player'` or `'admin'`                                 |
| `is_active`     | BOOLEAN       | `false` = deactivated by admin; cannot log in           |
| `invited_by`    | UUID → users  | `ON DELETE SET NULL`                                    |
| `consent_at`    | TIMESTAMPTZ   | Timestamp of registration consent                       |
| `created_at`    | TIMESTAMPTZ   |                                                         |

A sentinel row (`id = 00000000-0000-0000-0000-000000000001`, `username = '[deleted]'`, `is_active = false`) is seeded in migration 001 and must never be deleted. It is used to replace `user_id` on submissions and votes when a user is hard-deleted. See `docs/reference/decisions.md` ADR-006.

### sessions

DB-backed opaque sessions. Token stored as SHA-256 hash. Renewed on each authenticated request; renewed in the background by the WS hub for long-running connections.

| Column       | Type         | Notes                                            |
| ------------ | ------------ | ------------------------------------------------ |
| `id`         | UUID PK      |                                                  |
| `user_id`    | UUID → users | `ON DELETE CASCADE`                              |
| `token_hash` | TEXT UNIQUE  | SHA-256 of the random 32-byte cookie value       |
| `expires_at` | TIMESTAMPTZ  | `now() + SESSION_TTL`; refreshed on each request |
| `created_at` | TIMESTAMPTZ  |                                                  |

### magic_link_tokens

One-time login and email-change confirmation tokens. Only SHA-256 hash stored; raw token sent by email only.

| Column       | Type                 | Notes                                     |
| ------------ | -------------------- | ----------------------------------------- |
| `id`         | UUID PK              |                                           |
| `user_id`    | UUID → users         | `ON DELETE CASCADE`                       |
| `token_hash` | TEXT UNIQUE          | SHA-256 of the raw token                  |
| `purpose`    | TEXT                 | `'login'` or `'email_change'`             |
| `expires_at` | TIMESTAMPTZ          | `now() + MAGIC_LINK_TTL` (default 15 min) |
| `used_at`    | TIMESTAMPTZ nullable | Set on consume; null = not yet used       |
| `created_at` | TIMESTAMPTZ          |                                           |

### invites

Admin-created invite tokens for registration.

| Column             | Type                 | Notes                                             |
| ------------------ | -------------------- | ------------------------------------------------- |
| `id`               | UUID PK              |                                                   |
| `token`            | TEXT UNIQUE          | 12-char alphanumeric                              |
| `created_by`       | UUID → users         | `ON DELETE SET NULL`                              |
| `label`            | TEXT nullable        | Admin note                                        |
| `restricted_email` | TEXT nullable        | If set, only this address may register            |
| `max_uses`         | INT                  | `0` = unlimited                                   |
| `uses_count`       | INT                  | Atomically incremented on successful registration |
| `expires_at`       | TIMESTAMPTZ nullable | null = never expires                              |
| `created_at`       | TIMESTAMPTZ          |                                                   |

### game_types

Registry of available game types. Seeded by migrations; new types require a new migration.

| Column          | Type          | Notes                                                                 |
| --------------- | ------------- | --------------------------------------------------------------------- |
| `id`            | UUID PK       |                                                                       |
| `slug`          | TEXT UNIQUE   | e.g. `'meme-caption'`                                                 |
| `name`          | TEXT          | Display name                                                          |
| `description`   | TEXT nullable |                                                                       |
| `version`       | TEXT          | Semantic version                                                      |
| `supports_solo` | BOOLEAN       | Enforced at room creation                                             |
| `config`        | JSONB         | Min/max/default values for round durations, player count, round count |
| `created_at`    | TIMESTAMPTZ   |                                                                       |

### game_packs

Content libraries. Not tied to a specific game type — a pack of meme images can be used by any game type that supports its payload version. Soft-deleted: all queries filter `WHERE deleted_at IS NULL`.

| Column        | Type                  | Notes                                            |
| ------------- | --------------------- | ------------------------------------------------ |
| `id`          | UUID PK               |                                                  |
| `name`        | TEXT                  |                                                  |
| `description` | TEXT nullable         |                                                  |
| `owner_id`    | UUID → users nullable | `ON DELETE SET NULL`; null = official/admin pack |
| `is_official` | BOOLEAN               | Admin-settable only                              |
| `visibility`  | TEXT                  | `'private'` or `'public'`                        |
| `status`      | TEXT                  | `'active'`, `'flagged'`, or `'banned'`           |
| `created_at`  | TIMESTAMPTZ           |                                                  |
| `deleted_at`  | TIMESTAMPTZ nullable  | Soft-delete                                      |

### game_items

Generic content units within a pack. `payload` JSONB holds all item data; structure is defined per game type. Multiple game types can use the same item simultaneously.

| Column               | Type                               | Notes                                                |
| -------------------- | ---------------------------------- | ---------------------------------------------------- |
| `id`                 | UUID PK                            |                                                      |
| `pack_id`            | UUID → game_packs                  | `ON DELETE CASCADE`                                  |
| `position`           | INT                                | UNIQUE with `pack_id`; used for display ordering     |
| `payload_version`    | INT                                | Handlers declare supported versions                  |
| `current_version_id` | UUID → game_item_versions nullable | `ON DELETE SET NULL`; null if current version purged |
| `created_at`         | TIMESTAMPTZ                        |                                                      |

### game_item_versions

Full version history for each item. Each Studio save creates a new row; `game_items.current_version_id` points to the active one. `deleted_at` is a 30-day soft-delete bin; a background job hard-purges binned versions including their RustFS objects.

| Column           | Type                 | Notes                                       |
| ---------------- | -------------------- | ------------------------------------------- |
| `id`             | UUID PK              |                                             |
| `item_id`        | UUID → game_items    | `ON DELETE CASCADE`                         |
| `version_number` | INT                  | UNIQUE with `item_id`                       |
| `media_key`      | TEXT nullable        | RustFS object key; null for text-only items |
| `payload`        | JSONB                | Full item snapshot at this version          |
| `created_at`     | TIMESTAMPTZ          |                                             |
| `deleted_at`     | TIMESTAMPTZ nullable | Bin; hard-purged after 30 days              |

### rooms

Live game sessions. `code` is a 4-letter unique identifier generated with `crypto/rand`, retried on collision (ADR-007). Room `config` is validated by a CHECK constraint.

| Column         | Type                  | Notes                                                              |
| -------------- | --------------------- | ------------------------------------------------------------------ |
| `id`           | UUID PK               |                                                                    |
| `code`         | TEXT UNIQUE           | 4 uppercase letters                                                |
| `game_type_id` | UUID → game_types     |                                                                    |
| `pack_id`      | UUID → game_packs     |                                                                    |
| `host_id`      | UUID → users nullable | `ON DELETE SET NULL`; null if host was hard-deleted                |
| `mode`         | TEXT                  | `'multiplayer'` or `'solo'`                                        |
| `state`        | TEXT                  | `'lobby'`, `'playing'`, or `'finished'`                            |
| `config`       | JSONB                 | `{ round_duration_seconds, voting_duration_seconds, round_count }` |
| `created_at`   | TIMESTAMPTZ           |                                                                    |
| `finished_at`  | TIMESTAMPTZ nullable  | Set when state → `'finished'`                                      |

### room_players

Tracks which users are in which room and their cumulative score.

| Column      | Type         | Notes                               |
| ----------- | ------------ | ----------------------------------- |
| `room_id`   | UUID → rooms | `ON DELETE CASCADE`; part of PK     |
| `user_id`   | UUID → users | `ON DELETE CASCADE`; part of PK     |
| `score`     | INT          | Cumulative points across all rounds |
| `joined_at` | TIMESTAMPTZ  |                                     |

### rounds

One row per round within a room. `round_number` is assigned server-side inside a transaction to prevent duplicates.

| Column         | Type                 | Notes                             |
| -------------- | -------------------- | --------------------------------- |
| `id`           | UUID PK              |                                   |
| `room_id`      | UUID → rooms         | `ON DELETE CASCADE`               |
| `item_id`      | UUID → game_items    | The content item shown this round |
| `round_number` | INT                  | UNIQUE with `room_id`             |
| `started_at`   | TIMESTAMPTZ nullable |                                   |
| `ended_at`     | TIMESTAMPTZ nullable |                                   |

### submissions

Player answers for a round. `user_id` is NOT NULL — hard-deleted users are replaced by the sentinel UUID before row deletion. `payload` structure is defined per game type.

| Column       | Type          | Notes                                            |
| ------------ | ------------- | ------------------------------------------------ |
| `id`         | UUID PK       |                                                  |
| `round_id`   | UUID → rounds | `ON DELETE CASCADE`                              |
| `user_id`    | UUID → users  | NOT NULL; uses sentinel on hard-delete (ADR-006) |
| `payload`    | JSONB         | Game-type-specific content                       |
| `created_at` | TIMESTAMPTZ   |                                                  |

One submission per player per round enforced by UNIQUE `(round_id, user_id)`.

### votes

Player votes on submissions. `value` JSONB supports varied scoring models. Self-vote prevention enforced at the application layer.

| Column          | Type               | Notes                                               |
| --------------- | ------------------ | --------------------------------------------------- |
| `id`            | UUID PK            |                                                     |
| `submission_id` | UUID → submissions | `ON DELETE CASCADE`                                 |
| `voter_id`      | UUID → users       | NOT NULL; uses sentinel on hard-delete (ADR-006)    |
| `value`         | JSONB              | e.g. `{"points": 1}`, `{"rank": 2}`, `{"stars": 4}` |
| `created_at`    | TIMESTAMPTZ        |                                                     |

One vote per player per submission enforced by UNIQUE `(submission_id, voter_id)`.

### audit_logs

Tracks admin actions on users, invites, and packs. `admin_id` is SET NULL if the acting admin is later hard-deleted (use LEFT JOIN when querying). For `hard_delete_user` actions, `changes` captures the deleted user's PII before deletion — anonymised after 3 years.

| Column       | Type                  | Notes                                           |
| ------------ | --------------------- | ----------------------------------------------- |
| `id`         | UUID PK               |                                                 |
| `admin_id`   | UUID → users nullable | `ON DELETE SET NULL`                            |
| `action`     | TEXT                  | e.g. `'update_user_role'`, `'hard_delete_user'` |
| `resource`   | TEXT                  | e.g. `'user:abc-123'`                           |
| `changes`    | JSONB                 | e.g. `{"role": ["player", "admin"]}`            |
| `created_at` | TIMESTAMPTZ           |                                                 |

### admin_notifications

Content moderation inbox. Queued when a user creates or modifies a public pack. Pack remains live during review.

| Column       | Type                  | Notes                                   |
| ------------ | --------------------- | --------------------------------------- |
| `id`         | UUID PK               |                                         |
| `type`       | TEXT                  | `'pack_published'` or `'pack_modified'` |
| `pack_id`    | UUID → game_packs     | `ON DELETE CASCADE`                     |
| `actor_id`   | UUID → users nullable | `ON DELETE SET NULL`                    |
| `created_at` | TIMESTAMPTZ           |                                         |
| `read_at`    | TIMESTAMPTZ nullable  | null = unread                           |

---

### Data retention and cleanup

Nightly background jobs clean up expired tokens and sessions:

- Expired unused magic link tokens — deleted immediately
- Expired sessions — deleted immediately
- Used magic link tokens older than 7 days — deleted

Monthly job:

- Rooms finished more than 2 years ago — deleted (cascades to `room_players`, `rounds`, `submissions`, `votes`)

Annual job:

- `audit_logs` `hard_delete_user` entries older than 3 years — PII fields replaced with SHA-256 hashes

30-day bin job:

- `game_item_versions` with `deleted_at` older than 30 days — RustFS object deleted first, then DB row removed (logged as `asset_purge`)
