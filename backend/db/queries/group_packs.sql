-- backend/db/queries/group_packs.sql
-- Phase 3 of the groups paradigm. The existing game_packs queries cover
-- user-owned and system packs; this file adds the group-owned shape.

-- name: CreateGroupPack :one
-- Internal helper used by the duplication service; handler-level flows never
-- call it directly. owner_id stays NULL and is_official stays false — the
-- group_packs_group_no_user_owner_chk + group_packs_group_not_system_chk
-- constraints enforce that invariant.
INSERT INTO game_packs (
    name, description, owner_id, is_official, visibility, language,
    group_id, classification, duplicated_from_pack_id, duplicated_by_user_id
)
VALUES (
    $1, sqlc.narg(description), NULL, false, 'private', $2,
    $3, $4, sqlc.narg(duplicated_from_pack_id), sqlc.narg(duplicated_by_user_id)
)
RETURNING *;

-- name: ListGroupPacks :many
-- Every non-soft-deleted pack belonging to the group, newest first. The UI
-- filters by status client-side; `flagged` and `banned` packs are still
-- visible to the group so admins can see what the platform moderated.
SELECT * FROM game_packs
WHERE group_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: SumGroupPackMediaSize :one
-- Approximate quota fill: we don't have stored byte sizes per media object in
-- phase 3 so this returns the COUNT of current versions that carry a
-- media_key. The duplication service multiplies by an estimated average
-- size (`groupMediaSizeEstimateBytes`) for the pre-flight check — quota is
-- a soft bound for now and will be tightened once storage metadata lands.
SELECT COUNT(*)::bigint FROM game_item_versions giv
JOIN game_items gi ON gi.id = giv.item_id
JOIN game_packs gp ON gp.id = gi.pack_id
WHERE gp.group_id = $1
  AND gp.deleted_at IS NULL
  AND gi.deleted_at IS NULL
  AND giv.deleted_at IS NULL
  AND giv.media_key IS NOT NULL;

-- name: GetGroupPack :one
-- Member-gated read path used by the pack detail page. Does not short-circuit
-- on group ownership — the handler verifies the pack.group_id matches the
-- URL gid so we never serve a cross-group pack by accident.
SELECT * FROM game_packs WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteGroupPack :exec
-- Admin-only eviction. Uses the existing deleted_at column so historical
-- rooms that referenced the pack keep functioning via replay-redaction.
UPDATE game_packs SET deleted_at = now()
WHERE id = $1 AND group_id = $2 AND deleted_at IS NULL;

-- name: BumpGroupItemEditor :exec
-- Stamps the (last_editor_user_id, last_edited_at) audit pair on a group-pack
-- item after a successful add or modify. A no-op on non-group items (the
-- handler only calls it when it just loaded a group pack).
UPDATE game_items
SET last_editor_user_id = $2, last_edited_at = now()
WHERE id = $1;

-- name: ForceGroupClassificationNSFW :exec
-- Used by the duplication approval flow when an admin accepts an NSFW pack
-- into a SFW group — per spec the group is force-relabeled NSFW.
UPDATE groups SET classification = 'nsfw'
WHERE id = $1 AND deleted_at IS NULL;
