-- backend/db/migrations/007_system_pack_support.up.sql
--
-- Adds the is_system flag to game_packs and a deleted_at column to game_items.
--
-- is_system = true means the pack is managed by the filesystem (via
-- backend/internal/systempack). Every mutating pack/item handler rejects writes
-- to is_system=true packs with 403 system_pack_readonly. Default false for
-- existing rows.
--
-- deleted_at on game_items supports the sync loop's "file removed from the
-- bundled folder" case. Hard-deleting would break rounds.item_id (RESTRICT),
-- so we soft-delete and filter ListItemsForPack. Existing round-path queries
-- look up items by id and continue working unchanged.

ALTER TABLE game_packs
  ADD COLUMN is_system BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_game_packs_is_system
  ON game_packs(is_system) WHERE is_system = true;

ALTER TABLE game_items
  ADD COLUMN deleted_at TIMESTAMPTZ;
