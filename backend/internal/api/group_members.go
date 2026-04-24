// backend/internal/api/group_members.go
//
// Phase 1 of the groups paradigm — see
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
//
// Membership lifecycle (List, Kick, Ban, Unban, ListBans, Promote,
// SelfDemote, Leave). Group-admin authority is flat: any admin can do any of
// these, no separate "owner" role. The last-admin guard on Leave and
// SelfDemote enforces the spec invariant that a live group always has at
// least one admin.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

type GroupMemberHandler struct {
	pool *pgxpool.Pool
	db   *db.Queries
	cfg  *config.Config
}

func NewGroupMemberHandler(pool *pgxpool.Pool, cfg *config.Config) *GroupMemberHandler {
	return &GroupMemberHandler{pool: pool, db: db.New(pool), cfg: cfg}
}

// requireAuthAndGroup loads (actor uuid, group id) and verifies the group
// exists. Membership is loaded separately by the caller because the
// non-admin "List members" path doesn't need an admin gate, while every
// other path does.
func (h *GroupMemberHandler) requireAuthAndGroup(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	if _, err := h.db.GetGroupByID(r.Context(), gid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return uuid.Nil, uuid.Nil, false
	}
	return uid, gid, true
}

// requireMembership loads the (gid, uid) membership and writes 403 when
// absent. Returns the row so callers can inspect the role without a second
// query.
func (h *GroupMemberHandler) requireMembership(w http.ResponseWriter, r *http.Request, gid, uid uuid.UUID) (db.GroupMembership, bool) {
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load membership")
		}
		return db.GroupMembership{}, false
	}
	return mem, true
}

func (h *GroupMemberHandler) requireAdminMembership(w http.ResponseWriter, r *http.Request, gid, uid uuid.UUID) (db.GroupMembership, bool) {
	mem, ok := h.requireMembership(w, r, gid, uid)
	if !ok {
		return db.GroupMembership{}, false
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return db.GroupMembership{}, false
	}
	return mem, true
}

// parseTargetUserID reads {userID} from the chi URL.
func parseTargetUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	tid, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user id")
		return uuid.Nil, false
	}
	return tid, true
}

// ─── List ────────────────────────────────────────────────────────────────────

// List handles GET /api/groups/{id}/members — visible to any member.
func (h *GroupMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireMembership(w, r, gid, uid); !ok {
		return
	}
	rows, err := h.db.ListGroupMembers(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list members")
		return
	}
	if rows == nil {
		rows = []db.ListGroupMembersRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// ─── Kick ────────────────────────────────────────────────────────────────────

// Kick handles DELETE /api/groups/{id}/members/{userID} — admin only,
// cannot target self (use Leave for self-departure so the last-admin guard
// applies).
func (h *GroupMemberHandler) Kick(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}
	target, ok := parseTargetUserID(w, r)
	if !ok {
		return
	}
	if target == uid {
		writeError(w, r, http.StatusConflict, "cannot_kick_self",
			"Use the leave endpoint instead of kicking yourself.")
		return
	}
	// Verify the target is actually a member so we return 404 instead of
	// silently 204'ing on a no-op delete.
	if _, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: target}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "not_group_member", "Target is not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load target membership")
		}
		return
	}
	if err := h.db.DeleteMembership(r.Context(), db.DeleteMembershipParams{
		GroupID: gid, UserID: target,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to kick member")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.kick_member",
		"group:"+gid.String(), map[string]string{"target_user_id": target.String()})
	w.WriteHeader(http.StatusNoContent)
}

// ─── Ban + Unban + ListBans ──────────────────────────────────────────────────

type banRequest struct {
	UserID string `json:"user_id"`
}

