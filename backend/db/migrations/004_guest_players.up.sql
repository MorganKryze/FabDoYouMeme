-- backend/db/migrations/004_guest_players.up.sql
--
-- Guest players: ephemeral, per-room participants who join without an account.
-- Unlocks the pre-auth join flow (someone shares a code → friend joins without
-- registering). Guests are NOT users — they have no email, no consent_at, and
-- cascade away when the room is deleted.
--
-- Also adds rooms.rematch_window_expires_at for the B2 room-resurrect feature.

-- ── guest_players ─────────────────────────────────────────────────────────
CREATE TABLE guest_players (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id       UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  display_name  TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 32),
  token_hash    TEXT NOT NULL UNIQUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (room_id, display_name)
);

CREATE INDEX ON guest_players(room_id);

-- ── room_players: accept either a user or a guest ─────────────────────────
-- Drop the composite PK (room_id, user_id) so user_id can become nullable.
-- Uniqueness is preserved via two partial unique indexes: at most one row
-- per (room_id, user_id) for users, at most one per (room_id, guest_player_id)
-- for guests. A CHECK constraint enforces that exactly one of the two is set.
ALTER TABLE room_players
  DROP CONSTRAINT room_players_pkey,
  ALTER COLUMN user_id DROP NOT NULL,
  ADD COLUMN guest_player_id UUID REFERENCES guest_players(id) ON DELETE CASCADE,
  ADD CONSTRAINT room_players_identity_xor
    CHECK ((user_id IS NOT NULL) <> (guest_player_id IS NOT NULL));

CREATE UNIQUE INDEX room_players_user_unique
  ON room_players (room_id, user_id)
  WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX room_players_guest_unique
  ON room_players (room_id, guest_player_id)
  WHERE guest_player_id IS NOT NULL;

-- ── submissions: accept either a user or a guest as author ────────────────
ALTER TABLE submissions
  ALTER COLUMN user_id DROP NOT NULL,
  ADD COLUMN guest_player_id UUID REFERENCES guest_players(id) ON DELETE SET NULL,
  ADD CONSTRAINT submissions_author_xor
    CHECK ((user_id IS NOT NULL) <> (guest_player_id IS NOT NULL));

-- Original UNIQUE (round_id, user_id) doesn't cover guests. Replace with
-- two partial unique indexes so a guest and a user can both submit, but
-- no player (of either kind) can submit twice.
ALTER TABLE submissions DROP CONSTRAINT submissions_round_id_user_id_key;

CREATE UNIQUE INDEX submissions_round_user_unique
  ON submissions (round_id, user_id)
  WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX submissions_round_guest_unique
  ON submissions (round_id, guest_player_id)
  WHERE guest_player_id IS NOT NULL;

-- ── votes: accept either a user or a guest as voter ───────────────────────
ALTER TABLE votes
  ALTER COLUMN voter_id DROP NOT NULL,
  ADD COLUMN guest_voter_id UUID REFERENCES guest_players(id) ON DELETE SET NULL,
  ADD CONSTRAINT votes_voter_xor
    CHECK ((voter_id IS NOT NULL) <> (guest_voter_id IS NOT NULL));

ALTER TABLE votes DROP CONSTRAINT votes_submission_id_voter_id_key;

CREATE UNIQUE INDEX votes_submission_voter_unique
  ON votes (submission_id, voter_id)
  WHERE voter_id IS NOT NULL;

CREATE UNIQUE INDEX votes_submission_guest_voter_unique
  ON votes (submission_id, guest_voter_id)
  WHERE guest_voter_id IS NOT NULL;

-- ── rooms: rematch window for B2 (server-side resurrect) ──────────────────
-- NULL when the room has never finished or the rematch window has closed.
-- Set to now() + rematch window (5 minutes) when finishRoom() runs.
ALTER TABLE rooms
  ADD COLUMN rematch_window_expires_at TIMESTAMPTZ;
