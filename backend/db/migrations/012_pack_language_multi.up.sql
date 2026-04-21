-- Migration 012 — allow 'multi' as a pack language.
--
-- Image packs (memes, pictures) aren't inherently language-specific — a
-- funny image lands in any locale. Introducing a 'multi' value lets such
-- packs match hosts in any locale without duplicating rows per language.
-- Text packs stay authored-language (en/fr) because captions and prompts
-- are written in one language by definition.
--
-- The bundled image system pack is migrated to 'multi' so every host sees
-- it regardless of UI locale. The bundled text system pack stays 'en'.

ALTER TABLE game_packs DROP CONSTRAINT IF EXISTS game_packs_language_check;
ALTER TABLE game_packs ADD CONSTRAINT game_packs_language_check
  CHECK (language IN ('en', 'fr', 'multi'));

UPDATE game_packs
SET language = 'multi'
WHERE id = '00000000-0000-0000-0000-000000000001';
