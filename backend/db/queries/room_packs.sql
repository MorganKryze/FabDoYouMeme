-- backend/db/queries/room_packs.sql

-- name: InsertRoomPack :exec
INSERT INTO room_packs (room_id, role, pack_id, weight)
VALUES ($1, $2, $3, $4);

-- name: ListRoomPacks :many
-- Every (role, pack_id, weight) tuple for a room. Joined with game_packs so
-- callers that need the pack name (lobby UI, replay header) avoid a second
-- round-trip. Ordered by role then weight so the heaviest pack per role is
-- the first row of its group — useful for "primary mix" UI display.
SELECT rp.role, rp.pack_id, rp.weight, gp.name AS pack_name, gp.language AS pack_language
FROM room_packs rp
JOIN game_packs gp ON rp.pack_id = gp.id
WHERE rp.room_id = $1
ORDER BY rp.role, rp.weight DESC, rp.pack_id;

-- name: DeleteRoomPacks :exec
-- Used when PATCH /api/rooms/{code}/config rewrites the pack mix in lobby.
-- Followed by N InsertRoomPack calls in the same transaction.
DELETE FROM room_packs WHERE room_id = $1;
