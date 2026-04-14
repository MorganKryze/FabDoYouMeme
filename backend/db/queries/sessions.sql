-- backend/db/queries/sessions.sql

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.*, u.id AS u_id, u.username, u.email, u.role, u.is_active, u.created_at AS u_created_at
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: RenewSession :one
UPDATE sessions SET expires_at = $2 WHERE id = $1 RETURNING *;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteAllUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now();
