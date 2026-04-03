-- backend/db/migrations/001_initial_schema.down.sql
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS votes;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS rounds;
DROP TABLE IF EXISTS room_players;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS admin_notifications;
ALTER TABLE game_items DROP CONSTRAINT IF EXISTS fk_current_version;
DROP TABLE IF EXISTS game_item_versions;
DROP TABLE IF EXISTS game_items;
DROP TABLE IF EXISTS game_packs;
DROP TABLE IF EXISTS game_types;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS magic_link_tokens;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
