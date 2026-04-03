-- backend/db/migrations/002_seed_game_types.down.sql
DELETE FROM game_types WHERE slug = 'meme-caption';
