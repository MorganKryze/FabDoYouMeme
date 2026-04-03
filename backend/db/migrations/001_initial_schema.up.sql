-- backend/db/migrations/001_initial_schema.up.sql

-- Users
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  username      TEXT NOT NULL UNIQUE,
  email         TEXT NOT NULL UNIQUE,
  pending_email TEXT,
  role          TEXT NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
  is_active     BOOLEAN NOT NULL DEFAULT true,
  invited_by    UUID REFERENCES users(id),
  consent_at    TIMESTAMPTZ NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sentinel row for hard-deleted users (ADR-006). Never delete this row.
INSERT INTO users (id, username, email, role, is_active, consent_at, created_at)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  '[deleted]',
  'deleted@localhost',
  'player',
  false,
  now(),
  now()
);

-- Sessions
CREATE TABLE sessions (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  expires_at  TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Magic link tokens
CREATE TABLE magic_link_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  purpose     TEXT NOT NULL DEFAULT 'login' CHECK (purpose IN ('login', 'email_change')),
  expires_at  TIMESTAMPTZ NOT NULL,
  used_at     TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Invites
CREATE TABLE invites (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token            TEXT NOT NULL UNIQUE,
  created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
  label            TEXT,
  restricted_email TEXT,
  max_uses         INT NOT NULL DEFAULT 1,
  uses_count       INT NOT NULL DEFAULT 0,
  expires_at       TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Game types (seeded in migration 002)
CREATE TABLE game_types (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug          TEXT NOT NULL UNIQUE,
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

-- Game packs
CREATE TABLE game_packs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT NOT NULL,
  description TEXT,
  owner_id    UUID REFERENCES users(id) ON DELETE SET NULL,
  is_official BOOLEAN NOT NULL DEFAULT false,
  visibility  TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
  status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'flagged', 'banned')),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

-- Game items
CREATE TABLE game_items (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  pack_id            UUID NOT NULL REFERENCES game_packs(id) ON DELETE CASCADE,
  position           INT NOT NULL DEFAULT 0,
  payload_version    INT NOT NULL DEFAULT 1,
  current_version_id UUID,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (pack_id, position)
);

-- Game item versions
CREATE TABLE game_item_versions (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  item_id        UUID NOT NULL REFERENCES game_items(id) ON DELETE CASCADE,
  version_number INT NOT NULL,
  media_key      TEXT,
  payload        JSONB NOT NULL DEFAULT '{}',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at     TIMESTAMPTZ,
  UNIQUE (item_id, version_number)
);

-- Deferred FK to avoid circular dependency at creation time
ALTER TABLE game_items
  ADD CONSTRAINT fk_current_version
  FOREIGN KEY (current_version_id) REFERENCES game_item_versions(id) ON DELETE SET NULL;

-- Admin notifications
CREATE TABLE admin_notifications (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  type       TEXT NOT NULL CHECK (type IN ('pack_published', 'pack_modified')),
  pack_id    UUID REFERENCES game_packs(id) ON DELETE CASCADE,
  actor_id   UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  read_at    TIMESTAMPTZ
);

-- Rooms
CREATE TABLE rooms (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code         TEXT NOT NULL UNIQUE,
  game_type_id UUID NOT NULL REFERENCES game_types(id),
  pack_id      UUID NOT NULL REFERENCES game_packs(id),
  host_id      UUID NOT NULL REFERENCES users(id),
  mode         TEXT NOT NULL DEFAULT 'multiplayer' CHECK (mode IN ('multiplayer', 'solo')),
  state        TEXT NOT NULL DEFAULT 'lobby' CHECK (state IN ('lobby', 'playing', 'finished')),
  config       JSONB NOT NULL DEFAULT '{"round_duration_seconds":60,"voting_duration_seconds":30,"round_count":10}',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at  TIMESTAMPTZ,
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

-- Rounds
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

-- Submissions
CREATE TABLE submissions (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  round_id   UUID NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
  user_id    UUID NOT NULL REFERENCES users(id),
  payload    JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (round_id, user_id)
);

-- Votes
CREATE TABLE votes (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  voter_id      UUID NOT NULL REFERENCES users(id),
  value         JSONB NOT NULL DEFAULT '{"points":1}',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (submission_id, voter_id)
);

-- Audit log
CREATE TABLE audit_logs (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  admin_id   UUID REFERENCES users(id) ON DELETE SET NULL,
  action     TEXT NOT NULL,
  resource   TEXT NOT NULL,
  changes    JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Indexes ─────────────────────────────────────────────

CREATE INDEX ON sessions(user_id);
CREATE INDEX ON sessions(expires_at);
CREATE INDEX ON magic_link_tokens(user_id);
CREATE INDEX ON magic_link_tokens(expires_at) WHERE used_at IS NULL;
CREATE INDEX ON room_players(user_id);
CREATE INDEX ON rounds(room_id);
CREATE INDEX ON rounds(room_id, round_number DESC) WHERE started_at IS NOT NULL;
CREATE INDEX ON game_items(pack_id);
CREATE INDEX ON game_item_versions(item_id, version_number);
CREATE INDEX ON game_item_versions(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX ON submissions(round_id);
CREATE INDEX ON submissions(round_id, user_id);
CREATE INDEX ON votes(submission_id);
CREATE INDEX ON votes(voter_id);
CREATE INDEX ON audit_logs(admin_id, created_at DESC);
CREATE INDEX ON audit_logs(resource);
CREATE INDEX ON admin_notifications(read_at) WHERE read_at IS NULL;
CREATE INDEX ON admin_notifications(created_at DESC);
CREATE INDEX ON game_packs(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON game_packs(owner_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON users (lower(username) text_pattern_ops);
CREATE INDEX ON users (lower(email) text_pattern_ops);
