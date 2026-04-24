// backend/internal/api/admin_groups.go
//
// Phase 5 of the groups paradigm — platform-admin overview of every group on
// the instance + per-group quota and member-cap overrides. Separate from
// AdminHandler so tests can instantiate it in isolation; mounted under the
// existing /api/admin route group in main.go.
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
)

type AdminGroupsHandler struct {
	db *db.Queries
}

func NewAdminGroupsHandler(pool *pgxpool.Pool) *AdminGroupsHandler {
	return &AdminGroupsHandler{db: db.New(pool)}
}

// List handles GET /api/admin/groups — every group on the instance, newest
// first. Includes soft-deleted rows; the UI filters as it likes. Each row
// carries the member count so the admin can spot dormant groups at a
// glance.
func (h *AdminGroupsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	rows, err := h.db.ListAllGroupsAdmin(r.Context(), db.ListAllGroupsAdminParams{
		Lim: limit, Off: offset,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list groups")
		return
	}
	if rows == nil {
		rows = []db.ListAllGroupsAdminRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":        rows,
		"next_cursor": nextCursor(len(rows), limit, offset),
	})
}

type setQuotaRequest struct {
	QuotaBytes int64 `json:"quota_bytes"`
}

// SetQuota handles PATCH /api/admin/groups/{gid}/quota.
func (h *AdminGroupsHandler) SetQuota(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "gid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid group id")
		return
	}
	var req setQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.QuotaBytes < 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "quota_bytes must be at least 0")
		return
	}
	// Verify the group exists + is not soft-deleted so we can surface 404
	// separately from a successful-but-no-op update.
	if _, err := h.db.GetGroupByID(r.Context(), gid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	if err := h.db.SetGroupQuotaBytes(r.Context(), db.SetGroupQuotaBytesParams{
		ID: gid, QuotaBytes: req.QuotaBytes,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update quota")
		return
	}
	updated, _ := h.db.GetGroupByID(r.Context(), gid)
	writeJSON(w, http.StatusOK, updated)
}

type setMemberCapRequest struct {
	MemberCap int32 `json:"member_cap"`
}

// SetMemberCap handles PATCH /api/admin/groups/{gid}/member_cap.
func (h *AdminGroupsHandler) SetMemberCap(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "gid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid group id")
		return
	}
	var req setMemberCapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.MemberCap <= 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "member_cap must be positive")
		return
	}
	// Refuse to set the cap below current member count — the existing
	// members don't evaporate, so the constraint would be non-enforceable
	// and just confuse the admin.
	count, err := h.db.CountGroupMembers(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count members")
		return
	}
	if int64(req.MemberCap) < count {
		writeError(w, r, http.StatusConflict, "member_cap_below_current",
			"Cannot set member_cap below the current member count.")
		return
	}
	if err := h.db.SetGroupMemberCap(r.Context(), db.SetGroupMemberCapParams{
		ID: gid, MemberCap: req.MemberCap,
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update member cap")
		return
	}
	updated, _ := h.db.GetGroupByID(r.Context(), gid)
	writeJSON(w, http.StatusOK, updated)
}

// Get handles GET /api/admin/groups/{gid} — returns a single group row
// regardless of soft-delete state so the admin detail page has the full
// picture. Uses GetGroupByIDIncludingDeleted.
func (h *AdminGroupsHandler) Get(w http.ResponseWriter, r *http.Request) {
	gid, err := uuid.Parse(chi.URLParam(r, "gid"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid group id")
		return
	}
	row, err := h.db.GetGroupByIDIncludingDeleted(r.Context(), gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	memberCount, _ := h.db.CountGroupMembers(r.Context(), gid)
	writeJSON(w, http.StatusOK, map[string]any{
		"group":        row,
		"member_count": memberCount,
	})
	_ = pgtype.UUID{} // keep import — compatibility with other admin handlers
}
