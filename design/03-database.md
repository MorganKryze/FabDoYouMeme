# 03 — Database Schema

PostgreSQL 17. All migrations live in `backend/db/migrations/` as `golang-migrate` `.sql` files (one `up` + one `down` per migration). `sqlc` generates type-safe Go from queries in `backend/db/queries/`.

---

## Schema

```sql
-- ─────────────────────────────────────────────
-- Users
-- username: unique human-facing handle, used in-game and as display identity.
-- email: magic link delivery channel; changeable (see pending_email).
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
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Sessions (30-day TTL; renewed on each authenticated request)
-- ─────────────────────────────────────────────
CREATE TABLE sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,   -- SHA-256 of the random 32-byte cookie value
  expires_at  TIMESTAMPTZ NOT NULL,   -- now() + 30 days; refreshed on each authenticated request
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
  expires_at  TIMESTAMPTZ NOT NULL,   -- short TTL: 15 minutes
  used_at     TIMESTAMPTZ,            -- null = not yet used
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Invites
-- ─────────────────────────────────────────────
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

-- ─────────────────────────────────────────────
-- Game type registry
-- Seeded via migration; new game types added by new migrations only.
-- config: recommended/validated config ranges for rooms using this game type.
-- ─────────────────────────────────────────────
CREATE TABLE game_types (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          TEXT NOT NULL UNIQUE,   -- e.g. 'meme-caption', 'trivia', 'drawing'
  name          TEXT NOT NULL,
  description   TEXT,
  version       TEXT NOT NULL DEFAULT '1.0.0',
  supports_solo BOOLEAN NOT NULL DEFAULT false,
  config        JSONB NOT NULL DEFAULT '{
    "min_round_duration_seconds":     15,
    "max_round_duration_seconds":     300,
    "default_round_duration_seconds": 60,
    "min_voting_duration_seconds":    10,
    "max_voting_duration_seconds":    120,
    "default_voting_duration_seconds": 30,
    "min_players":                    2,
    "max_players":                    null,
    "min_round_count":                1,
    "max_round_count":                50,
    "default_round_count":            10
  }',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─────────────────────────────────────────────
-- Game packs (portable content libraries; not tied to a specific game type)
-- A pack of meme images can be reused across 'meme-caption', 'meme-vote', or any future type.
-- Soft-deleted: deleted_at IS NOT NULL. All queries must filter WHERE deleted_at IS NULL.
-- ─────────────────────────────────────────────
CREATE TABLE game_packs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  description TEXT,
  created_by  UUID NOT NULL REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

-- ─────────────────────────────────────────────
-- Game items (generic content units)
-- payload JSONB holds all item data. Game type handlers read the fields they need.
-- A single item can serve multiple game types simultaneously.
-- payload_version tracks schema revision; handlers declare supported versions.
-- ─────────────────────────────────────────────
CREATE TABLE game_items (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  pack_id         UUID NOT NULL REFERENCES game_packs(id) ON DELETE CASCADE,
  position        INT NOT NULL DEFAULT 0,
  media_key       TEXT,                          -- Rustfs object key; nullable if text-only
  payload         JSONB NOT NULL DEFAULT '{}',
  payload_version INT NOT NULL DEFAULT 1,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (pack_id, position)
);

-- ─────────────────────────────────────────────
-- Rooms (live game sessions)
-- code: 4 uppercase letters (e.g. "WXYZ").
--   On creation, backend picks a random code and retries if any row matches:
--   WHERE code = ? AND (state != 'finished' OR finished_at > now() - interval '24 hours')
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
  finished_at  TIMESTAMPTZ,   -- set when state → 'finished'; code re-usable 24h after

  CONSTRAINT config_valid CHECK (
    (config->>'round_duration_seconds')::int BETWEEN 15 AND 300 AND
    (config->>'voting_duration_seconds')::int BETWEEN 10 AND 120 AND
    (config->>'round_count')::int BETWEEN 1 AND 50
  )
);

CREATE TABLE room_players (
  room_id   UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  user_id   UUID NOT NULL REFERENCES users(id),
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
-- payload JSONB matches game_items.payload convention — flexible for all game types.
-- e.g. text answer: {"text": "..."}, image pick: {"item_id": "..."}
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
-- Self-vote prevention is enforced at the application layer (see 04-api.md).
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
-- Tracks admin actions (user changes, invite revocations, pack deletions).
-- resource format: "{table}:{id}", e.g. "user:uuid", "invite:uuid"
-- ─────────────────────────────────────────────
CREATE TABLE audit_logs (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id   UUID NOT NULL REFERENCES users(id),
  action     TEXT NOT NULL,   -- e.g. 'update_user_role', 'revoke_invite', 'delete_pack'
  resource   TEXT NOT NULL,   -- e.g. 'user:abc-123'
  changes    JSONB NOT NULL,  -- e.g. {"role": ["player", "admin"]}
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## Indexes

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
CREATE INDEX ON game_packs(created_by) WHERE deleted_at IS NULL;

-- Partial: find the active round in a room (used before every WS event)
CREATE INDEX ON rounds(room_id, round_number DESC) WHERE started_at IS NOT NULL;

-- Cleanup job support: expire old tokens and sessions efficiently
CREATE INDEX ON magic_link_tokens(expires_at) WHERE used_at IS NULL;
CREATE INDEX ON sessions(expires_at);
```

