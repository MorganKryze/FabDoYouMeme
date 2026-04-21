-- Migration 011 DOWN — drop locale columns.

DROP INDEX IF EXISTS game_packs_language_idx;

ALTER TABLE invites DROP COLUMN IF EXISTS locale;
ALTER TABLE game_packs DROP COLUMN IF EXISTS language;
ALTER TABLE users DROP COLUMN IF EXISTS locale;
