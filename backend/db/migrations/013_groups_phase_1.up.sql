-- backend/db/migrations/013_groups_phase_1.up.sql
-- Phase 1 of the groups paradigm: schema only. CRUD handlers ship in the same
-- branch behind FEATURE_GROUPS=false; later phases (invites, duplications,
-- group-scoped rooms) extend this schema additively.
-- See docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.

BEGIN;

-- ── New column on users ────────────────────────────────────────────────────
-- Populated at session creation by the auth handler in a later phase. Nullable
-- to cover the window between migration and first login. Backfill with
-- created_at so the future 90-day auto-promotion scan does not immediately
-- flag everyone.
ALTER TABLE users
    ADD COLUMN last_login_at TIMESTAMPTZ;

UPDATE users SET last_login_at = created_at WHERE last_login_at IS NULL;

-- ── Groups ─────────────────────────────────────────────────────────────────
CREATE TABLE groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    name_normalized  TEXT GENERATED ALWAYS AS (lower(name)) STORED,
    description      TEXT NOT NULL CHECK (char_length(description) <= 500),
    avatar_media_key TEXT,
    language         TEXT NOT NULL CHECK (language IN ('en', 'fr', 'multi')),
    classification   TEXT NOT NULL CHECK (classification IN ('sfw', 'nsfw')),
    member_cap       INT  NOT NULL DEFAULT 100 CHECK (member_cap > 0),
    quota_bytes      BIGINT NOT NULL CHECK (quota_bytes >= 0),
    created_by       UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

-- Case-insensitive uniqueness across live (non-soft-deleted) groups. The
-- partial-index shape lets a hard-deleted name be re-used while still blocking
-- look-alike collisions during the 30-day soft-delete window.
CREATE UNIQUE INDEX groups_name_normalized_live_unique
    ON groups (name_normalized) WHERE deleted_at IS NULL;

CREATE INDEX groups_deleted_at_idx ON groups (deleted_at) WHERE deleted_at IS NOT NULL;

-- ── Memberships ────────────────────────────────────────────────────────────
CREATE TABLE group_memberships (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id)
);

CREATE INDEX group_memberships_user_idx
    ON group_memberships (user_id);
CREATE INDEX group_memberships_group_admin_idx
    ON group_memberships (group_id) WHERE role = 'admin';

-- ── Bans ───────────────────────────────────────────────────────────────────
CREATE TABLE group_bans (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    banned_by UUID REFERENCES users(id) ON DELETE SET NULL,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id)
);

CREATE INDEX group_bans_group_idx ON group_bans (group_id);

-- ── User invite quotas ─────────────────────────────────────────────────────
-- Platform+group invite allocation. Set by the platform admin; consumed at
-- mint time in phase 2. Present in phase 1 so the admin UI can allocate ahead.
CREATE TABLE user_invite_quotas (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    allocated  INT  NOT NULL DEFAULT 0 CHECK (allocated >= 0),
    used       INT  NOT NULL DEFAULT 0 CHECK (used >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (used <= allocated)
);

-- ── Extensions to game_packs (phase 1: additive only, not yet written to) ──
-- Shipped now so phase 3 (pack duplication) is a code-only change.
ALTER TABLE game_packs
    ADD COLUMN group_id                UUID REFERENCES groups(id) ON DELETE CASCADE,
    ADD COLUMN classification          TEXT NOT NULL DEFAULT 'sfw'
        CHECK (classification IN ('sfw', 'nsfw')),
    ADD COLUMN duplicated_from_pack_id UUID REFERENCES game_packs(id) ON DELETE SET NULL,
    ADD COLUMN duplicated_by_user_id   UUID REFERENCES users(id)       ON DELETE SET NULL;

-- Group-pack invariants. We do NOT impose full ownership exclusivity (the
-- spec's three-way split conflicts with existing admin-authored "official"
-- packs that legitimately have both owner_id and is_official=true). The two
-- new invariants are: a group-owned pack cannot also be user-owned, and a
-- group-owned pack cannot be a system pack. Both are conditions the legacy
-- data trivially satisfies (group_id is null on every pre-013 row).
ALTER TABLE game_packs
    ADD CONSTRAINT game_packs_group_no_user_owner_chk
    CHECK (group_id IS NULL OR owner_id IS NULL),
    ADD CONSTRAINT game_packs_group_not_system_chk
    CHECK (group_id IS NULL OR is_system = false);

CREATE INDEX game_packs_group_idx ON game_packs (group_id) WHERE group_id IS NOT NULL;

-- ── Extensions to game_items (phase 1: additive only) ──────────────────────
-- last_editor_user_id and last_edited_at feed the moderation audit trail
-- group admins read in phase 2+. Both nullable so existing rows are valid.
ALTER TABLE game_items
    ADD COLUMN last_editor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN last_edited_at      TIMESTAMPTZ;

-- ── Extensions to rooms (phase 1: additive only) ───────────────────────────
ALTER TABLE rooms
    ADD COLUMN group_id UUID REFERENCES groups(id) ON DELETE SET NULL;

CREATE INDEX rooms_group_idx ON rooms (group_id) WHERE group_id IS NOT NULL;

COMMIT;
