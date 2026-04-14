-- 006_add_user_is_protected.up.sql
--
-- Adds is_protected to users. Protected users cannot be deleted or have
-- their role changed via the admin API. The flag is set for the bootstrap
-- admin (SEED_ADMIN_EMAIL) on every startup by auth.SeedAdmin so the
-- guarantee survives both env-var drift and admin self-renames.
--
-- Default false for existing rows — the first startup after this migration
-- will stamp the bootstrap admin via the idempotent path in SeedAdmin.

ALTER TABLE users
  ADD COLUMN is_protected BOOLEAN NOT NULL DEFAULT false;
