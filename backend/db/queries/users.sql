-- backend/db/queries/users.sql

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: CreateUser :one
INSERT INTO users (username, email, role, is_active, invited_by, consent_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateUserUsername :one
UPDATE users SET username = $2 WHERE id = $1 RETURNING *;

-- name: SetPendingEmail :one
UPDATE users SET pending_email = $2 WHERE id = $1 RETURNING *;

-- name: ConfirmEmailChange :one
UPDATE users SET email = pending_email, pending_email = NULL WHERE id = $1 RETURNING *;

-- name: UpdateUserRole :one
UPDATE users SET role = $2 WHERE id = $1 RETURNING *;

-- name: SetUserActive :one
UPDATE users SET is_active = $2 WHERE id = $1 RETURNING *;

-- name: UpdateUserEmailAdmin :one
UPDATE users SET email = $2 WHERE id = $1 RETURNING *;

-- name: ListUsers :many
-- Excludes the GDPR sentinel row (see SentinelUserID). The sentinel is a
-- data-integrity placeholder, never a real account, so it must not appear
-- in admin tooling or counts.
--
-- games_played: count of finished rooms this user was a player in. Scoped
-- to state='finished' so lobby/in-progress rooms don't inflate the figure.
--
-- last_login_at: MAX(sessions.created_at). Sessions are deleted on logout,
-- so this is "most recent live-login timestamp" — NULL for users who are
-- fully logged out. Good enough as a health signal without schema churn;
-- graduate to a dedicated users.last_login_at column if accuracy matters.
SELECT
  u.id,
  u.username,
  u.email,
  u.pending_email,
  u.role,
  u.is_active,
  u.invited_by,
  u.consent_at,
  u.created_at,
  u.is_protected,
  COALESCE((
    SELECT COUNT(*)
    FROM room_players rp
    JOIN rooms r ON r.id = rp.room_id
    WHERE rp.user_id = u.id AND r.state = 'finished'
  ), 0)::bigint AS games_played,
  (SELECT MAX(s.created_at) FROM sessions s WHERE s.user_id = u.id) AS last_login_at
FROM users u
WHERE u.id != '00000000-0000-0000-0000-000000000001'
  AND (lower(u.username) LIKE lower('%' || sqlc.arg(search) || '%')
       OR lower(u.email) LIKE lower('%' || sqlc.arg(search) || '%'))
ORDER BY u.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountUsers :one
-- Matches ListUsers' sentinel exclusion so the admin dashboard count and
-- table row count always agree.
SELECT COUNT(*) FROM users
WHERE id != '00000000-0000-0000-0000-000000000001'
  AND (lower(username) LIKE lower('%' || sqlc.arg(search) || '%')
       OR lower(email) LIKE lower('%' || sqlc.arg(search) || '%'));

-- name: GetUsernamesByIDs :many
-- Batch lookup for the audit-log enrichment path. Returns only id + username
-- so the admin dashboard can resolve "user:<uuid>" audit resources without
-- paying for full user rows.
SELECT id, username FROM users WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: SetUserProtected :exec
-- Toggles the is_protected flag. Called exclusively by auth.SeedAdmin to
-- stamp the bootstrap admin. There is deliberately no handler that flips
-- this from the admin API — protection is a runtime-immutable property.
UPDATE users SET is_protected = $2 WHERE id = $1;

-- Sentinel UUID: 00000000-0000-0000-0000-000000000001 (see auth.SentinelUserID in Go).
-- Used to anonymize submissions/votes on hard-delete without breaking referential integrity.

-- name: GetSentinelUser :one
SELECT * FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- name: HardDeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: UpdateSubmissionsSentinel :exec
UPDATE submissions SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id = $1;

-- name: UpdateVotesSentinel :exec
UPDATE votes SET voter_id = '00000000-0000-0000-0000-000000000001' WHERE voter_id = $1;

-- name: GetUserGameHistory :many
-- Lists finished rooms the user participated in. Excludes rooms that
-- carry no actual gameplay: the replay screen would render an empty
-- stepper for these, which looks like a broken link. Concretely: a
-- room only appears here if at least one submission landed in it.
-- This covers 24h-auto-closed lobbies (no rounds at all), rooms killed
-- mid-round-zero via host_disconnected/pack_exhausted, and rounds that
-- started but got aborted before anyone could submit.
SELECT
  r.code,
  gt.slug AS game_type_slug,
  gp.name AS pack_name,
  r.created_at AS started_at,
  r.finished_at,
  rp.score,
  r.id AS room_id,
  ((SELECT COUNT(*) FROM room_players rp2 WHERE rp2.room_id = r.id AND rp2.score > rp.score) + 1)::bigint AS rank,
  (SELECT COUNT(*) FROM room_players rp3 WHERE rp3.room_id = r.id) AS player_count
FROM room_players rp
JOIN rooms r ON rp.room_id = r.id
JOIN game_types gt ON r.game_type_id = gt.id
JOIN game_packs gp ON r.pack_id = gp.id
WHERE rp.user_id = $1
  AND r.state = 'finished'
  AND EXISTS (
    SELECT 1 FROM submissions s
    JOIN rounds rnd ON s.round_id = rnd.id
    WHERE rnd.room_id = r.id
  )
ORDER BY r.finished_at DESC NULLS LAST
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: GetUserSubmissions :many
SELECT
  s.id,
  s.round_id,
  r2.code AS room_code,
  gt.slug AS game_type_slug,
  s.payload,
  s.created_at
FROM submissions s
JOIN rounds rnd ON s.round_id = rnd.id
JOIN rooms r2 ON rnd.room_id = r2.id
JOIN game_types gt ON r2.game_type_id = gt.id
WHERE s.user_id = $1
  AND r2.state = 'finished'
ORDER BY s.created_at DESC;
