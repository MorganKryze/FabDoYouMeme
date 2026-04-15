-- backend/db/queries/game_items.sql

-- name: CreateItem :one
INSERT INTO game_items (pack_id, name, position, payload_version)
SELECT $1, $2, COALESCE(MAX(position), 0) + 1, $3 FROM game_items WHERE pack_id = $1
RETURNING *;

-- name: GetItemByID :one
SELECT * FROM game_items WHERE id = $1;

-- name: ListItemsForPack :many
SELECT gi.*, giv.media_key, giv.payload, giv.version_number
FROM game_items gi
LEFT JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE gi.pack_id = $1 AND gi.deleted_at IS NULL
ORDER BY gi.position ASC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: SoftDeleteItem :exec
UPDATE game_items SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SetCurrentVersion :one
UPDATE game_items SET current_version_id = $2 WHERE id = $1 RETURNING *;

-- name: DeleteItem :exec
DELETE FROM game_items WHERE id = $1;

-- name: CreateItemVersion :one
INSERT INTO game_item_versions (item_id, version_number, media_key, payload)
SELECT $1, COALESCE(MAX(version_number), 0) + 1, $2, $3 FROM game_item_versions WHERE item_id = $1
RETURNING *;

-- name: GetVersionByID :one
SELECT * FROM game_item_versions WHERE id = $1;

-- name: ListVersionsForItem :many
SELECT * FROM game_item_versions WHERE item_id = $1 ORDER BY version_number DESC;

-- name: SoftDeleteVersion :exec
UPDATE game_item_versions SET deleted_at = now() WHERE id = $1;

-- name: HardDeleteVersion :exec
DELETE FROM game_item_versions WHERE id = $1;

-- name: ReorderItems :exec
UPDATE game_items SET position = $2 WHERE id = $1 AND pack_id = $3;

-- name: GetRandomUnplayedItems :many
SELECT gi.*, giv.media_key, giv.payload, giv.id AS version_id
FROM game_items gi
JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE gi.pack_id = $1
  AND gi.payload_version = ANY(sqlc.arg(versions)::int[])
  AND gi.id NOT IN (
    SELECT item_id FROM rounds WHERE room_id = sqlc.arg(room_id)
  )
ORDER BY random()
LIMIT 1;
