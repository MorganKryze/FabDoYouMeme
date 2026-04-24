-- backend/db/queries/groups.sql

-- name: CreateGroup :one
INSERT INTO groups (name, description, language, classification, quota_bytes, created_by, avatar_media_key, member_cap)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(avatar_media_key), $7)
RETURNING *;

-- name: GetGroupByID :one
SELECT * FROM groups WHERE id = $1 AND deleted_at IS NULL;

-- name: GetGroupByIDIncludingDeleted :one
-- Used by the soft-delete restore flow; returns the row even when deleted_at
-- is set so the handler can decide whether the 30-day window has elapsed.
SELECT * FROM groups WHERE id = $1;

-- name: GetGroupByNormalizedName :one
SELECT * FROM groups WHERE name_normalized = $1 AND deleted_at IS NULL;

-- name: ListGroupsForUser :many
SELECT g.*, gm.role AS member_role
FROM groups g
JOIN group_memberships gm ON gm.group_id = g.id
WHERE gm.user_id = $1 AND g.deleted_at IS NULL
ORDER BY g.created_at DESC;

-- name: ListAllGroupsAdmin :many
-- Platform admin view: every live and soft-deleted group with a current
-- member count. Pagination via lim/off.
SELECT g.*, (SELECT count(*) FROM group_memberships gm WHERE gm.group_id = g.id)::bigint AS member_count
FROM groups g
ORDER BY g.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: UpdateGroup :one
-- COALESCE-with-narg lets the handler pass nil for fields the caller did not
-- send. The avatar column is special: a nil narg means "don't touch", but the
-- caller sometimes wants to clear the avatar — represented by avatar_set=true
-- with a NULL avatar_media_key.
UPDATE groups
SET name             = COALESCE(sqlc.narg(name), name),
    description      = COALESCE(sqlc.narg(description), description),
    language         = COALESCE(sqlc.narg(language), language),
    classification   = COALESCE(sqlc.narg(classification), classification),
    avatar_media_key = CASE WHEN sqlc.arg(avatar_set)::boolean THEN sqlc.narg(avatar_media_key) ELSE avatar_media_key END
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteGroup :exec
UPDATE groups SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreGroup :exec
-- Clears deleted_at if the soft-delete is still inside the 30-day window.
-- A row outside the window is left untouched; the handler verifies via a
-- subsequent GetGroupByID and surfaces 410 Gone when restore was a no-op.
UPDATE groups SET deleted_at = NULL
WHERE id = $1 AND deleted_at IS NOT NULL AND deleted_at > now() - interval '30 days';

-- name: CountGroupsForUser :one
-- How many live groups the given user is currently a member of (any role).
SELECT count(*) FROM group_memberships gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.user_id = $1 AND g.deleted_at IS NULL;

-- name: CountCreatedGroupsForUser :one
-- How many live groups the given user originally created. Used by the
-- per-user create cap (MAX_GROUPS_PER_USER).
SELECT count(*) FROM groups
WHERE created_by = $1 AND deleted_at IS NULL;

-- name: SetGroupQuotaBytes :exec
-- Platform-admin override of the per-group asset cap. Members and group
-- admins see the current value but cannot mutate it.
UPDATE groups SET quota_bytes = $2 WHERE id = $1 AND deleted_at IS NULL;

-- name: SetGroupMemberCap :exec
-- Platform-admin override of the per-group member cap.
UPDATE groups SET member_cap = $2 WHERE id = $1 AND deleted_at IS NULL;
