-- backend/db/migrations/014_group_invites.up.sql
-- Phase 2 of the groups paradigm: group-join + platform+group invite codes.
-- Token storage matches the existing platform `invites` table — plaintext,
-- because admins look up codes by token to revoke and the value is opaque
-- random bytes anyway. See docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.

BEGIN;

CREATE TABLE group_invites (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token            TEXT NOT NULL UNIQUE,
    group_id         UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    kind             TEXT NOT NULL CHECK (kind IN ('group_join', 'platform_plus_group')),
    restricted_email TEXT,
    max_uses         INT  NOT NULL DEFAULT 1 CHECK (max_uses > 0),
    uses_count       INT  NOT NULL DEFAULT 0 CHECK (uses_count >= 0),
    expires_at       TIMESTAMPTZ,
    revoked_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-(admin, group) "active codes" lookup for the rate-limit check on mint.
-- The full predicate (revoked_at IS NULL, uses_count < max_uses, expires_at
-- in the future) is evaluated in SQL; this index just narrows the scan.
CREATE INDEX group_invites_creator_group_active_idx
    ON group_invites (created_by, group_id)
    WHERE revoked_at IS NULL;

-- Token redemption is the hot read path; the UNIQUE constraint above already
-- gives us a btree index, so no additional index is needed for it.

COMMIT;
