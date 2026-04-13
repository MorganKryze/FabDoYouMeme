-- backend/db/queries/guest_players.sql

-- name: CreateGuestPlayer :one
INSERT INTO guest_players (room_id, display_name, token_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetGuestPlayerByTokenHash :one
SELECT * FROM guest_players WHERE token_hash = $1;

-- name: GetGuestPlayerByID :one
SELECT * FROM guest_players WHERE id = $1;

-- name: TouchGuestPlayer :exec
UPDATE guest_players SET last_seen_at = now() WHERE id = $1;

-- name: DeleteGuestPlayersForRoom :exec
DELETE FROM guest_players WHERE room_id = $1;
