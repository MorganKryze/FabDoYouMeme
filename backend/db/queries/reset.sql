-- backend/db/queries/reset.sql
--
-- Queries for the destructive admin actions ("danger zone"). Each query
-- returns the deleted ids so the service layer can count without an
-- extra COUNT round-trip. All queries assume they run inside a caller-
-- owned transaction.

-- name: DangerDeleteAllVotes :many
DELETE FROM votes RETURNING id;

-- name: DangerDeleteAllSubmissions :many
DELETE FROM submissions RETURNING id;

-- name: DangerDeleteAllRounds :many
DELETE FROM rounds RETURNING id;

-- name: DangerDeleteAllRoomPlayers :many
DELETE FROM room_players RETURNING room_id;

-- name: DangerDeleteAllRooms :many
DELETE FROM rooms RETURNING id;

-- name: DangerDeleteAllGameItemVersions :many
-- Items cascade from packs, versions cascade from items, so in practice
-- this count is only interesting if packs are NOT being deleted. Kept as
-- a separate query so the Report can still surface a version count when
-- WipePacksAndMedia runs.
DELETE FROM game_item_versions RETURNING id;

-- name: DangerDeleteAllGameItems :many
DELETE FROM game_items RETURNING id;

-- name: DangerDeleteAllGamePacks :many
DELETE FROM game_packs RETURNING id;

-- name: DangerDeleteAllAdminNotifications :many
DELETE FROM admin_notifications RETURNING id;

-- name: DangerDeleteAllInvites :many
DELETE FROM invites RETURNING id;

-- name: DangerDeleteSessionsExcept :many
-- Wipe every session except the acting admin's. Pass uuid.Nil to wipe all.
DELETE FROM sessions WHERE user_id != $1 RETURNING id;

-- name: DangerDeleteAllMagicLinkTokens :many
DELETE FROM magic_link_tokens RETURNING id;

-- name: DangerDeleteNonProtectedUsersExcept :many
-- Preserves: sentinel user, any user with is_protected=true, and the
-- acting admin (passed as $1).
DELETE FROM users
WHERE id != '00000000-0000-0000-0000-000000000001'::uuid
  AND is_protected = false
  AND id != $1
RETURNING id;
