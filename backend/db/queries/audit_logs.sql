-- backend/db/queries/audit_logs.sql

-- name: CreateAuditLog :one
INSERT INTO audit_logs (admin_id, action, resource, changes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
