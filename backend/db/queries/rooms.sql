-- backend/db/queries/rooms.sql

-- name: CreateRoom :one
INSERT INTO rooms (code, game_type_id, pack_id, text_pack_id, host_id, mode, config, group_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.narg(group_id))
RETURNING *;

-- name: GetRoomByCode :one
SELECT r.*, gt.slug AS game_type_slug FROM rooms r
JOIN game_types gt ON r.game_type_id = gt.id
WHERE r.code = $1;

-- name: SetRoomState :one
UPDATE rooms SET state = $2, finished_at = CASE WHEN $2 = 'finished' THEN now() ELSE NULL END
WHERE id = $1 RETURNING *;

-- name: UpdateRoomConfig :one
UPDATE rooms SET config = $2 WHERE id = $1 AND state = 'lobby' RETURNING *;

-- name: DeleteRoom :exec
-- Hard-deletes a room and everything that references it via ON DELETE CASCADE:
-- room_players, guest_players, rounds, submissions, votes. Used by the cancel
-- path (POST /rooms/{code}/end) so a cancelled room vanishes from history and
-- leaderboard as if it was never created. Naturally-finished rooms keep using
-- SetRoomState so post-game history still works.
DELETE FROM rooms WHERE id = $1;

-- name: AddRoomPlayer :one
INSERT INTO room_players (room_id, user_id) VALUES ($1, $2) RETURNING *;

-- name: UpsertRoomPlayer :exec
-- Idempotent insert used by the hub (handleRegister) and RoomHandler.Create
-- so a registered user's participation is persisted exactly once per room.
-- Relies on the partial unique index room_players_user_unique from migration
-- 004; guests are not reachable via this query because user_id is NOT NULL.
INSERT INTO room_players (room_id, user_id)
VALUES ($1, $2)
ON CONFLICT (room_id, user_id) WHERE user_id IS NOT NULL DO NOTHING;

-- name: AddGuestRoomPlayer :one
INSERT INTO room_players (room_id, guest_player_id) VALUES ($1, $2) RETURNING *;

-- name: GetRoomPlayer :one
SELECT * FROM room_players WHERE room_id = $1 AND user_id = $2;

-- name: ListRoomPlayers :many
-- Players in a room — unifies registered users and guests. For users,
-- username comes from the users table; for guests, it comes from guest_players.
-- The is_guest flag lets the frontend render a badge.
SELECT
  rp.room_id,
  rp.user_id,
  rp.guest_player_id,
  rp.score,
  rp.joined_at,
  COALESCE(u.username, gp.display_name)::text AS display_name,
  (rp.guest_player_id IS NOT NULL)::bool AS is_guest
FROM room_players rp
LEFT JOIN users u ON rp.user_id = u.id
LEFT JOIN guest_players gp ON rp.guest_player_id = gp.id
WHERE rp.room_id = $1;

-- name: RemoveRoomPlayer :exec
DELETE FROM room_players WHERE room_id = $1 AND user_id = $2;

-- name: RemoveGuestRoomPlayer :exec
DELETE FROM room_players WHERE room_id = $1 AND guest_player_id = $2;

-- name: UpdatePlayerScore :one
UPDATE room_players SET score = score + $3
 WHERE room_id = $1 AND user_id = $2
 RETURNING *;

-- name: UpdateGuestPlayerScore :one
UPDATE room_players SET score = score + $3
 WHERE room_id = $1 AND guest_player_id = $2
 RETURNING *;

-- name: CountActiveRooms :one
-- Counts rooms currently in lobby or playing state. Used by the admin dashboard
-- stats card. Finished rooms are excluded — operators care about live activity.
SELECT COUNT(*) FROM rooms WHERE state IN ('lobby', 'playing');

-- name: CountFinishedRooms :one
-- Counts rooms that reached the 'finished' state — i.e. a played-to-completion
-- game. Used by the admin dashboard "Total Games Played" card. Rooms that
-- were abandoned in lobby and auto-closed by startup cleanup are also in
-- this state, but at this scale the signal is close enough to "real games".
SELECT COUNT(*) FROM rooms WHERE state = 'finished';

-- name: FinishCrashedRooms :execresult
UPDATE rooms SET state = 'finished', finished_at = now() WHERE state = 'playing';

-- name: FinishAbandonedLobbies :execresult
UPDATE rooms SET state = 'finished', finished_at = now()
WHERE state = 'lobby' AND created_at < now() - interval '24 hours';

-- name: CreateRound :one
INSERT INTO rounds (room_id, item_id, round_number)
SELECT $1, $2, COALESCE(MAX(round_number), 0) + 1 FROM rounds WHERE room_id = $1
RETURNING *;

