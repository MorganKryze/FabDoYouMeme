-- 016_room_packs.up.sql
-- Replace the per-role pack columns on rooms with a join table that allows a
-- weighted list of packs per role. ADR-013 named this as the planned future
-- shape; ADR-016 (Weighted Multi-Pack Rooms) extends it with a `weight`
-- column. Existing rooms backfill at weight=1 so behaviour is identical until
-- a host adds a second pack.

CREATE TABLE room_packs (
  room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
  role    TEXT NOT NULL,
  pack_id UUID NOT NULL REFERENCES game_packs(id),
  weight  INT  NOT NULL DEFAULT 1 CHECK (weight > 0),
  PRIMARY KEY (room_id, role, pack_id)
);

CREATE INDEX room_packs_room_role_idx ON room_packs (room_id, role);

-- Backfill from the old positional columns. required_packs metadata lives
-- only in the handler manifests (not in game_types.config), so the role
-- name per slug is hard-coded here for the four shipped game types. New
-- game types added after this migration use the join table directly via
-- POST /api/rooms — no future backfill needed.
INSERT INTO room_packs (room_id, role, pack_id, weight)
SELECT r.id,
       CASE gt.slug
         WHEN 'meme-freestyle'   THEN 'image'
         WHEN 'meme-showdown'    THEN 'image'
         WHEN 'prompt-freestyle' THEN 'prompt'
         WHEN 'prompt-showdown'  THEN 'prompt'
       END,
       r.pack_id, 1
FROM rooms r
JOIN game_types gt ON r.game_type_id = gt.id
WHERE r.pack_id IS NOT NULL
  AND gt.slug IN ('meme-freestyle','meme-showdown','prompt-freestyle','prompt-showdown');

INSERT INTO room_packs (room_id, role, pack_id, weight)
SELECT r.id,
       CASE gt.slug
         WHEN 'meme-showdown'   THEN 'text'
         WHEN 'prompt-showdown' THEN 'filler'
       END,
       r.text_pack_id, 1
FROM rooms r
JOIN game_types gt ON r.game_type_id = gt.id
WHERE r.text_pack_id IS NOT NULL
  AND gt.slug IN ('meme-showdown','prompt-showdown');

ALTER TABLE rooms DROP COLUMN text_pack_id;
ALTER TABLE rooms DROP COLUMN pack_id;
