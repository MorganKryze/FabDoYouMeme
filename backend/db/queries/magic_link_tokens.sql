-- backend/db/queries/magic_link_tokens.sql

-- name: CreateMagicLinkToken :one
INSERT INTO magic_link_tokens (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetMagicLinkToken :one
SELECT * FROM magic_link_tokens
WHERE token_hash = $1 AND expires_at > now() AND used_at IS NULL;

-- name: ConsumeMagicLinkToken :one
UPDATE magic_link_tokens SET used_at = now() WHERE id = $1 RETURNING *;

-- name: InvalidatePendingTokens :exec
UPDATE magic_link_tokens SET used_at = now()
WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL;

-- name: DeleteExpiredUnusedTokens :exec
DELETE FROM magic_link_tokens WHERE expires_at < now() AND used_at IS NULL;

-- name: DeleteOldUsedTokens :exec
DELETE FROM magic_link_tokens
WHERE used_at IS NOT NULL AND used_at < now() - interval '7 days';
