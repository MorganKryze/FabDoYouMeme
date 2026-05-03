-- backend/db/queries/user_invite_quotas.sql
--
-- Per-user platform+group invite allocation. Set by the platform admin;
-- consumed when a user mints a platform+group invite (phase 2). The CHECK
-- (used <= allocated) constraint defends against admin lowering allocation
-- below current consumption — the handler maps the resulting pg error to
-- a 409.

-- name: UpsertUserInviteQuota :one
INSERT INTO user_invite_quotas (user_id, allocated, used)
VALUES ($1, $2, 0)
ON CONFLICT (user_id) DO UPDATE
SET allocated = EXCLUDED.allocated, updated_at = now()
RETURNING *;

-- name: GetUserInviteQuota :one
SELECT * FROM user_invite_quotas WHERE user_id = $1;

-- name: ListUserInviteQuotas :many
-- Admin overview: every user that has a quota row, alphabetised by username
-- so the table is stable across renders.
SELECT q.*, u.username, u.email
FROM user_invite_quotas q
JOIN users u ON u.id = q.user_id
ORDER BY u.username ASC;

-- name: ConsumeUserInviteQuota :one
-- Atomic decrement-by-incrementing-used at platform+group invite mint time.
-- Returns no row (sql.ErrNoRows) when the user is at or above their cap, so
-- the handler maps that to platform_plus_quota_exhausted. We do NOT
-- pre-create a row here — admins must explicitly allocate a quota first.
UPDATE user_invite_quotas
SET used = used + 1, updated_at = now()
WHERE user_id = $1 AND used < allocated
RETURNING *;

-- name: ConsumeUserInviteQuotaN :one
-- Multi-slot variant for multi-use platform+group invites. The mint debits
-- max_uses slots upfront so the admin cannot mint a 50-use code from a
-- 1-slot allowance. Same return-no-row contract as the single-slot version
-- when the requested debit would exceed the user's cap.
UPDATE user_invite_quotas
SET used = used + sqlc.arg(amount)::int, updated_at = now()
WHERE user_id = sqlc.arg(user_id)::uuid AND used + sqlc.arg(amount)::int <= allocated
RETURNING *;
