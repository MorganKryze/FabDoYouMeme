# Database Schema Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all database-layer issues found in the 2026-04-09 code review (A3-* issues).

**Architecture:** Add migration `003` to fix FK constraints (host_id ON DELETE SET NULL, invited_by ON DELETE SET NULL, indexes), update the hard-delete sequence to explicitly delete sessions before removing the user, fix `GetUserSubmissions` to filter finished rooms, and define a `SentinelUserID` constant in Go.

**Tech Stack:** PostgreSQL 17, `golang-migrate`, `sqlc`, Go

> **False positives verified:** A3-H2's concern (sessions "silently disappearing") is real but the fix is adding an explicit `DeleteAllUserSessions` call before `HardDeleteUser` — the `GetSessionByTokenHash` INNER JOIN is fine since sessions cascade-delete with the user, but explicit deletion prevents race conditions.

---

### Task 1: Add migration 003 — FK and index fixes

**Covers:** A3-C1 (host_id GDPR), A3-H1 (invited_by), A3-H3 (rooms index), A3-M1 (game_packs index), A3-L2 (down migration comment)

**Files:**
- Create: `backend/db/migrations/003_schema_fixes.up.sql`
- Create: `backend/db/migrations/003_schema_fixes.down.sql`

- [ ] **Step 1: Create the up migration**

```sql
-- backend/db/migrations/003_schema_fixes.up.sql

-- A3-C1: rooms.host_id — allow ON DELETE SET NULL for GDPR hard-delete.
-- Without this, deleting a user who hosted any room (even finished) fails.
-- Making host_id nullable lets rooms survive after their host is erased.
ALTER TABLE rooms
  DROP CONSTRAINT rooms_host_id_fkey,
  ALTER COLUMN host_id DROP NOT NULL,
  ADD CONSTRAINT rooms_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id) ON DELETE SET NULL;

-- A3-H1: users.invited_by — add ON DELETE SET NULL (was RESTRICT by default).
-- All other nullable user FKs already use ON DELETE SET NULL; this aligns them.
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_invited_by_fkey,
  ADD CONSTRAINT users_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL;

-- A3-H3: index on rooms.host_id for FK cascade performance and host queries.
CREATE INDEX ON rooms(host_id);

-- A3-M1: simple non-partial index on game_packs.owner_id for FK cascade.
-- The existing partial index (WHERE deleted_at IS NULL) is not used by the
-- FK engine for ON DELETE SET NULL operations.
CREATE INDEX ON game_packs(owner_id);
```

- [ ] **Step 2: Create the down migration**

```sql
-- backend/db/migrations/003_schema_fixes.down.sql

-- Reverse game_packs index
DROP INDEX IF EXISTS game_packs_owner_id_idx;

-- Reverse rooms index
DROP INDEX IF EXISTS rooms_host_id_idx;

-- Reverse users.invited_by (restore RESTRICT default by dropping explicit constraint;
-- PostgreSQL does not store "RESTRICT" explicitly, re-adding the FK with no ON DELETE
-- clause is equivalent)
ALTER TABLE users
  DROP CONSTRAINT IF EXISTS users_invited_by_fkey,
  ADD CONSTRAINT users_invited_by_fkey
    FOREIGN KEY (invited_by) REFERENCES users(id);

-- Reverse rooms.host_id (restore NOT NULL + RESTRICT)
ALTER TABLE rooms
  DROP CONSTRAINT IF EXISTS rooms_host_id_fkey,
  ALTER COLUMN host_id SET NOT NULL,
  ADD CONSTRAINT rooms_host_id_fkey
    FOREIGN KEY (host_id) REFERENCES users(id);

-- Note: the sentinel row inserted in migration 001 is removed implicitly
-- when the users table is dropped in migration 001's down file. No explicit
-- DELETE is needed here because migration 003 does not touch the sentinel row.
```

- [ ] **Step 3: Run the migration locally to verify syntax**

```bash
cd backend && go build ./...
# If the build passes, run migration (requires local DB):
migrate -path ./db/migrations -database "$DATABASE_URL" up
```

Expected: migration version advances to 3, no errors.

- [ ] **Step 4: Regenerate sqlc (host_id is now nullable)**

```bash
cd backend && sqlc generate
```

Expected: `backend/db/sqlc/rooms.sql.go` regenerated. `Room.HostID` type will change from `uuid.UUID` to `pgtype.UUID`.

- [ ] **Step 5: Fix compilation errors from nullable HostID**

After `sqlc generate`, any code dereferencing `room.HostID` as a non-null UUID will fail to compile. Find all usages:

```bash
cd backend && grep -rn "\.HostID" --include="*.go"
```

In `backend/internal/api/ws.go` and `backend/internal/api/rooms.go`, when reading `room.HostID` to pass to the hub, extract with null-safety:

In `backend/internal/api/ws.go` (ServeHTTP), where the hub is created (in the ws handler or manager call), the hub needs `hostUserID`. If `room.HostID` is now `pgtype.UUID`, extract:

```go
// If host_id is NULL (host was deleted), use empty string — hub will have no host
var hostUserID string
if room.HostID.Valid {
    hostUserID = room.HostID.Bytes.String()
}
```

