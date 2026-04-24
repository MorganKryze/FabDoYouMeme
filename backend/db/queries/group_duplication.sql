-- backend/db/queries/group_duplication.sql
-- NSFW→SFW approval queue. See docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.

-- name: CreatePendingDuplication :one
INSERT INTO group_duplication_pending (group_id, source_pack_id, requested_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPendingDuplication :one
SELECT * FROM group_duplication_pending WHERE id = $1;

-- name: ListOpenPendingDuplications :many
-- Admin-facing queue view for a group. Excludes resolved rows by design so
-- the list stays focused on what needs action.
SELECT gdp.*, gp.name AS source_pack_name, gp.classification AS source_classification,
       u.username AS requested_by_username
FROM group_duplication_pending gdp
JOIN game_packs gp ON gp.id = gdp.source_pack_id
JOIN users u ON u.id = gdp.requested_by
WHERE gdp.group_id = $1 AND gdp.resolved_at IS NULL
ORDER BY gdp.requested_at DESC;

-- name: ResolvePendingDuplication :one
-- Single-shot resolve: stamps resolved_at/resolved_by/resolution, returns the
-- updated row. Idempotent on resolution — a second accept/reject is rejected
-- by the handler via the resolved_at check, not here.
UPDATE group_duplication_pending
SET resolved_at = now(), resolved_by = $2, resolution = $3
WHERE id = $1 AND resolved_at IS NULL
RETURNING *;
