-- backend/db/migrations/010_rename_game_type_slugs.up.sql
--
-- Rename the two existing game_types rows in place so existing rooms
-- (which reference game_type_id, not slug) stay attached. After this
-- migration, the startup SyncGameTypes upsert still finds the correct
-- row via the new slug and only touches mutable metadata (name,
-- description, version, config).
UPDATE game_types SET slug = 'meme-freestyle' WHERE slug = 'meme-caption';
UPDATE game_types SET slug = 'meme-showdown'  WHERE slug = 'meme-vote';
