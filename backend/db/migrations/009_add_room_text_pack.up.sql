-- Migration 009 — add rooms.text_pack_id
--
-- Some game types (e.g. meme-vote) need a second pack for text content
-- alongside the image pack referenced by rooms.pack_id. We model this as a
-- nullable FK so the column is only populated for rooms whose game type
-- declares a `text` pack role. Not renaming pack_id — it continues to hold
-- "the image pack" for rooms that have one, keeping existing queries intact.
ALTER TABLE rooms
  ADD COLUMN text_pack_id uuid NULL REFERENCES game_packs(id);

CREATE INDEX IF NOT EXISTS rooms_text_pack_id_idx ON rooms(text_pack_id)
  WHERE text_pack_id IS NOT NULL;
