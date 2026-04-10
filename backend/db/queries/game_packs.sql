-- backend/db/queries/game_packs.sql

-- name: CreatePack :one
INSERT INTO game_packs (name, description, owner_id, is_official, visibility)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPackByID :one
SELECT * FROM game_packs WHERE id = $1 AND deleted_at IS NULL;

-- name: ListPacksForUser :many
SELECT * FROM game_packs
WHERE deleted_at IS NULL
  AND (owner_id = sqlc.arg(user_id) OR (visibility = 'public' AND status = 'active'))
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListAllPacksAdmin :many
SELECT * FROM game_packs WHERE deleted_at IS NULL
ORDER BY created_at DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: UpdatePack :one
UPDATE game_packs SET name = $2, description = $3, visibility = $4 WHERE id = $1 RETURNING *;

-- name: SetPackStatus :one
UPDATE game_packs SET status = $2 WHERE id = $1 RETURNING *;

-- name: SoftDeletePack :exec
UPDATE game_packs SET deleted_at = now() WHERE id = $1;

-- name: CountCompatibleItems :one
SELECT COUNT(*) FROM game_items gi
JOIN game_packs gp ON gi.pack_id = gp.id
WHERE gi.pack_id = $1
  AND gi.payload_version = ANY(sqlc.arg(versions)::int[])
  AND gp.deleted_at IS NULL;

-- name: CanUserDownloadMedia :one
-- Authorization predicate for /api/assets/download-url. Returns true when the
-- given user_id is allowed to download the media at media_key, which holds
-- iff the media belongs to a non-soft-deleted version inside a non-soft-
-- deleted pack that EITHER the user owns OR is public+active. Admin callers
-- bypass this query entirely in the handler — encoding the role lookup in SQL
-- would force a second join on users for every download.
SELECT EXISTS (
  SELECT 1
  FROM game_item_versions giv
  JOIN game_items gi ON giv.item_id = gi.id
  JOIN game_packs gp ON gi.pack_id = gp.id
  WHERE giv.media_key = sqlc.arg(media_key)
    AND giv.deleted_at IS NULL
    AND gp.deleted_at IS NULL
    AND (
      gp.owner_id = sqlc.arg(user_id)::uuid
      OR (gp.visibility = 'public' AND gp.status = 'active')
    )
) AS allowed;
