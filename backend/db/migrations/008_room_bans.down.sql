-- backend/db/migrations/008_room_bans.down.sql
DROP INDEX IF EXISTS room_bans_room_idx;
DROP INDEX IF EXISTS room_bans_guest_unique;
DROP INDEX IF EXISTS room_bans_user_unique;
DROP TABLE IF EXISTS room_bans;
