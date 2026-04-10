-- backend/db/queries/users.sql

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CreateUser :one
INSERT INTO users (username, email, role, is_active, invited_by, consent_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateUserUsername :one
UPDATE users SET username = $2 WHERE id = $1 RETURNING *;

-- name: SetPendingEmail :one
UPDATE users SET pending_email = $2 WHERE id = $1 RETURNING *;

-- name: ConfirmEmailChange :one
UPDATE users SET email = pending_email, pending_email = NULL WHERE id = $1 RETURNING *;

-- name: UpdateUserRole :one
UPDATE users SET role = $2 WHERE id = $1 RETURNING *;

-- name: SetUserActive :one
UPDATE users SET is_active = $2 WHERE id = $1 RETURNING *;

-- name: UpdateUserEmailAdmin :one
UPDATE users SET email = $2 WHERE id = $1 RETURNING *;

-- name: ListUsers :many
SELECT * FROM users
WHERE lower(username) LIKE lower('%' || sqlc.arg(search) || '%')
   OR lower(email)    LIKE lower('%' || sqlc.arg(search) || '%')
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE lower(username) LIKE lower('%' || sqlc.arg(search) || '%')
   OR lower(email)    LIKE lower('%' || sqlc.arg(search) || '%');

-- Sentinel UUID: 00000000-0000-0000-0000-000000000001 (see auth.SentinelUserID in Go).
-- Used to anonymize submissions/votes on hard-delete without breaking referential integrity.

-- name: GetSentinelUser :one
SELECT * FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- name: HardDeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateSubmissionsSentinel :exec
UPDATE submissions SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id = $1;

-- name: UpdateVotesSentinel :exec
UPDATE votes SET voter_id = '00000000-0000-0000-0000-000000000001' WHERE voter_id = $1;

-- name: GetUserGameHistory :many
SELECT
  r.code,
  gt.slug AS game_type_slug,
  gp.name AS pack_name,
  r.created_at AS started_at,
  r.finished_at,
  rp.score,
  r.id AS room_id,
  ((SELECT COUNT(*) FROM room_players rp2 WHERE rp2.room_id = r.id AND rp2.score > rp.score) + 1)::bigint AS rank,
  (SELECT COUNT(*) FROM room_players rp3 WHERE rp3.room_id = r.id) AS player_count
FROM room_players rp
JOIN rooms r ON rp.room_id = r.id
JOIN game_types gt ON r.game_type_id = gt.id
JOIN game_packs gp ON r.pack_id = gp.id
WHERE rp.user_id = $1 AND r.state = 'finished'
ORDER BY r.finished_at DESC NULLS LAST
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: GetUserSubmissions :many
SELECT
  s.id,
  s.round_id,
  r2.code AS room_code,
  gt.slug AS game_type_slug,
  s.payload,
  s.created_at
FROM submissions s
JOIN rounds rnd ON s.round_id = rnd.id
JOIN rooms r2 ON rnd.room_id = r2.id
JOIN game_types gt ON r2.game_type_id = gt.id
WHERE s.user_id = $1
  AND r2.state = 'finished'
ORDER BY s.created_at DESC;
