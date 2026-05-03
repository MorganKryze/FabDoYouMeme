-- backend/db/queries/game_items.sql

-- name: CreateItem :one
INSERT INTO game_items (pack_id, name, position, payload_version)
SELECT $1, $2, COALESCE(MAX(position), 0) + 1, $3 FROM game_items WHERE pack_id = $1
RETURNING *;

-- name: GetItemByID :one
SELECT * FROM game_items WHERE id = $1;

-- name: ListItemsForPack :many
-- COALESCE on `payload` is what makes sqlc emit json.RawMessage instead of
-- []byte for this column. With the bare LEFT JOIN sqlc treats `giv.payload`
-- as nullable and falls back from the jsonb→RawMessage override declared in
-- sqlc.yaml, which silently base64-encoded the payload field in the JSON
-- response and broke the studio's text-snippet rendering for every text
-- item. The fallback to '{}' has no semantic effect — items without a
-- current version (orphans) carry an empty payload either way.
SELECT gi.*,
       giv.media_key,
       COALESCE(giv.payload, '{}'::jsonb)::jsonb AS payload,
       giv.version_number
FROM game_items gi
LEFT JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE gi.pack_id = $1 AND gi.deleted_at IS NULL
ORDER BY gi.position ASC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: SoftDeleteItem :exec
UPDATE game_items SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL;

-- name: SweepOrphanItems :execresult
-- Hard-delete items that never reached a confirmed version. The pre-bulk
-- upload pipeline could leave such rows behind when a mid-chain step (S3
-- upload, version insert, promote) failed and the client-side cleanup DELETE
-- was itself rate-limited. The bulk endpoint now wraps the chain in a
-- transaction so new orphans are impossible, but a startup sweep is the
-- defence-in-depth that cleans historical rows. The 1-hour grace window
-- keeps in-flight uploads safe from being eaten between the CreateItem call
-- and the SetCurrentVersion call (worst case a few seconds, even on a slow
-- network).
DELETE FROM game_items
WHERE current_version_id IS NULL
  AND deleted_at IS NULL
  AND created_at < now() - interval '1 hour';

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

-- name: ListPackItemsByPayloadVersion :many
-- Returns every non-deleted item in a pack whose current version payload
-- matches a specific payload_version. Used by the hub to build per-game
-- decks (e.g. the text-caption deck for meme-showdown) where one item per row
-- is enough and a single random-pick is not.
SELECT gi.id, giv.payload
FROM game_items gi
JOIN game_item_versions giv ON giv.id = gi.current_version_id
WHERE gi.pack_id = $1
  AND gi.deleted_at IS NULL
  AND giv.deleted_at IS NULL
  AND gi.payload_version = $2;

-- name: GetCurrentVersionsForItems :many
-- Batch lookup used by the replay enrichment path for showdown game types:
-- given a set of card item IDs, return each item's current version payload
-- so the handler can splice the card text into submission payloads before
-- returning. Soft-deleted items and versions are excluded so a tombstoned
-- card can't bleed back into a replay row's text.
SELECT gi.id AS item_id, giv.payload
FROM game_items gi
JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE gi.id = ANY(sqlc.arg(ids)::uuid[])
  AND gi.deleted_at IS NULL
  AND giv.deleted_at IS NULL;

-- name: ListVersionsMissingOrientation :many
-- Returns every non-deleted version that has a media_key but no `orientation`
-- key in its payload. Used by the startup backfill to enrich pre-existing
-- rows uploaded before orientation detection landed. The JSONB `?` operator
-- is used here because payload->>'orientation' returns NULL both for
-- "key missing" and "key set to JSON null"; we want to skip only the former.
SELECT id, media_key, payload
FROM game_item_versions
WHERE media_key IS NOT NULL
  AND media_key <> ''
  AND deleted_at IS NULL
  AND NOT (payload ? 'orientation');

-- name: SetVersionOrientation :exec
-- Merges the orientation key into an existing version payload without
-- creating a new version row. JSONB `||` right-merges so any prior keys
-- (e.g. sha256) are preserved.
UPDATE game_item_versions
SET payload = payload || jsonb_build_object('orientation', sqlc.arg(orientation)::text)
WHERE id = $1;

-- name: GetRandomUnplayedItems :many
-- Picks one random unplayed item for the next round. `gi.deleted_at IS NULL`
-- and `giv.deleted_at IS NULL` keep tombstoned items out of the deck so
-- mid-game soft-deletes (e.g. an admin pulling a card) can't surface in a
-- live room.
SELECT gi.*, giv.media_key, giv.payload, giv.id AS version_id
FROM game_items gi
JOIN game_item_versions giv ON gi.current_version_id = giv.id
WHERE gi.pack_id = $1
  AND gi.payload_version = ANY(sqlc.arg(versions)::int[])
  AND gi.deleted_at IS NULL
  AND giv.deleted_at IS NULL
  AND gi.id NOT IN (
    SELECT item_id FROM rounds WHERE room_id = sqlc.arg(room_id)
  )
ORDER BY random()
LIMIT 1;
