-- backend/db/migrations/015_group_duplication_and_notifications.down.sql

BEGIN;

DROP TABLE IF EXISTS group_notifications;
DROP TABLE IF EXISTS group_duplication_pending;

COMMIT;
