# 03 — Data & Assets

PostgreSQL 17 schema and RustFS file storage. These two concerns are unified here because asset lifecycle (upload, download, cleanup) is tightly coupled to the DB schema (item rows, pack soft-delete, room history).

---

## Database

All migrations live in `backend/db/migrations/` as `golang-migrate` `.sql` files (one `up` + one `down` per migration). `sqlc` generates type-safe Go from queries in `backend/db/queries/`.

### Schema

```sql
-- ─────────────────────────────────────────────
-- Users
-- username: unique human-facing handle, used in-game and as display identity.
-- email: magic link delivery channel; changeable (see pending_email in 02-identity.md).
-- consent_at: timestamp when the user accepted the privacy policy at registration.
--   Required for GDPR Art. 7 consent record. Set once; never updated.
-- Role and is_active are re-checked on every session lookup — never cached.
-- ─────────────────────────────────────────────
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username      TEXT NOT NULL UNIQUE,
  email         TEXT NOT NULL UNIQUE,
  pending_email TEXT,                            -- set when user requests email change; cleared on confirm
  role          TEXT NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
  is_active     BOOLEAN NOT NULL DEFAULT true,   -- false = deactivated by admin; cannot log in
  invited_by    UUID REFERENCES users(id),
  consent_at    TIMESTAMPTZ NOT NULL,              -- set at registration; GDPR Art. 7 consent record
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- SENTINEL ROW — seeded in migration 001, never deleted.
-- id = '00000000-0000-0000-0000-000000000001', username = '[deleted]', is_active = false.
-- Used to replace user_id on submissions when a user is hard-deleted (see ADR-006).

-- ─────────────────────────────────────────────
-- Sessions (30-day TTL; renewed on each authenticated request)
-- ─────────────────────────────────────────────
CREATE TABLE sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of the random 32-byte cookie value
  expires_at  TIMESTAMPTZ NOT NULL,   -- now() + SESSION_TTL; refreshed on each authenticated request
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Magic link tokens (login and email-change confirmation)
-- ─────────────────────────────────────────────
CREATE TABLE magic_link_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of the raw token sent by email
  purpose     TEXT NOT NULL DEFAULT 'login' CHECK (purpose IN ('login', 'email_change')),
  expires_at  TIMESTAMPTZ NOT NULL,   -- short TTL: MAGIC_LINK_TTL (default 15 minutes)
  used_at     TIMESTAMPTZ,            -- null = not yet used; set on consume
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Invites
-- ─────────────────────────────────────────────
CREATE TABLE invites (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token             TEXT NOT NULL UNIQUE,
  created_by        UUID REFERENCES users(id) ON DELETE SET NULL,  -- null when creator is hard-deleted
  label             TEXT,
  restricted_email  TEXT,              -- null = any email may use this invite
  max_uses          INT NOT NULL DEFAULT 1,   -- 0 = unlimited
  uses_count        INT NOT NULL DEFAULT 0,
  expires_at        TIMESTAMPTZ,       -- null = never
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Game type registry
-- Seeded via migration; new game types added by new migrations only.
-- config: recommended/validated config ranges for rooms using this game type.
-- supports_solo: enforced at POST /api/rooms — returns solo_mode_not_supported if false.
-- Payload version support is declared by the Go handler (SupportedPayloadVersions()),
-- not stored here. See 04-protocol.md for the handler interface.
-- ─────────────────────────────────────────────
CREATE TABLE game_types (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          TEXT NOT NULL UNIQUE,   -- e.g. 'meme-caption', 'trivia', 'drawing'
  name          TEXT NOT NULL,
  description   TEXT,
  version       TEXT NOT NULL DEFAULT '1.0.0',
  supports_solo BOOLEAN NOT NULL DEFAULT false,
  config        JSONB NOT NULL DEFAULT '{
    "min_round_duration_seconds":      15,
    "max_round_duration_seconds":      300,
    "default_round_duration_seconds":  60,
    "min_voting_duration_seconds":     10,
    "max_voting_duration_seconds":     120,
    "default_voting_duration_seconds": 30,
    "min_players":                     2,
    "max_players":                     null,
    "min_round_count":                 1,
    "max_round_count":                 50,
    "default_round_count":             10
  }',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Game packs (portable content libraries; not tied to a specific game type)
-- A pack of meme images can be reused across 'meme-caption', 'meme-vote', or any future type.
-- Soft-deleted: deleted_at IS NOT NULL. All queries must filter WHERE deleted_at IS NULL.
-- owner_id: null = official/admin pack (V1, V2, V3…); non-null = user-created pack.
-- is_official: admin-settable only; marks canonical packs. Users cannot set this.
-- visibility: private = usable only in rooms started by the owner; public = anyone can use.
-- status: active (default) | flagged (admin notified, still usable) | banned (removed from room selection).
-- ─────────────────────────────────────────────
CREATE TABLE game_packs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  description TEXT,
  owner_id    UUID REFERENCES users(id) ON DELETE SET NULL,  -- null for official packs
  is_official BOOLEAN NOT NULL DEFAULT false,
  visibility  TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
  status      TEXT NOT NULL DEFAULT 'active'  CHECK (status IN ('active', 'flagged', 'banned')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

-- ─────────────────────────────────────────────
-- Game items (generic content units)
-- payload JSONB holds all item data. Game type handlers read the fields they need.
-- A single item can serve multiple game types simultaneously.
-- payload_version tracks schema revision; handlers declare supported versions.
-- current_version_id: points to the active game_item_versions row for this item.
--   Set to NULL if the current version is purged (item still exists for historical integrity).
-- ─────────────────────────────────────────────
CREATE TABLE game_items (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  pack_id             UUID NOT NULL REFERENCES game_packs(id) ON DELETE CASCADE,
  position            INT NOT NULL DEFAULT 0,
  payload_version     INT NOT NULL DEFAULT 1,
  current_version_id  UUID,                      -- FK set after INSERT of first game_item_versions row
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (pack_id, position)
);

-- ─────────────────────────────────────────────
-- Game item versions (full history of each item's content)
-- Each save in the Studio creates a new version row; current_version_id on game_items points to the active one.
-- media_key: RustFS object key for image versions; null for text-only items.
-- payload: full JSONB snapshot of the item at this version (text content, alt text, etc.)
-- deleted_at: soft delete (moved to bin); a background job hard-purges rows where
--   deleted_at < now() - 30 days, including the corresponding RustFS object deletion
--   (logged as asset_purge before each S3 DELETE — same pattern as pack purge).
-- ─────────────────────────────────────────────
CREATE TABLE game_item_versions (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id        UUID NOT NULL REFERENCES game_items(id) ON DELETE CASCADE,
  version_number INT NOT NULL,
  media_key      TEXT,                            -- null for text-only items
  payload        JSONB NOT NULL DEFAULT '{}',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at     TIMESTAMPTZ,
  UNIQUE (item_id, version_number)
);

-- Add FK from game_items to game_item_versions (deferred to avoid circular dependency at creation time)
ALTER TABLE game_items
  ADD CONSTRAINT fk_current_version
  FOREIGN KEY (current_version_id) REFERENCES game_item_versions(id) ON DELETE SET NULL;

-- ─────────────────────────────────────────────
-- Admin notifications (content moderation inbox)
-- Queued when a user creates or modifies a public pack.
-- Pack remains live during review; admin can flag/ban via PATCH /api/packs/:id/status.
-- type: 'pack_published' (new public pack) | 'pack_modified' (existing public pack changed).
-- actor_id: the user who triggered the event; SET NULL if user is hard-deleted.
-- read_at: null = unread.
-- ─────────────────────────────────────────────
CREATE TABLE admin_notifications (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type       TEXT NOT NULL CHECK (type IN ('pack_published', 'pack_modified')),
  pack_id    UUID REFERENCES game_packs(id) ON DELETE CASCADE,
  actor_id   UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  read_at    TIMESTAMPTZ
);

-- ─────────────────────────────────────────────
-- Rooms (live game sessions)
-- code: 4 uppercase letters (e.g. "WXYZ").
--   DB UNIQUE constraint is unconditional: two live rooms can never share a code.
--   On creation, backend generates a random code and INSERTs; retries on 23505
--   (unique violation) up to 10 times. See ADR-007 in ref-decisions.md.
-- config shape validated by CHECK constraint below.
-- ─────────────────────────────────────────────
CREATE TABLE rooms (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code         TEXT NOT NULL UNIQUE,
  game_type_id UUID NOT NULL REFERENCES game_types(id),
  pack_id      UUID NOT NULL REFERENCES game_packs(id),
  host_id      UUID NOT NULL REFERENCES users(id),
  mode         TEXT NOT NULL DEFAULT 'multiplayer' CHECK (mode IN ('multiplayer', 'solo')),
  state        TEXT NOT NULL DEFAULT 'lobby' CHECK (state IN ('lobby', 'playing', 'finished')),
  config       JSONB NOT NULL DEFAULT '{"round_duration_seconds": 60, "voting_duration_seconds": 30, "round_count": 10}',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at  TIMESTAMPTZ,   -- set when state → 'finished'

  CONSTRAINT config_valid CHECK (
    (config->>'round_duration_seconds')::int BETWEEN 15 AND 300 AND
    (config->>'voting_duration_seconds')::int BETWEEN 10 AND 120 AND
    (config->>'round_count')::int BETWEEN 1 AND 50
  )
);

CREATE TABLE room_players (
  room_id   UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  score     INT NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (room_id, user_id)
);

-- ─────────────────────────────────────────────
-- Rounds
-- round_number is always assigned server-side:
--   SELECT COALESCE(MAX(round_number), 0) + 1 FROM rounds WHERE room_id = ?
--   inside a transaction to prevent duplicates.
-- ─────────────────────────────────────────────
CREATE TABLE rounds (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id      UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  item_id      UUID NOT NULL REFERENCES game_items(id),
  round_number INT NOT NULL,
  started_at   TIMESTAMPTZ,
  ended_at     TIMESTAMPTZ,
  UNIQUE (room_id, round_number),

  CONSTRAINT timeline_valid CHECK (
    started_at IS NULL OR ended_at IS NULL OR started_at < ended_at
  )
);

-- ─────────────────────────────────────────────
-- Submissions
-- user_id: NOT NULL. Hard-deleted users are replaced by the sentinel UUID
-- '00000000-0000-0000-0000-000000000001' before the user row is deleted.
-- See ADR-006 in ref-decisions.md.
-- payload JSONB matches game_items.payload convention — flexible for all game types.
-- ─────────────────────────────────────────────
CREATE TABLE submissions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  round_id   UUID NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id),
  payload    JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (round_id, user_id)
);

-- ─────────────────────────────────────────────
-- Votes
-- value JSONB supports varied scoring models:
--   simple upvote: {"points": 1}
--   ranked choice: {"rank": 2}
--   star rating:   {"stars": 4}
-- Self-vote prevention enforced at application layer (see 04-protocol.md).
-- voter_id: NOT NULL. Hard-deleted users are replaced by the sentinel UUID
-- '00000000-0000-0000-0000-000000000001' before the user row is deleted (same as submissions).
-- ─────────────────────────────────────────────
CREATE TABLE votes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  voter_id      UUID NOT NULL REFERENCES users(id),
  value         JSONB NOT NULL DEFAULT '{"points": 1}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (submission_id, voter_id)
);

-- ─────────────────────────────────────────────
-- Audit log
-- Tracks admin actions (user changes, invite revocations, pack deletions, user hard-deletes).
-- resource format: "{table}:{id}", e.g. "user:uuid", "invite:uuid"
-- admin_id: SET NULL if the admin who performed the action is later hard-deleted.
--   Use LEFT JOIN when joining to users on admin_id to avoid silently dropping rows.
-- ─────────────────────────────────────────────
CREATE TABLE audit_logs (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id   UUID REFERENCES users(id) ON DELETE SET NULL,  -- null if acting admin is later deleted
  action     TEXT NOT NULL,   -- e.g. 'update_user_role', 'revoke_invite', 'delete_pack', 'hard_delete_user'
  resource   TEXT NOT NULL,   -- e.g. 'user:abc-123'
  changes    JSONB NOT NULL,  -- e.g. {"role": ["player", "admin"]}; for hard_delete_user: {"username": "...", "email": "..."}
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

### Indexes

```sql
-- Foreign key indexes (PostgreSQL does not auto-index FK columns)
CREATE INDEX ON sessions(user_id);
CREATE INDEX ON magic_link_tokens(user_id);
CREATE INDEX ON room_players(user_id);
CREATE INDEX ON rounds(room_id);
CREATE INDEX ON game_items(pack_id);
CREATE INDEX ON submissions(round_id);
CREATE INDEX ON votes(submission_id);
CREATE INDEX ON votes(voter_id);
CREATE INDEX ON audit_logs(admin_id, created_at DESC);
CREATE INDEX ON audit_logs(resource);

