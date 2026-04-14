-- backend/db/queries/invites.sql

-- name: CreateInvite :one
INSERT INTO invites (token, created_by, label, restricted_email, max_uses, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInviteByToken :one
SELECT * FROM invites WHERE token = $1;

-- name: ConsumeInvite :one
UPDATE invites SET uses_count = uses_count + 1
WHERE id = $1
  AND (max_uses = 0 OR uses_count < max_uses)
  AND (expires_at IS NULL OR expires_at > now())
RETURNING *;

-- name: DeleteInvite :exec
DELETE FROM invites WHERE id = $1;

-- name: ListInvites :many
SELECT * FROM invites ORDER BY created_at DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountInvites :one
SELECT COUNT(*) FROM invites;

-- name: CountPendingInvites :one
-- An invite is "pending" iff it still has remaining uses and is not expired.
-- max_uses = 0 means unlimited. Used by the admin dashboard stats card.
SELECT COUNT(*) FROM invites
WHERE (max_uses = 0 OR uses_count < max_uses)
  AND (expires_at IS NULL OR expires_at > now());
