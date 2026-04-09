// backend/internal/auth/sentinel.go
package auth

// SentinelUserID is the UUID of the placeholder user row that replaces hard-deleted
// users in submissions and votes tables (see ADR-006 and migration 001).
// This value matches the SQL literal in db/migrations/001_initial_schema.up.sql.
const SentinelUserID = "00000000-0000-0000-0000-000000000001"
