-- backend/db/queries/rooms.sql

-- name: CreateRoom :one
INSERT INTO rooms (code, game_type_id, pack_id, host_id, mode, config)
VALUES ($1, $2, $3, $4, $5, $6)
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

-- name: SetRematchWindow :one
-- Stamps rematch_window_expires_at when a room transitions finished → (rematchable).
-- Called by finishRoom() so the client's EndStage can show a countdown banner.
UPDATE rooms SET rematch_window_expires_at = $2 WHERE id = $1 RETURNING *;

-- name: ResurrectRoom :one
-- Transitions a finished room back to lobby iff the rematch window is still open
-- and the caller is the host. Returns the updated row, or no rows if the gate fails.
UPDATE rooms
   SET state = 'lobby',
       finished_at = NULL,
       rematch_window_expires_at = NULL
 WHERE id = $1
   AND host_id = $2
   AND state = 'finished'
   AND rematch_window_expires_at IS NOT NULL
   AND rematch_window_expires_at > now()
RETURNING *;

-- name: AddRoomPlayer :one
INSERT INTO room_players (room_id, user_id) VALUES ($1, $2) RETURNING *;

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

-- name: ResetRoomPlayerScores :exec
-- Zeroes out scores for every player in the room. Used by the rematch flow
-- (B2) so a resurrected room starts cleanly without wiping participation rows.
UPDATE room_players SET score = 0 WHERE room_id = $1;

-- name: CountActiveRooms :one
-- Counts rooms currently in lobby or playing state. Used by the admin dashboard
-- stats card. Finished rooms are excluded — operators care about live activity.
SELECT COUNT(*) FROM rooms WHERE state IN ('lobby', 'playing');

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
