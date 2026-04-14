-- Add a human-readable display name to game_items. Defaults to '' so existing
-- rows stay valid without a backfill; the API requires a non-empty name on
-- creation.
ALTER TABLE game_items ADD COLUMN name TEXT NOT NULL DEFAULT '';
