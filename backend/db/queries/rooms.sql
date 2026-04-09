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

-- name: AddRoomPlayer :one
INSERT INTO room_players (room_id, user_id) VALUES ($1, $2) RETURNING *;

-- name: GetRoomPlayer :one
SELECT * FROM room_players WHERE room_id = $1 AND user_id = $2;

-- name: ListRoomPlayers :many
SELECT rp.*, u.username FROM room_players rp
JOIN users u ON rp.user_id = u.id
WHERE rp.room_id = $1;

-- name: RemoveRoomPlayer :exec
DELETE FROM room_players WHERE room_id = $1 AND user_id = $2;

-- name: UpdatePlayerScore :one
UPDATE room_players SET score = score + $3 WHERE room_id = $1 AND user_id = $2 RETURNING *;

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

-- name: GetSubmissionsForRound :many
SELECT * FROM submissions WHERE round_id = $1;

-- name: CreateVote :one
INSERT INTO votes (submission_id, voter_id, value) VALUES ($1, $2, $3) RETURNING *;

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
SELECT u.id AS user_id, u.username, rp.score,
       RANK() OVER (ORDER BY rp.score DESC) AS rank
FROM room_players rp
JOIN users u ON rp.user_id = u.id
WHERE rp.room_id = $1
ORDER BY rp.score DESC;
