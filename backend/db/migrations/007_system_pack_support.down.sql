-- backend/db/migrations/007_system_pack_support.down.sql
ALTER TABLE game_items DROP COLUMN IF EXISTS deleted_at;
DROP INDEX IF EXISTS idx_game_packs_is_system;
ALTER TABLE game_packs DROP COLUMN IF EXISTS is_system;