-- name: StartRound :one
UPDATE rounds SET started_at = now() WHERE id = $1 RETURNING *;

-- name: EndRound :one
UPDATE rounds SET ended_at = now() WHERE id = $1 RETURNING *;

-- name: GetCurrentRound :one
SELECT * FROM rounds WHERE room_id = $1 AND started_at IS NOT NULL AND ended_at IS NULL
ORDER BY round_number DESC LIMIT 1;

-- name: CreateSubmission :one
INSERT INTO submissions (round_id, user_id, payload) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateGuestSubmission :one
INSERT INTO submissions (round_id, guest_player_id, payload) VALUES ($1, $2, $3) RETURNING *;

-- name: GetSubmissionsForRound :many
SELECT * FROM submissions WHERE round_id = $1;

-- name: CreateVote :one
INSERT INTO votes (submission_id, voter_id, value) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateGuestVote :one
INSERT INTO votes (submission_id, guest_voter_id, value) VALUES ($1, $2, $3) RETURNING *;

-- name: GetVotesForRound :many
SELECT v.* FROM votes v
JOIN submissions s ON v.submission_id = s.id
WHERE s.round_id = $1;

-- name: GetVoteByVoterInRound :one
SELECT v.* FROM votes v
JOIN submissions s ON v.submission_id = s.id
WHERE s.round_id = $1 AND v.voter_id = $2;

-- name: GetRoomByID :one
SELECT * FROM rooms WHERE id = $1;

-- name: GetRoomLeaderboard :many
-- Unified leaderboard over users and guests. player_id is the UUID of whichever
-- kind of player the row represents; display_name coalesces across users.username
-- and guest_players.display_name; is_guest lets the client render a badge.
SELECT
  COALESCE(rp.user_id, rp.guest_player_id)::uuid AS player_id,
  COALESCE(u.username, gp.display_name)::text AS display_name,
  (rp.guest_player_id IS NOT NULL)::bool AS is_guest,
  rp.score,
  RANK() OVER (ORDER BY rp.score DESC) AS rank
FROM room_players rp
LEFT JOIN users u ON rp.user_id = u.id
LEFT JOIN guest_players gp ON rp.guest_player_id = gp.id
WHERE rp.room_id = $1
ORDER BY rp.score DESC;

-- name: GetActiveRoomForUser :one
-- Returns the single lobby/playing room the user is bound to, as either host
-- or participant. Returns no rows if the user is free. The two legs are
-- mutually exclusive in practice because RoomHandler.Create upserts the host
-- into room_players on creation — if both somehow match, prefer the host row
-- (listed first in the UNION).
SELECT r.id, r.code, r.state, gt.slug::text AS game_type_slug, TRUE AS is_host
FROM rooms r
JOIN game_types gt ON r.game_type_id = gt.id
WHERE r.host_id = $1 AND r.state IN ('lobby','playing')
UNION ALL
SELECT r.id, r.code, r.state, gt.slug::text AS game_type_slug, FALSE AS is_host
FROM room_players rp
JOIN rooms r ON rp.room_id = r.id
JOIN game_types gt ON r.game_type_id = gt.id
WHERE rp.user_id = $1 AND r.state IN ('lobby','playing')
LIMIT 1;

-- name: GetRecentRoomsForUser :many
-- Powers the "Recent rooms" strip on the new Home page. Scoped to the session
-- user — never take a user_id parameter from the client. Participation is
-- detected via room_players join so both hosts and non-host players qualify.
SELECT
  r.id,
  r.code,
  r.state,
  r.created_at,
  r.finished_at,
  gt.slug AS game_type_slug,
  gp.name AS pack_name
FROM room_players rp
JOIN rooms r ON rp.room_id = r.id
JOIN game_types gt ON r.game_type_id = gt.id
JOIN game_packs gp ON r.pack_id = gp.id
WHERE rp.user_id = $1
  AND r.created_at > now() - interval '30 days'
ORDER BY r.created_at DESC
LIMIT $2;

-- name: CreateUserRoomBan :exec
-- Records a registered user's ban in this room. Idempotent: a duplicate
-- kick is a no-op. Caller provides banned_by = session user ID (or admin
-- acting on the room). See docs/superpowers/specs/2026-04-18-lobby-kick-and-ban-design.md.
INSERT INTO room_bans (room_id, user_id, banned_by)
VALUES ($1, $2, $3)
ON CONFLICT (room_id, user_id) WHERE user_id IS NOT NULL DO NOTHING;

-- name: CreateGuestRoomBan :exec
-- Records a guest's ban in this room, keyed by guest_player_id. The guest's
-- existing token becomes useless for this room; a fresh guest-join with a
-- new display name mints a new guest_player_id and is not blocked
-- (accepted limitation — see spec non-goals).
INSERT INTO room_bans (room_id, guest_player_id, banned_by)
VALUES ($1, $2, $3)
ON CONFLICT (room_id, guest_player_id) WHERE guest_player_id IS NOT NULL DO NOTHING;

