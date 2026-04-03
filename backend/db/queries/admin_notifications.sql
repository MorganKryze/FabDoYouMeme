-- backend/db/queries/admin_notifications.sql

-- name: CreateAdminNotification :one
INSERT INTO admin_notifications (type, pack_id, actor_id) VALUES ($1, $2, $3) RETURNING *;

-- name: ListAdminNotifications :many
SELECT * FROM admin_notifications
WHERE (sqlc.arg(unread_only)::bool = false OR read_at IS NULL)
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: MarkNotificationRead :one
UPDATE admin_notifications SET read_at = now() WHERE id = $1 RETURNING *;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM admin_notifications WHERE read_at IS NULL;
