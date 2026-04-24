-- backend/db/queries/group_notifications.sql
-- Per-group notification stream. Parallel to admin_notifications but scoped
-- to a group so the feed isn't polluted by cross-group moderation traffic.

-- name: CreateGroupNotification :one
INSERT INTO group_notifications (group_id, type, actor_id, subject_id, payload)
VALUES ($1, $2, sqlc.narg(actor_id), sqlc.narg(subject_id), $3)
RETURNING *;

-- name: ListGroupNotifications :many
-- Paginated descending-by-time list for the group's admins. The handler
-- gates access on admin role before calling.
SELECT * FROM group_notifications
WHERE group_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: MarkGroupNotificationRead :exec
UPDATE group_notifications SET read_at = now()
WHERE id = $1 AND group_id = $2 AND read_at IS NULL;

-- name: CountUnreadGroupNotifications :one
SELECT count(*) FROM group_notifications WHERE group_id = $1 AND read_at IS NULL;