-- name: IsUserBannedFromRoom :one
-- Used by the WS handshake to reject a registered user's (re)join.
SELECT EXISTS(
  SELECT 1 FROM room_bans WHERE room_id = $1 AND user_id = $2
);

-- name: IsGuestBannedFromRoom :one
-- Used by the WS handshake after a guest token resolves to a guest_player_id.
SELECT EXISTS(
  SELECT 1 FROM room_bans WHERE room_id = $1 AND guest_player_id = $2
);

-- name: GetFinishedRoomByCode :one
-- Replay landing lookup. Returns public room metadata plus slug/pack names.
-- text_pack_name is the empty string when the game type has no text pack.
SELECT
  r.id,
  r.code,
  r.host_id,
  r.state,
  r.config,
  r.created_at AS started_at,
  r.finished_at,
  gt.slug::text                            AS game_type_slug,
  gp.name::text                            AS pack_name,
  COALESCE(tp.name, '')::text              AS text_pack_name,
  (SELECT COUNT(*) FROM room_players rp WHERE rp.room_id = r.id)::bigint AS player_count
FROM rooms r
JOIN game_types gt ON r.game_type_id = gt.id
JOIN game_packs gp ON r.pack_id = gp.id
LEFT JOIN game_packs tp ON r.text_pack_id = tp.id
WHERE r.code = $1 AND r.state = 'finished';

-- name: IsUserRoomMember :one
-- Authorization check for replay: returns TRUE iff this user participated in
-- the room as a registered player. Guests are never members for this purpose
-- (they have no login); admins bypass in the handler.
SELECT EXISTS(
  SELECT 1 FROM room_players
  WHERE room_id = $1 AND user_id = $2
)::bool;

-- name: GetReplayRounds :many
-- All rounds for the room, ordered by round_number. Every row in `rounds`
-- corresponds to a round the engine actually began (CreateRound is the first
-- step of the per-round loop); abandoned lobbies never populate this table.
-- Joins the prompt item's current version so the frontend gets the full
-- payload (image media_key, caption prompt, text) without a second round-trip.
SELECT
  rnd.id              AS round_id,
  rnd.round_number,
  rnd.started_at,
  rnd.ended_at,
  gi.payload_version  AS prompt_payload_version,
  giv.media_key       AS prompt_media_key,
  giv.payload         AS prompt_payload
FROM rounds rnd
JOIN game_items gi ON rnd.item_id = gi.id
JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE rnd.room_id = $1
ORDER BY rnd.round_number;

-- name: GetReplaySubmissions :many
-- Submissions for every round of the room, with author resolution across
-- users and guests. Sentinel rows (hard-deleted users) surface as
-- display_name='[deleted]', kind='deleted'. votes_received is a subquery
-- count per submission.
SELECT
  s.id                                                           AS submission_id,
  s.round_id,
  s.payload,
  COALESCE(u.username, gp.display_name, '[deleted]')::text       AS author_name,
  CASE
    WHEN u.id = '00000000-0000-0000-0000-000000000001' THEN 'deleted'
    WHEN s.guest_player_id IS NOT NULL                   THEN 'guest'
    ELSE 'user'
  END::text                                                      AS author_kind,
  (SELECT COUNT(*) FROM votes v WHERE v.submission_id = s.id)::int AS votes_received
FROM submissions s
JOIN rounds rnd ON s.round_id = rnd.id
LEFT JOIN users u         ON s.user_id = u.id
LEFT JOIN guest_players gp ON s.guest_player_id = gp.id
WHERE rnd.room_id = $1
ORDER BY rnd.round_number, s.created_at;

-- name: GetReplayLeaderboard :many
-- Final room_players leaderboard, sorted by score. Rank is assigned in Go
-- so tie-handling matches the live ResultsView (dense rank).
SELECT
  COALESCE(rp.user_id, rp.guest_player_id)::uuid AS player_id,
  COALESCE(u.username, gp.display_name, '[deleted]')::text AS display_name,
  CASE
    WHEN u.id = '00000000-0000-0000-0000-000000000001' THEN 'deleted'
    WHEN rp.guest_player_id IS NOT NULL                THEN 'guest'
    ELSE 'user'
  END::text                                      AS kind,
  rp.score
FROM room_players rp
LEFT JOIN users u         ON rp.user_id = u.id
LEFT JOIN guest_players gp ON rp.guest_player_id = gp.id
WHERE rp.room_id = $1
ORDER BY rp.score DESC, rp.joined_at ASC;