// Ban handles POST /api/groups/{id}/bans — admin only. Removes the
// membership and inserts a ban row in one transaction so a partial failure
// can never leave the user un-banned but with an active membership (or vice
// versa).
func (h *GroupMemberHandler) Ban(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}
	var req banRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	target, err := uuid.Parse(req.UserID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user id")
		return
	}
	if target == uid {
		writeError(w, r, http.StatusConflict, "cannot_kick_self", "You cannot ban yourself.")
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	qtx := h.db.WithTx(tx)

	if err := qtx.DeleteMembership(r.Context(), db.DeleteMembershipParams{
		GroupID: gid, UserID: target,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to remove membership")
		return
	}
	if err := qtx.CreateBan(r.Context(), db.CreateBanParams{
		GroupID:  gid,
		UserID:   target,
		BannedBy: pgtype.UUID{Bytes: uid, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to record ban")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.ban_member",
		"group:"+gid.String(), map[string]string{"target_user_id": target.String()})
	w.WriteHeader(http.StatusNoContent)
}

// Unban handles DELETE /api/groups/{id}/bans/{userID} — admin only.
func (h *GroupMemberHandler) Unban(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}
	target, ok := parseTargetUserID(w, r)
	if !ok {
		return
	}
	if err := h.db.DeleteBan(r.Context(), db.DeleteBanParams{GroupID: gid, UserID: target}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to remove ban")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.unban_member",
		"group:"+gid.String(), map[string]string{"target_user_id": target.String()})
	w.WriteHeader(http.StatusNoContent)
}

// ListBans handles GET /api/groups/{id}/bans — admin only.
func (h *GroupMemberHandler) ListBans(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}
	rows, err := h.db.ListGroupBans(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list bans")
		return
	}
	if rows == nil {
		rows = []db.ListGroupBansRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// ─── Promote ─────────────────────────────────────────────────────────────────

// Promote handles POST /api/groups/{id}/members/{userID}/promote — admin
// only. Idempotent — promoting an existing admin is a 200 no-op.
func (h *GroupMemberHandler) Promote(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}
	target, ok := parseTargetUserID(w, r)
	if !ok {
		return
	}
	updated, err := h.db.UpdateMembershipRole(r.Context(), db.UpdateMembershipRoleParams{
		GroupID: gid, UserID: target, Role: "admin",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "not_group_member", "Target is not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to promote member")
		}
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.promote_member",
		"group:"+gid.String(), map[string]string{"target_user_id": target.String()})
	writeJSON(w, http.StatusOK, updated)
}

// ─── SelfDemote ──────────────────────────────────────────────────────────────

// SelfDemote handles POST /api/groups/{id}/members/self/demote. Admin steps
// down to member; rejected when the actor is the sole admin (use Leave or
// Delete for those cases).
func (h *GroupMemberHandler) SelfDemote(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	mem, ok := h.requireMembership(w, r, gid, uid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusConflict, "not_group_admin", "Already a member.")
		return
	}
	adminCount, err := h.db.CountGroupAdmins(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count admins")
		return
	}
	if adminCount <= 1 {
		writeError(w, r, http.StatusConflict, "last_admin_cannot_leave",
			"Promote another member before demoting yourself.")
		return
	}
	updated, err := h.db.UpdateMembershipRole(r.Context(), db.UpdateMembershipRoleParams{
		GroupID: gid, UserID: uid, Role: "member",
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to demote")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ─── Leave ───────────────────────────────────────────────────────────────────

// Leave handles DELETE /api/groups/{id}/members/self. The last-admin guard
// blocks departure when the actor is the sole admin — they must promote a
// peer or delete the group first.
func (h *GroupMemberHandler) Leave(w http.ResponseWriter, r *http.Request) {
	uid, gid, ok := h.requireAuthAndGroup(w, r)
	if !ok {
		return
	}
	mem, ok := h.requireMembership(w, r, gid, uid)
	if !ok {
		return
	}
	if mem.Role == "admin" {
		adminCount, err := h.db.CountGroupAdmins(r.Context(), gid)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count admins")
			return
		}
		if adminCount <= 1 {
			writeError(w, r, http.StatusConflict, "last_admin_cannot_leave",
				"Promote another member or delete the group before leaving.")
			return
		}
	}
	if err := h.db.DeleteMembership(r.Context(), db.DeleteMembershipParams{
		GroupID: gid, UserID: uid,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to leave group")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
