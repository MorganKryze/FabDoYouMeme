-- backend/db/queries/group_memberships.sql

-- name: CreateMembership :one
INSERT INTO group_memberships (group_id, user_id, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM group_memberships WHERE group_id = $1 AND user_id = $2;

-- name: ListGroupMembers :many
-- Admins always sort first so the UI doesn't need a second pass to surface
-- moderators. last_login_at is exposed so phase-2 auto-promotion / dormancy
-- views can render without a second join.
SELECT gm.*, u.username, u.last_login_at
FROM group_memberships gm
JOIN users u ON u.id = gm.user_id
WHERE gm.group_id = $1
ORDER BY (gm.role = 'admin') DESC, gm.joined_at ASC;

-- name: CountGroupMembers :one
SELECT count(*) FROM group_memberships WHERE group_id = $1;

-- name: CountGroupAdmins :one
-- Used by the last-admin guard on Leave and SelfDemote.
SELECT count(*) FROM group_memberships WHERE group_id = $1 AND role = 'admin';

-- name: UpdateMembershipRole :one
UPDATE group_memberships
SET role = $3
WHERE group_id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMembership :exec
DELETE FROM group_memberships WHERE group_id = $1 AND user_id = $2;

-- name: CreateBan :exec
-- ON CONFLICT no-op so a Ban call after a previous Ban is idempotent (the
-- handler may double-fire from the network layer; second call must succeed).
INSERT INTO group_bans (group_id, user_id, banned_by)
VALUES ($1, $2, $3)
ON CONFLICT (group_id, user_id) DO NOTHING;

-- name: DeleteBan :exec
DELETE FROM group_bans WHERE group_id = $1 AND user_id = $2;

-- name: IsUserBannedFromGroup :one
SELECT EXISTS(SELECT 1 FROM group_bans WHERE group_id = $1 AND user_id = $2) AS banned;

-- name: ListGroupBans :many
SELECT gb.*, u.username
FROM group_bans gb
JOIN users u ON u.id = gb.user_id
WHERE gb.group_id = $1
ORDER BY gb.banned_at DESC;

-- name: CountGroupMembershipsForUser :one
-- Drives MAX_GROUP_MEMBERSHIPS_PER_USER cap on join. Counts only memberships
-- in non-soft-deleted groups; a soft-deleted group still has membership rows
-- but the user can't act inside it.
SELECT count(*) FROM group_memberships gm
JOIN groups g ON g.id = gm.group_id
WHERE gm.user_id = $1 AND g.deleted_at IS NULL;

-- name: DeleteAllMembershipsForUser :exec
-- Used by the platform-ban cascade. Drops every membership row in one shot;
-- the caller is responsible for triggering auto-promotion in any group that
-- this leaves admin-less.
DELETE FROM group_memberships WHERE user_id = $1;

-- name: ListGroupsWhereUserIsSoleAdmin :many
-- Used by the platform-ban cascade BEFORE deleting memberships, so the
-- caller knows which groups need an immediate auto-promotion run.
SELECT gm.group_id
FROM group_memberships gm
WHERE gm.user_id = $1 AND gm.role = 'admin'
  AND NOT EXISTS (
    SELECT 1 FROM group_memberships gm2
    WHERE gm2.group_id = gm.group_id
      AND gm2.role = 'admin'
      AND gm2.user_id != gm.user_id
  );

-- name: ScanDormantSoleAdmins :many
-- The 90-day auto-promotion scan. Returns each (group_id, dormant_admin_id)
-- pair where the admin has not logged in for 90+ days AND no other admin in
-- the group has logged in within that window. The caller iterates and either
-- promotes the longest-tenured active member or notifies the platform admin.
SELECT gm.group_id, gm.user_id
FROM group_memberships gm
JOIN users u ON u.id = gm.user_id
JOIN groups g ON g.id = gm.group_id
WHERE g.deleted_at IS NULL
  AND gm.role = 'admin'
  AND (u.last_login_at IS NULL OR u.last_login_at < now() - sqlc.arg(dormant_after)::interval)
  AND NOT EXISTS (
    SELECT 1 FROM group_memberships gm2
    JOIN users u2 ON u2.id = gm2.user_id
    WHERE gm2.group_id = gm.group_id
      AND gm2.role = 'admin'
      AND gm2.user_id != gm.user_id
      AND u2.last_login_at IS NOT NULL
      AND u2.last_login_at >= now() - sqlc.arg(dormant_after)::interval
  );

-- name: PickPromotionCandidate :one
-- Finds the longest-tenured "active" (logged in within the dormancy window)
-- non-admin member of a group. Returns no rows when no candidate exists; the
-- caller surfaces that as a platform-admin notification per the spec.
SELECT gm.user_id
FROM group_memberships gm
JOIN users u ON u.id = gm.user_id
WHERE gm.group_id = $1
  AND gm.role = 'member'
  AND u.last_login_at IS NOT NULL
  AND u.last_login_at >= now() - sqlc.arg(dormant_after)::interval
ORDER BY gm.joined_at ASC
LIMIT 1;
