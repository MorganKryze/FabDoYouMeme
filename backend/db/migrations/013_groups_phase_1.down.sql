-- backend/db/migrations/013_groups_phase_1.down.sql

BEGIN;

ALTER TABLE rooms       DROP COLUMN IF EXISTS group_id;

ALTER TABLE game_items  DROP COLUMN IF EXISTS last_editor_user_id,
                        DROP COLUMN IF EXISTS last_edited_at;

ALTER TABLE game_packs  DROP CONSTRAINT IF EXISTS game_packs_group_no_user_owner_chk,
                        DROP CONSTRAINT IF EXISTS game_packs_group_not_system_chk,
                        DROP COLUMN IF EXISTS group_id,
                        DROP COLUMN IF EXISTS classification,
                        DROP COLUMN IF EXISTS duplicated_from_pack_id,
                        DROP COLUMN IF EXISTS duplicated_by_user_id;

DROP TABLE IF EXISTS user_invite_quotas;
DROP TABLE IF EXISTS group_bans;
DROP TABLE IF EXISTS group_memberships;
DROP TABLE IF EXISTS groups;

ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;

COMMIT;
