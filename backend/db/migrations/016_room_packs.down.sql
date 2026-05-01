-- 016_room_packs.down.sql
-- Restore the per-role pack columns on rooms by collapsing each role's
-- weighted list down to its highest-weighted pack (ties broken arbitrarily
-- by pack_id). Rooms created with truly-multi-pack rolls lose the secondary
-- entries — the down migration is a one-way view of best-effort recovery,
-- not a lossless inverse.

ALTER TABLE rooms ADD COLUMN pack_id      UUID REFERENCES game_packs(id);
ALTER TABLE rooms ADD COLUMN text_pack_id UUID REFERENCES game_packs(id);

WITH ranked AS (
  SELECT room_id, role, pack_id,
         ROW_NUMBER() OVER (PARTITION BY room_id, role ORDER BY weight DESC, pack_id) AS rn
  FROM room_packs
),
primary_pick AS (
  SELECT r.id AS room_id, ranked.pack_id
  FROM rooms r
  JOIN game_types gt ON r.game_type_id = gt.id
  JOIN ranked
    ON ranked.room_id = r.id
   AND ranked.rn = 1
   AND ranked.role = CASE gt.slug
                       WHEN 'meme-freestyle'   THEN 'image'
                       WHEN 'meme-showdown'    THEN 'image'
                       WHEN 'prompt-freestyle' THEN 'prompt'
                       WHEN 'prompt-showdown'  THEN 'prompt'
                     END
)
UPDATE rooms r SET pack_id = pp.pack_id FROM primary_pick pp WHERE pp.room_id = r.id;

WITH ranked AS (
  SELECT room_id, role, pack_id,
         ROW_NUMBER() OVER (PARTITION BY room_id, role ORDER BY weight DESC, pack_id) AS rn
  FROM room_packs
),
secondary_pick AS (
  SELECT r.id AS room_id, ranked.pack_id
  FROM rooms r
  JOIN game_types gt ON r.game_type_id = gt.id
  JOIN ranked
    ON ranked.room_id = r.id
   AND ranked.rn = 1
   AND ranked.role = CASE gt.slug
                       WHEN 'meme-showdown'   THEN 'text'
                       WHEN 'prompt-showdown' THEN 'filler'
                     END
)
UPDATE rooms r SET text_pack_id = sp.pack_id FROM secondary_pick sp WHERE sp.room_id = r.id;

ALTER TABLE rooms ALTER COLUMN pack_id SET NOT NULL;

DROP TABLE room_packs;