-- Composite: "did this user already submit this round?" (hot path during submission)
CREATE INDEX ON submissions(round_id, user_id);

-- Partial: active-pack list (most common admin query)
CREATE INDEX ON game_packs(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON game_packs(owner_id, created_at DESC) WHERE deleted_at IS NULL;

-- game_item_versions: version lookup + bin cleanup
CREATE INDEX ON game_item_versions(item_id, version_number);
CREATE INDEX ON game_item_versions(deleted_at) WHERE deleted_at IS NOT NULL;

-- admin_notifications: unread inbox query
CREATE INDEX ON admin_notifications(read_at) WHERE read_at IS NULL;
CREATE INDEX ON admin_notifications(created_at DESC);

-- Partial: find the active round in a room (used before every WS event)
CREATE INDEX ON rounds(room_id, round_number DESC) WHERE started_at IS NOT NULL;

-- Cleanup job support: expire old tokens and sessions efficiently
CREATE INDEX ON magic_link_tokens(expires_at) WHERE used_at IS NULL;
CREATE INDEX ON sessions(expires_at);

-- Admin user search: case-insensitive substring match on GET /api/admin/users?q=
CREATE INDEX ON users (lower(username) text_pattern_ops);
CREATE INDEX ON users (lower(email) text_pattern_ops);
```

---

### Atomic Invite Use

The `uses_count` increment must be atomic to prevent two simultaneous registrations from both passing the capacity check:

```sql
-- Use UPDATE ... RETURNING instead of SELECT + UPDATE
UPDATE invites
SET uses_count = uses_count + 1
WHERE id = $1
  AND (max_uses = 0 OR uses_count < max_uses)
  AND (expires_at IS NULL OR expires_at > now())
RETURNING *;
```

If zero rows are returned, the invite is full or expired — return `400 invalid_invite`.

---

### Cleanup Strategy

Run nightly (host cron or backend scheduler):

```sql
-- Delete expired, unused magic link tokens
DELETE FROM magic_link_tokens
WHERE expires_at < now() AND used_at IS NULL;

-- Delete expired sessions
DELETE FROM sessions WHERE expires_at < now();

-- Delete used magic link tokens older than 7 days (keep recent ones for audit)
DELETE FROM magic_link_tokens
WHERE used_at IS NOT NULL AND used_at < now() - interval '7 days';
```

**Game data retention**: rooms, rounds, submissions, and votes are retained for **2 years** after `rooms.finished_at`. A background job running monthly soft-purges old rooms:

```sql
-- Rooms finished more than 2 years ago; cascades to room_players, rounds, submissions, votes
DELETE FROM rooms
WHERE finished_at < now() - interval '2 years';
```

Rationale: 2 years balances player interest in historical scores against the storage limitation principle (Art. 5(1)(e) GDPR). Adjust the window via a future env var if needed.

**Audit log anonymization**: a background job running annually anonymizes `hard_delete_user` entries older than 3 years by replacing the PII fields with SHA-256 hashes. This satisfies Art. 5(1)(e) (storage limitation) while preserving audit integrity under Art. 17(3)(b) (legal obligation exception):

```sql
UPDATE audit_logs
SET changes = jsonb_set(
  jsonb_set(
    changes,
    '{username}', to_jsonb('hash:' || encode(digest(changes->>'username', 'sha256'), 'hex'))
  ),
  '{email}', to_jsonb('hash:' || encode(digest(changes->>'email', 'sha256'), 'hex'))
)
WHERE action = 'hard_delete_user'
  AND created_at < now() - interval '3 years'
  AND changes ? 'email';   -- skip already-anonymized rows
```

**Version bin purge**: a background job hard-purges `game_item_versions` rows where `deleted_at < now() - interval '30 days'`. For each row with a non-null `media_key`, the RustFS object is deleted first (same `asset_purge` / `asset_purge_failed` log pattern as pack purge below). The DB row is deleted after the S3 call regardless of outcome — the structured log enables recovery of any orphaned object references.

**Asset cleanup audit log**: when an admin purges orphaned RustFS objects from a soft-deleted pack, each deletion MUST be preceded by a structured log entry:

```json
{
  "msg": "asset_purge",
  "level": "INFO",
  "media_key": "packs/...",
  "pack_id": "uuid",
  "admin_id": "uuid"
}
```

If the S3 DELETE fails, log `asset_purge_failed` with the same fields plus `"error"`. The audit record exists regardless of whether the deletion succeeded, enabling recovery of orphaned references.

**Startup cleanup log events**: on startup, the two SQL cleanup statements (see `06-operations.md`) emit structured log events:

- `room.crash_recovery` — rooms moved from `playing` to `finished` (includes `count`)
- `room.abandoned` — rooms moved from `lobby` to `finished` after 24h (includes `count`)

---

### Item Position Reordering

The `UNIQUE (pack_id, position)` constraint blocks naive in-place swaps. The chosen strategy (ADR-010) is a two-pass update:

1. Shift all items in the pack to `position + 10000` in one `UPDATE`
2. Set final positions as specified

Both passes run in a single transaction. See `04-protocol.md` for the API contract and `ref-error-codes.md` for `positions_invalid`.

---

### Pack–Game Type Compatibility

Packs are game-type-agnostic. Compatibility is checked dynamically at room creation (`POST /api/rooms`) — no junction table required (see ADR-008 in `ref-decisions.md`).

```sql
-- Count usable items in the pack for the given game type
SELECT COUNT(*) FROM game_items
WHERE pack_id = $pack_id
  AND payload_version = ANY($supported_versions::int[])
  AND pack_id IN (SELECT id FROM game_packs WHERE deleted_at IS NULL);
```

Error responses (see `ref-error-codes.md`):

- **Zero compatible items**: `422 pack_no_supported_items` — pack has no items at a payload version the handler supports at all
- **Some but insufficient**: `422 pack_insufficient_items` — compatible items exist but fewer than `config.round_count`

`POST /api/rooms` additionally rejects `mode='solo'` when `game_types.supports_solo = false` with `422 solo_mode_not_supported`.

**Pack dropdown filtering**: `GET /api/packs?game_type_id={uuid}` filters the list to packs with ≥1 item whose `payload_version` is in the handler's supported set. The `supported_payload_versions` list is exposed via `GET /api/game-types/:slug` (see `04-protocol.md`). The frontend uses this filter to prevent admins from selecting incompatible packs before hitting the room-creation endpoint.

---

### Game Item Payload Convention

Packs and items are game-type-agnostic. The same pack can serve multiple game types simultaneously. Each handler reads the `payload` fields it needs and ignores the rest.

```jsonc
// Works for 'meme-caption' (uses "prompt") and 'meme-vote' (uses media_key only)
{ "prompt": "When the CI passes on the first try" }

// Text-only trivia item (no media_key needed)
{ "question": "What year was Go released?", "answers": ["2007", "2009", "2012", "2015"], "correct": 1 }
```

**Payload versioning**: when a game type's payload schema evolves, increment `payload_version` on new items. Handlers declare which versions they support via `SupportedPayloadVersions()` and branch on `payload_version` to parse correctly. Old items keep their version — no data migration, no data loss. See `04-protocol.md` for the handler interface.

---

## File Storage

> **External dependency**: RustFS is deployed in a separate Docker Compose stack and is not managed by this project. The backend connects to it over the shared `pangolin` Docker network.

### RustFS Setup

Before starting this stack, deploy RustFS in its own Compose file attached to the `pangolin` external network:

```yaml
# rustfs/docker-compose.yml (separate stack — not in this repo)
services:
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
    expose:
      - 9000
    networks:
      - pangolin

volumes:
  rustfs_data:
networks:
  pangolin:
    external: true
```

Before starting this stack:

1. Create the `fabyoumeme-assets` bucket
2. Create credentials (`RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`) and note them for this project's `.env`

Once RustFS is running on `pangolin`, the backend resolves it by the container name `rustfs`.

### Storage Interface

The storage layer is wrapped behind a `Storage` interface in `internal/storage/`. The concrete implementation uses `aws-sdk-go-v2/s3` pointed at RustFS. Swapping to MinIO or any other S3-compatible store requires changing only the concrete implementation — call sites are unaffected.

### Access Model

- All game assets stored in a single **private** bucket (`fabyoumeme-assets`)
- **No public bucket access** — every read goes through a short-lived pre-signed URL
- Pre-signed download URLs: **15-minute TTL**
- Pre-signed upload URLs: issued to authenticated pack owners (for their own packs) and admins

### Object Key Convention

```plain
packs/{pack_id}/items/{item_id}/v{version_number}/{original_filename}
```

The version number is included so multiple versions of the same item can coexist in storage without key collision. Purging a specific version deletes only that versioned key, leaving other versions intact.

### Upload Flow

```plain
1. POST /api/packs/:id/items
     Body: { payload, payload_version }    ← no image yet
     Response: { item_id, ... }

2. POST /api/assets/upload-url
     Body: { pack_id, item_id, filename, mime_type, size_bytes }
     Server validates:
       - mime_type ∈ { image/jpeg, image/png, image/webp }
       - magic bytes match the declared mime_type (Go image.DecodeConfig)
       - size_bytes ≤ MAX_UPLOAD_SIZE_BYTES (default 2 MB)
     Response: { upload_url, media_key }

3. Frontend PUT {file} to upload_url directly (client → RustFS, bypasses backend)

4. PATCH /api/packs/:id/items/:item_id
     Body: { media_key }
     Server stores media_key on the item record — upload confirmed
```

The item must exist before the upload URL is requested (step 1 before step 2). The backend has no S3 webhook; step 4 is the explicit frontend confirmation.

**MIME validation**: the server first checks `mime_type` against the allowlist, then reads the first ~512 bytes with `image.DecodeConfig` to validate magic bytes. `Content-Type` header alone is insufficient — an attacker can send any header value with any file content.

**Frontend preview**: before confirming upload, the frontend renders `<img src={URL.createObjectURL(file)}>` so the admin can verify the correct file was selected.

### Download Flow — Embedded in WebSocket Events

Clients **do not** request download URLs individually. When the backend broadcasts `round_started`, it generates pre-signed GET URLs (15-minute TTL) for all assets in the round and embeds them in the event payload. No extra round-trips.

`POST /api/assets/download-url` is retained only for **admin preview** (viewing items outside a live game).

**Pre-signed URL policy**: generated with `response-content-disposition=attachment`. This forces the browser to treat the resource as a download rather than rendering it inline. For in-game rendering, the frontend loads the asset into `<img>` via an object URL from a `fetch()` response — the image is never directly navigable.

### Asset Lifecycle

When a game pack is soft-deleted (`deleted_at` set), its items remain in the DB for historical game data integrity. The corresponding RustFS objects are **not** deleted automatically. If storage reclamation is needed, the admin triggers a purge operation:

1. Query `game_items WHERE pack_id = $id AND media_key IS NOT NULL` for all items of the soft-deleted pack
2. For each `media_key`, log `asset_purge` BEFORE issuing the S3 DELETE (see Cleanup Strategy above)
3. Issue `DeleteObject` to RustFS
4. Log `asset_upload_confirmed` on success or `asset_purge_failed` on error

**Constraint**: purge must only run on packs where `deleted_at IS NOT NULL`. The backend enforces this check before initiating any purge. The item rows remain in the DB for historical round integrity — only the RustFS objects are removed. Pre-signed URLs embedded in historical `round_started` events will 404 after purge; this is expected and acceptable for purged content.
