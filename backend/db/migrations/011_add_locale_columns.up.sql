-- Migration 011 — add per-row language columns.
--
-- Three parallel columns, one migration:
--   users.locale         → user's preferred UI language
--   game_packs.language  → authored language of the pack's content
--   invites.locale       → locale for the invite email sent to an unregistered address
--
-- All three use the same CHECK constraint set (en, fr). Future locales
-- extend the set in a follow-up migration.
--
-- Indexes: only game_packs gets one because the pack list endpoint
-- filters by language. Users and invites are looked up by primary key
-- or session; an index on locale would bloat for no read benefit.

ALTER TABLE users
  ADD COLUMN locale TEXT NOT NULL DEFAULT 'en'
  CHECK (locale IN ('en', 'fr'));

ALTER TABLE game_packs
  ADD COLUMN language TEXT NOT NULL DEFAULT 'en'
  CHECK (language IN ('en', 'fr'));

CREATE INDEX IF NOT EXISTS game_packs_language_idx
  ON game_packs(language)
  WHERE deleted_at IS NULL;

ALTER TABLE invites
  ADD COLUMN locale TEXT NOT NULL DEFAULT 'en'
  CHECK (locale IN ('en', 'fr'));
