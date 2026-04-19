-- backend/db/migrations/010_rename_game_type_slugs.down.sql
UPDATE game_types SET slug = 'meme-caption' WHERE slug = 'meme-freestyle';
UPDATE game_types SET slug = 'meme-vote'    WHERE slug = 'meme-showdown';
