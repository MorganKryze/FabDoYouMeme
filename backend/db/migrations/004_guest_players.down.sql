-- backend/db/migrations/004_guest_players.down.sql
--
-- Reverses 004_guest_players.up.sql. Fails if any guest rows exist — that is
-- expected. Guard callers: run `down` only on empty DBs in tests.

ALTER TABLE rooms DROP COLUMN rematch_window_expires_at;

DROP INDEX IF EXISTS votes_submission_guest_voter_unique;
DROP INDEX IF EXISTS votes_submission_voter_unique;
ALTER TABLE votes
  DROP CONSTRAINT votes_voter_xor,
  DROP COLUMN guest_voter_id,
  ALTER COLUMN voter_id SET NOT NULL,
  ADD CONSTRAINT votes_submission_id_voter_id_key UNIQUE (submission_id, voter_id);

DROP INDEX IF EXISTS submissions_round_guest_unique;
DROP INDEX IF EXISTS submissions_round_user_unique;
ALTER TABLE submissions
  DROP CONSTRAINT submissions_author_xor,
  DROP COLUMN guest_player_id,
  ALTER COLUMN user_id SET NOT NULL,
  ADD CONSTRAINT submissions_round_id_user_id_key UNIQUE (round_id, user_id);

DROP INDEX IF EXISTS room_players_guest_unique;
DROP INDEX IF EXISTS room_players_user_unique;
ALTER TABLE room_players
  DROP CONSTRAINT room_players_identity_xor,
  DROP COLUMN guest_player_id,
  ALTER COLUMN user_id SET NOT NULL,
  ADD PRIMARY KEY (room_id, user_id);

DROP TABLE guest_players;