---

## Atomic Invite Use

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

If zero rows are returned, the invite is full or expired — return 409.

---

## Cleanup Strategy

Expired tokens and sessions accumulate indefinitely without a cleanup job. Run nightly (e.g., cron on the host or a scheduled task in the backend):

```sql
-- Delete expired, unused magic link tokens
DELETE FROM magic_link_tokens
WHERE expires_at < now() AND used_at IS NULL;

-- Delete expired sessions
DELETE FROM sessions WHERE expires_at < now();

-- Delete used magic link tokens older than 7 days (keep recent ones for audit purposes)
DELETE FROM magic_link_tokens
WHERE used_at IS NOT NULL AND used_at < now() - interval '7 days';
```

---

## Item Position Reordering

The `UNIQUE (pack_id, position)` constraint blocks naive in-place swaps because the intermediate state violates uniqueness. Two safe approaches:

**Option A — Two-pass update (chosen)**: shift all positions to a large temporary range (`position + 10000`), then set final values. Both steps in one transaction. No schema changes needed.

**Option B — Deferred constraint**: alter the constraint to be deferrable:

```sql
ALTER TABLE game_items DROP CONSTRAINT game_items_pack_id_position_key;
ALTER TABLE game_items ADD CONSTRAINT game_items_pack_id_position_key
  UNIQUE (pack_id, position) DEFERRABLE INITIALLY DEFERRED;
```

With a deferred constraint the uniqueness check runs at `COMMIT` rather than per-row, so arbitrary in-place updates work inside a transaction. The trade-off is slightly higher per-transaction overhead. Option A is simpler and equally correct for this use case.

---

## Pack–Game Type Compatibility

Packs are game-type-agnostic by design, but a pack must have at least as many usable items as the room's `round_count` for the chosen game type. "Usable" means the handler's `SupportedPayloadVersions` list includes `item.payload_version`.

This validation happens at room creation (`POST /api/rooms`):

```sql
-- Count usable items in the pack for the given game type
SELECT COUNT(*) FROM game_items
WHERE pack_id = $pack_id
  AND payload_version = ANY($supported_versions::int[]);
```

If the count is less than `config.round_count`, return `422 {"code":"pack_insufficient_items", "error":"Pack does not have enough items for the requested round count"}`.

No junction table is required — compatibility is determined dynamically at room creation time by the handler's declared supported versions. Adding a new game type does not require tagging existing packs.

---

## Game Item Payload Convention

Packs and items are **game-type-agnostic**. The room configuration is where a game type and pack are paired — the same pack of meme images can be used for `meme-caption`, `meme-vote`, or any future type without duplicating content.

Each game type's backend handler reads the `payload` fields it needs and ignores the rest. Admins populate items with whichever fields are relevant; a well-authored item can serve multiple game types simultaneously.

```jsonc
// Works for 'meme-caption' (uses "prompt") and 'meme-vote' (uses media_key only)
{ "prompt": "When the CI passes on the first try" }

// Text-only trivia item (no media_key needed)
{ "question": "What year was Go released?", "answers": ["2007", "2009", "2012", "2015"], "correct": 1 }
```

**Payload versioning**: when a game type's payload schema evolves, increment `payload_version` on new items. Handlers declare which versions they support and branch on `payload_version` to parse correctly. Old items keep their version — no data migration, no data loss. See [06-game-engine.md](06-game-engine.md) for the handler interface.

**Solo mode**: gated by `game_types.supports_solo`. Game types where solo play is meaningless (e.g., pure peer-voting) set `supports_solo = false`; the UI hides the solo option for those types.
