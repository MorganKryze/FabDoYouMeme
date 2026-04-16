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

-- name: CountAllPacks :one
-- Total non-deleted packs, regardless of visibility or status. Used by the
-- admin dashboard stats card.
SELECT COUNT(*) FROM game_packs WHERE deleted_at IS NULL;

-- name: GetPackNamesByIDs :many
-- Batch lookup for the audit-log enrichment path. Returns only id + name so
-- the admin dashboard can resolve "pack:<uuid>" audit resources into a
-- human-readable label. Soft-deleted packs are included on purpose — we
-- still want audit history to show what was banned/deleted.
SELECT id, name FROM game_packs WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: CountCompatibleItems :one
SELECT COUNT(*) FROM game_items gi
JOIN game_packs gp ON gi.pack_id = gp.id
WHERE gi.pack_id = $1
  AND gi.payload_version = ANY(sqlc.arg(versions)::int[])
  AND gp.deleted_at IS NULL;

-- name: UpsertSystemPack :one
-- Upserts the bundled "system" pack row with a fixed sentinel UUID. Called
-- once per boot from backend/internal/systempack. Forces is_official,
-- visibility, status, is_system, and deleted_at back to their canonical values
-- on every boot so the pack cannot drift from its managed state.
INSERT INTO game_packs (id, name, description, owner_id, is_official, visibility, status, is_system)
VALUES ($1, $2, $3, NULL, true, 'public', 'active', true)
ON CONFLICT (id) DO UPDATE
  SET name = EXCLUDED.name,
      description = EXCLUDED.description,
      is_official = true,
      visibility = 'public',
      status = 'active',
      is_system = true,
      deleted_at = NULL
RETURNING *;

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

-- name: CanGuestDownloadMedia :one
-- Authorization predicate for guest reads of /api/assets/media. Guests have no
-- session and cannot own packs, so the user-side rules in CanUserDownloadMedia
-- don't apply. Instead we scope visibility to "items the guest could plausibly
-- have seen in the room they joined" — a version is readable iff it belongs to
-- an item that was actually used in a round of a room the guest is currently a
-- player of. Narrower than "any media in the pack backing the room" so a guest
-- can't enumerate unshown items via media_key guessing.
SELECT EXISTS (
  SELECT 1
  FROM game_item_versions giv
  JOIN rounds rd ON rd.item_id = giv.item_id
  JOIN room_players rp ON rp.room_id = rd.room_id
  WHERE giv.media_key = sqlc.arg(media_key)
    AND giv.deleted_at IS NULL
    AND rp.guest_player_id = sqlc.arg(guest_player_id)::uuid
) AS allowed;
