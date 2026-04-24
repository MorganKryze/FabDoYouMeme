-- backend/db/queries/group_invites.sql

-- name: CreateGroupInvite :one
INSERT INTO group_invites (token, group_id, created_by, kind, restricted_email, max_uses, expires_at)
VALUES ($1, $2, $3, $4, sqlc.narg(restricted_email), $5, sqlc.narg(expires_at))
RETURNING *;

-- name: GetGroupInviteByToken :one
SELECT * FROM group_invites WHERE token = $1;

-- name: ListGroupInvites :many
-- Admin-facing list of every invite ever minted for the group, newest first.
-- Includes consumed and revoked codes so the audit trail is visible; the UI
-- filters as it likes.
SELECT * FROM group_invites WHERE group_id = $1 ORDER BY created_at DESC;

-- name: RevokeGroupInvite :one
-- Idempotent revoke: stamps revoked_at the first time, leaves it alone on
-- subsequent calls. Returns the row so the handler can confirm the (gid,
-- inviteID) pair actually matched something.
UPDATE group_invites
SET revoked_at = COALESCE(revoked_at, now())
WHERE id = $1 AND group_id = $2
RETURNING *;

-- name: ConsumeGroupInvite :one
-- Atomic redemption check + bump. Mirrors invites.ConsumeInvite. The full set
-- of preconditions (group not soft-deleted, redeemer not banned, redeemer
-- under membership cap) is enforced by the handler because they need
-- separate query roundtrips and per-precondition error codes.
UPDATE group_invites SET uses_count = uses_count + 1
WHERE id = $1
  AND revoked_at IS NULL
  AND uses_count < max_uses
  AND (expires_at IS NULL OR expires_at > now())
RETURNING *;

-- name: CountActiveGroupInvitesForCreator :one
-- Rate-limit input: how many "active" codes (unrevoked, unexpired, not yet
-- exhausted) the actor currently has for the given group.
SELECT count(*) FROM group_invites
WHERE created_by = $1 AND group_id = $2
  AND revoked_at IS NULL
  AND uses_count < max_uses
  AND (expires_at IS NULL OR expires_at > now());

-- name: CountGroupInvitesMintedSince :one
-- Rate-limit input: how many codes (any state) this actor has minted in the
-- given group since the cutoff. Drives the per-hour mint cap.
SELECT count(*) FROM group_invites
WHERE created_by = $1 AND group_id = $2 AND created_at > $3;

-- name: RevokeAllInvitesByCreator :exec
-- Used by the platform-ban cascade — every still-active code minted by the
-- banned user is revoked. Idempotent.
UPDATE group_invites
SET revoked_at = COALESCE(revoked_at, now())
WHERE created_by = $1 AND revoked_at IS NULL;
