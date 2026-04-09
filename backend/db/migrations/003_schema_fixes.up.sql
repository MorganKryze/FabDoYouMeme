-- backend/db/migrations/003_schema_fixes.up.sql

-- A3-C1: rooms.host_id — allow ON DELETE SET NULL for GDPR hard-delete.
-- Without this, deleting a user who hosted any room (even finished) fails.
-- Making host_id nullable lets rooms survive after their host is erased.
ALTER TABLE rooms
  DROP CONSTRAINT rooms_host_id_fkey,
  ALTER COLUMN host_id DROP NOT NULL,
  ADD CONSTRAINT rooms_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE SET NULL;

-- A3-H1: users.invited_by — add ON DELETE SET NULL (was RESTRICT by default).
-- All other nullable user FKs already use ON DELETE SET NULL; this aligns them.
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_invited_by_fkey,
  ADD CONSTRAINT users_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL;

-- A3-H3: index on rooms.host_id for FK cascade performance and host queries.
CREATE INDEX ON rooms(host_id);

-- A3-M1: simple non-partial index on game_packs.owner_id for FK cascade.
-- The existing partial index (WHERE deleted_at IS NULL) is not used by the
-- FK engine for ON DELETE SET NULL operations.
CREATE INDEX ON game_packs(owner_id);
