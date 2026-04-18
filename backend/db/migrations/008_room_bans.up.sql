-- backend/db/migrations/008_room_bans.up.sql
--
-- Per-room ban list. Populated by the kick handler so a removed player
-- cannot rejoin the same room. Polymorphic like room_players: a ban row
-- holds EITHER a user_id (registered user) OR a guest_player_id (guest).
-- Bans are room-scoped and die with the room (ON DELETE CASCADE), matching
-- the host-room-creator model — a host's new room is a clean slate.

CREATE TABLE room_bans (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
  guest_player_id UUID REFERENCES guest_players(id) ON DELETE CASCADE,
  banned_by       UUID REFERENCES users(id) ON DELETE SET NULL,
  banned_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT room_bans_identity_xor
    CHECK (num_nonnulls(user_id, guest_player_id) = 1)
);

-- Partial unique indexes (not a single composite) mirror the
-- room_players_user_unique / room_players_guest_unique pattern from
-- migration 004 so idempotent re-kick is an ON CONFLICT no-op per kind.
CREATE UNIQUE INDEX room_bans_user_unique
  ON room_bans (room_id, user_id) WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX room_bans_guest_unique
  ON room_bans (room_id, guest_player_id) WHERE guest_player_id IS NOT NULL;

CREATE INDEX room_bans_room_idx ON room_bans (room_id);
