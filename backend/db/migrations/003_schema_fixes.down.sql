-- backend/db/migrations/003_schema_fixes.down.sql

-- Reverse game_packs index
DROP INDEX IF EXISTS game_packs_owner_id_idx;

-- Reverse rooms index
DROP INDEX IF EXISTS rooms_host_id_idx;

-- Reverse users.invited_by (restore RESTRICT default by dropping explicit constraint;
-- PostgreSQL does not store "RESTRICT" explicitly, re-adding the FK with no ON DELETE
-- clause is equivalent)
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_invited_by_fkey,
  ADD CONSTRAINT users_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id);

-- Reverse rooms.host_id (restore NOT NULL + RESTRICT)
ALTER TABLE rooms
  DROP CONSTRAINT IF EXISTS rooms_host_id_fkey,
  ALTER COLUMN host_id SET NOT NULL,
  ADD CONSTRAINT rooms_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id);

-- Note: the sentinel row inserted in migration 001 is removed implicitly
-- when the users table is dropped in migration 001's down file. No explicit
-- DELETE is needed here because migration 003 does not touch the sentinel row.
