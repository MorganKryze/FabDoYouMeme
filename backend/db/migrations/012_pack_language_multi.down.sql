-- Migration 012 DOWN — revert 'multi' to 'en' and shrink the CHECK set.

UPDATE game_packs SET language = 'en' WHERE language = 'multi';

ALTER TABLE game_packs DROP CONSTRAINT IF EXISTS game_packs_language_check;
ALTER TABLE game_packs ADD CONSTRAINT game_packs_language_check
  CHECK (language IN ('en', 'fr'));
