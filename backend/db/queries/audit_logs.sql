-- backend/db/queries/audit_logs.sql

-- name: CreateAuditLog :one
INSERT INTO audit_logs (admin_id, action, resource, changes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListRecentAuditLogs :many
-- Admin dashboard feed: joins the audit log to users so the UI can render
-- "action · who · when" without a second roundtrip. LEFT JOIN so logs survive
-- after a GDPR hard-delete replaces admin_id with the sentinel.
SELECT
  a.id,
  a.admin_id,
  a.action,
  a.resource,
  a.changes,
  a.created_at,
  COALESCE(u.username, '')::text AS admin_username
FROM audit_logs a
LEFT JOIN users u ON a.admin_id = u.id
ORDER BY a.created_at DESC
LIMIT sqlc.arg(lim);
