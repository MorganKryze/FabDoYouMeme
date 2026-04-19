DROP INDEX IF EXISTS rooms_text_pack_id_idx;
ALTER TABLE rooms DROP COLUMN IF EXISTS text_pack_id;
