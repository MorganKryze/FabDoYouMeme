// backend/internal/auth/admin.go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// sentinelUserID is the package-local alias for SentinelUserID, used for guard checks.
const sentinelUserID = SentinelUserID

// DeleteUser handles DELETE /api/admin/users/:id.
// Runs the 5-step hard-delete protocol in a single transaction.
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	adminUser, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	targetID := chi.URLParam(r, "id")
	targetUUID, err := uuid.Parse(targetID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	// Guard: cannot delete the sentinel user
	if targetID == sentinelUserID {
		writeError(w, r, http.StatusConflict, "cannot_delete_sentinel", "The sentinel user cannot be deleted")
		return
	}

	// Guard: cannot delete yourself
	if targetID == adminUser.UserID {
		writeError(w, r, http.StatusConflict, "cannot_delete_self", "Cannot delete your own account")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Transaction failed")
		return
	}
	defer tx.Rollback(r.Context())

	q := h.db.WithTx(tx)

	// Step 1: Capture PII before deletion (audit trail requires it)
	target, err := q.GetUserByID(r.Context(), targetUUID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	// Guard: bootstrap admin cannot be hard-deleted. This runs after the
	// fetch so we know the flag with certainty, and before the audit log
	// so we don't record a failed attempt as a hard_delete_user entry.
	if target.IsProtected {
		writeError(w, r, http.StatusConflict, "cannot_delete_protected_user",
			"The bootstrap admin cannot be deleted")
		return
	}

	// Step 2: Write audit log (PII recorded before delete so log is meaningful)
	adminUUID, _ := uuid.Parse(adminUser.UserID)
	changes, _ := json.Marshal(map[string]string{
		"username": target.Username,
		"email":    target.Email,
	})
	if _, err := q.CreateAuditLog(r.Context(), db.CreateAuditLogParams{
		AdminID:  pgtype.UUID{Bytes: adminUUID, Valid: true},
		Action:   "hard_delete_user",
		Resource: fmt.Sprintf("user:%s", targetID),
		Changes:  json.RawMessage(changes),
	}); err != nil {
		if h.log != nil {
			h.log.Error("delete user: audit log", "error", err)
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Audit log failed")
		return
	}

	// Step 3: Replace submissions with sentinel (preserve game history integrity).
	// After migration 004 made submissions.user_id nullable (to support
	// guest participants), the sqlc param is pgtype.UUID — we wrap the
	// target user UUID with Valid: true because this path only ever runs
	// against a registered user.
	targetPG := pgtype.UUID{Bytes: targetUUID, Valid: true}
	if err := q.UpdateSubmissionsSentinel(r.Context(), targetPG); err != nil && h.log != nil {
		h.log.Error("delete user: submissions sentinel", "error", err)
	}

	// Step 4: Replace votes with sentinel (preserve vote history integrity)
	if err := q.UpdateVotesSentinel(r.Context(), targetPG); err != nil && h.log != nil {
		h.log.Error("delete user: votes sentinel", "error", err)
	}

	// Step 4b: Explicitly invalidate all sessions before hard delete.
	// Although sessions cascade-delete with the user, explicit deletion closes
	// the window where GetSessionByTokenHash (INNER JOIN) could race with deletion.
	if err := q.DeleteAllUserSessions(r.Context(), targetUUID); err != nil && h.log != nil {
		h.log.Error("delete user: delete sessions", "error", err)
	}

	// Step 5: Delete user — cascades: sessions, magic_link_tokens, room_players
	//           sets NULL on: invites.created_by, audit_logs.admin_id, game_packs.owner_id
	if err := q.HardDeleteUser(r.Context(), targetUUID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Transaction commit failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
