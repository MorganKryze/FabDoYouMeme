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