In `backend/internal/game/hub.go`, `handleGraceExpired` already checks `msg.userID == h.hostUserID` — if hostUserID is `""`, no player will ever match, which is acceptable (no host controls in a room whose host was deleted).

- [ ] **Step 6: Verify build passes**

```bash
cd backend && go build ./... && go vet ./...
```

Expected: no errors.

- [ ] **Step 7: Run tests**

```bash
cd backend && go test -race -count=1 ./...
```

Expected: all tests pass (DB integration tests require a running DB container; those that fail due to missing DB are acceptable in CI without Docker).

- [ ] **Step 8: Commit**

```bash
git add backend/db/migrations/003_schema_fixes.up.sql \
        backend/db/migrations/003_schema_fixes.down.sql \
        backend/db/sqlc/ \
        backend/internal/api/ws.go \
        backend/internal/api/rooms.go
git commit -m "fix(db): migration 003 — host_id ON DELETE SET NULL, invited_by ON DELETE SET NULL, add FK indexes"
```

---

### Task 2: Explicit session deletion before hard-delete

**Covers:** A3-H2

**Files:**
- Modify: `backend/internal/auth/admin.go`

- [ ] **Step 1: Add DeleteAllUserSessions before HardDeleteUser**

In `backend/internal/auth/admin.go`, in the `DeleteUser` handler, add an explicit session deletion before step 5. Currently step 5 relies on CASCADE. Add:

Find the section in admin.go between `UpdateVotesSentinel` and `HardDeleteUser` and insert:

```go
// Step 4b: Explicitly invalidate all sessions before hard delete.
// Although sessions cascade-delete with the user, explicit deletion closes
// the window where GetSessionByTokenHash (INNER JOIN) could race with deletion.
if err := q.DeleteAllUserSessions(r.Context(), targetUUID); err != nil && h.log != nil {
    h.log.Error("delete user: delete sessions", "error", err)
}
```

- [ ] **Step 2: Verify build**

```bash
cd backend && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/auth/admin.go
git commit -m "fix(auth): explicitly delete sessions before hard-delete user"
```

---

### Task 3: Filter GetUserSubmissions to finished rooms

**Covers:** A3-M3

**Files:**
- Modify: `backend/db/queries/users.sql`
- Regenerate: `backend/db/sqlc/` (run `sqlc generate`)

- [ ] **Step 1: Update query**

In `backend/db/queries/users.sql`, find `GetUserSubmissions`:

```sql
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
ORDER BY s.created_at DESC;
```

Change to:

```sql
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
```

- [ ] **Step 2: Regenerate sqlc**

```bash
cd backend && sqlc generate
```

- [ ] **Step 3: Verify build**

```bash
cd backend && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/users.sql backend/db/sqlc/
git commit -m "fix(db): filter GetUserSubmissions to finished rooms only"
```

---

### Task 4: Define SentinelUserID Go constant

**Covers:** A3-L1

**Files:**
- Modify: `backend/db/queries/users.sql` (add comment)
- Create: `backend/internal/auth/sentinel.go`

- [ ] **Step 1: Create sentinel constant**

```go
// backend/internal/auth/sentinel.go
package auth

// SentinelUserID is the UUID of the placeholder user row that replaces hard-deleted
// users in submissions and votes tables (see ADR-006 and migration 001).
// This value matches the SQL literal in db/migrations/001_initial_schema.up.sql.
const SentinelUserID = "00000000-0000-0000-0000-000000000001"
```

- [ ] **Step 2: Add comment to users.sql near sentinel usage**

In `backend/db/queries/users.sql`, add a comment above the sentinel queries:

```sql
-- Sentinel UUID: 00000000-0000-0000-0000-000000000001 (see auth.SentinelUserID in Go).
-- Used to anonymize submissions/votes on hard-delete without breaking referential integrity.

-- name: GetSentinelUser :one
SELECT * FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

-- name: UpdateSubmissionsSentinel :exec
UPDATE submissions SET user_id = '00000000-0000-0000-0000-000000000001' WHERE user_id = $1;

-- name: UpdateVotesSentinel :exec
UPDATE votes SET voter_id = '00000000-0000-0000-0000-000000000001' WHERE voter_id = $1;
```

- [ ] **Step 3: Verify build**

```bash
cd backend && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/auth/sentinel.go backend/db/queries/users.sql
git commit -m "fix(db): define SentinelUserID Go constant, document sentinel usage in SQL"
```

---

### Task 5: Document ConsumeMagicLinkTokenAtomic usage

**Covers:** A3-M2

**Files:**
- Modify: `backend/db/queries/magic_link_tokens.sql`

- [ ] **Step 1: Add documentation comment**

In `backend/db/queries/magic_link_tokens.sql`, add a comment above both query definitions to clarify which is canonical:

```sql
-- ConsumeMagicLinkTokenAtomic is the ONLY query that should be used in production
-- for token consumption. It marks the token used in a single atomic UPDATE, preventing
-- the race condition where two concurrent requests consume the same token.
--
-- ConsumeMagicLinkToken (non-atomic) is retained only for reference; never call it
-- from application code.
```

- [ ] **Step 2: Commit**

```bash
git add backend/db/queries/magic_link_tokens.sql
git commit -m "docs(db): document that ConsumeMagicLinkTokenAtomic is the canonical production query"
```
