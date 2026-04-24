-- backend/db/migrations/015_group_duplication_and_notifications.up.sql
-- Phase 3 of the groups paradigm: pack duplication + in-group notifications.
-- See docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.

BEGIN;

-- ── group_duplication_pending ─────────────────────────────────────────────
-- Queue for NSFW-into-SFW duplications awaiting group admin approval.
-- Every other mismatch (SFW→SFW, SFW→NSFW, NSFW→NSFW) is synchronous and
-- never touches this table. `resolution` stays NULL until an admin acts.
CREATE TABLE group_duplication_pending (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id        UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    source_pack_id  UUID NOT NULL REFERENCES game_packs(id) ON DELETE CASCADE,
    requested_by    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    resolved_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    resolution      TEXT CHECK (resolution IN ('accepted', 'rejected'))
);

-- Admins view the open queue per-group; the partial index keeps the scan
-- narrow even as the resolved history grows.
CREATE INDEX group_duplication_pending_open_idx
    ON group_duplication_pending (group_id, requested_at DESC)
    WHERE resolved_at IS NULL;

-- ── group_notifications ───────────────────────────────────────────────────
-- Per-group in-app notification stream (parallel to admin_notifications but
-- scoped to a group). Visibility rule: every group admin sees every row
-- where group_id matches. `type` enumerates every event that can land here.
-- Additional kinds will be added in later phases as needed.
CREATE TABLE group_notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id    UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN (
        'duplication_pending',
        'pack_evicted',
        'member_joined',
        'member_kicked',
        'member_banned',
        'auto_promotion'
    )),
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    subject_id  UUID,  -- pack / item / user id depending on `type`; free-form
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at     TIMESTAMPTZ
);

CREATE INDEX group_notifications_group_created_idx
    ON group_notifications (group_id, created_at DESC);

CREATE INDEX group_notifications_unread_idx
    ON group_notifications (group_id)
    WHERE read_at IS NULL;

COMMIT;
