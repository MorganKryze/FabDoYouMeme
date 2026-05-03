// backend/internal/auth/admin.go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// adminMagicLinkCooldown bounds how often an admin can force-resend a login
// magic link to the same target. Distinct from the public 60s cooldown on
// /api/auth/magic-link: that one prevents enumeration by anonymous callers,
// this one only protects the SMTP relay from a hammering admin.
const adminMagicLinkCooldown = 15 * time.Second

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

// SendMagicLink handles POST /api/admin/users/{id}/magic-link.
// Issues a fresh login magic link to the target user, bypassing the public
// per-email cooldown (see auth.MagicLink). Per-target 15s cooldown applied here.
func (h *Handler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	admin, ok := middleware.GetSessionUser(r)
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
	if targetID == sentinelUserID {
		writeError(w, r, http.StatusConflict, "cannot_send_to_sentinel",
			"Cannot send a magic link to the sentinel user")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), targetUUID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "user_not_found", "User not found")
		return
	}
	if !user.IsActive {
		writeError(w, r, http.StatusConflict, "user_inactive",
			"Cannot send a magic link to a deactivated account")
		return
	}

	if createdAt, err := h.db.GetLatestMagicLinkToken(r.Context(),
		db.GetLatestMagicLinkTokenParams{UserID: user.ID, Purpose: "login"}); err == nil {
		elapsed := h.clock.Now().Sub(createdAt)
		if elapsed < adminMagicLinkCooldown {
			retryAfter := int((adminMagicLinkCooldown - elapsed).Seconds()) + 1
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":        "cooldown_active",
				"error":       "Please wait before resending",
				"retry_after": retryAfter,
			})
			return
		}
	}

	if err := h.sendMagicLinkToUserAsync(r.Context(), user, "login"); err != nil {
		if h.log != nil {
			h.log.Error("admin force magic link: token persist failed",
				"error", err, "user_id", user.ID)
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to send magic link")
		return
	}

	// Best-effort audit log. Email is masked in line with /api/admin/* policy
	// of avoiding raw PII in the changes blob; the resource line carries the
	// stable user ID for cross-reference.
	adminUUID, _ := uuid.Parse(admin.UserID)
	changes, _ := json.Marshal(map[string]string{"email": maskEmail(user.Email)})
	if _, auditErr := h.db.CreateAuditLog(r.Context(), db.CreateAuditLogParams{
		AdminID:  pgtype.UUID{Bytes: adminUUID, Valid: true},
		Action:   "admin_force_magic_link",
		Resource: fmt.Sprintf("user:%s", targetID),
		Changes:  json.RawMessage(changes),
	}); auditErr != nil && h.log != nil {
		h.log.Warn("admin force magic link: audit log failed", "error", auditErr)
	}

	w.WriteHeader(http.StatusNoContent)
}
