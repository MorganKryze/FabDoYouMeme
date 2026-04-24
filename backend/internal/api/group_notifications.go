// backend/internal/api/group_notifications.go
//
// Phase 5 of the groups paradigm. Group admins read their per-group feed
// via this handler; writes are produced by the various handlers that
// create the events (group_packs eviction, group_members kick/ban, the
// duplication service, groupjobs auto-promotion).
package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

type GroupNotificationHandler struct {
	db  *db.Queries
	cfg *config.Config
}

func NewGroupNotificationHandler(pool *pgxpool.Pool, cfg *config.Config) *GroupNotificationHandler {
	return &GroupNotificationHandler{db: db.New(pool), cfg: cfg}
}

// List handles GET /api/groups/{id}/notifications — admin-only, paginated
// via the shared lim/off pattern. The `unread` query param requests only
// unread rows when set to 1; otherwise every row newest-first.
func (h *GroupNotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load membership")
		}
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	limit, offset := parsePagination(r)
	rows, err := h.db.ListGroupNotifications(r.Context(), db.ListGroupNotificationsParams{
		GroupID: gid, Lim: limit, Off: offset,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list notifications")
		return
	}
	if rows == nil {
		rows = []db.GroupNotification{}
	}
	unread, _ := h.db.CountUnreadGroupNotifications(r.Context(), gid)
	writeJSON(w, http.StatusOK, map[string]any{
		"data":   rows,
		"unread": unread,
	})
}

// MarkRead handles PATCH /api/groups/{id}/notifications/{nid}/read.
func (h *GroupNotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: uid})
	if err != nil {
		writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group.")
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	nid, err := uuid.Parse(chi.URLParam(r, "nid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid notification id")
		return
	}
	if err := h.db.MarkGroupNotificationRead(r.Context(), db.MarkGroupNotificationReadParams{
		ID: nid, GroupID: gid,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to mark read")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
