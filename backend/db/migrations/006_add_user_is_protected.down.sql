-- 006_add_user_is_protected.down.sql

ALTER TABLE users DROP COLUMN is_protected;
