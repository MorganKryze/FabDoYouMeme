-- backend/db/migrations/002_seed_game_types.up.sql
INSERT INTO game_types (slug, name, description, version, supports_solo, config)
VALUES (
  'meme-caption',
  'Meme Caption',
  'Write the funniest caption for an image. Others vote for their favourite.',
  '1.0.0',
  false,
  '{
    "min_round_duration_seconds":      15,
    "max_round_duration_seconds":      300,
    "default_round_duration_seconds":  60,
    "min_voting_duration_seconds":     10,
    "max_voting_duration_seconds":     120,
    "default_voting_duration_seconds": 30,
    "min_players":                     2,
    "max_players":                     null,
    "min_round_count":                 1,
    "max_round_count":                 50,
    "default_round_count":             10
  }'::jsonb
);
