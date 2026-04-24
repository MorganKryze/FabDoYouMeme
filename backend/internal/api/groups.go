// backend/internal/api/groups.go
//
// Groups paradigm CRUD surface. Spec:
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// defaultGroupQuotaBytes is the per-group asset cap applied at creation
// time. Platform admins raise it on demand via SetGroupQuotaBytes (phase 5).
// 500 MB matches the spec's starting floor.
const defaultGroupQuotaBytes int64 = 500 * 1024 * 1024

// defaultGroupMemberCap is the spec's starting member ceiling. Per-group
// override is a phase-5 admin surface; the column already exists so the
// override is a single UPDATE when we ship that flow.
const defaultGroupMemberCap int32 = 100

type GroupHandler struct {
	pool *pgxpool.Pool
	db   *db.Queries
	cfg  *config.Config
}

func NewGroupHandler(pool *pgxpool.Pool, cfg *config.Config) *GroupHandler {
	return &GroupHandler{pool: pool, db: db.New(pool), cfg: cfg}
}

// requireSessionUUID parses the session user's id into a uuid.UUID.
// Returns uuid.Nil + false on missing session (handler should already have
// called ensureFeatureEnabled and is mounted under RequireAuth, so this is a
// belt-and-braces guard).
func requireSessionUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(u.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Malformed session")
		return uuid.Nil, false
	}
	return uid, true
}

// parseGroupID reads {id} from the chi URL and writes 400 on failure.
func parseGroupID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	gid, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid group id")
		return uuid.Nil, false
	}
	return gid, true
}

// requireMembership loads the membership row for (gid, uid). Writes 403
// not_group_member when absent. The bool return tells the caller whether to
// proceed; the membership is returned so callers can branch on role without
// a second round-trip.
func (h *GroupHandler) requireMembership(w http.ResponseWriter, r *http.Request, gid, uid uuid.UUID) (db.GroupMembership, bool) {
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{
		GroupID: gid, UserID: uid,
	})
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

// requireAdminMembership extends requireMembership with a role check.
func (h *GroupHandler) requireAdminMembership(w http.ResponseWriter, r *http.Request, gid, uid uuid.UUID) (db.GroupMembership, bool) {
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

// validateLanguage returns true iff the value is in the platform-known set.
func validateGroupLanguage(s string) bool {
	return s == "en" || s == "fr" || s == "multi"
}

// validateClassification returns true iff the value is sfw or nsfw.
func validateGroupClassification(s string) bool {
	return s == "sfw" || s == "nsfw"
}

// nameTaken returns true iff a live (non-soft-deleted) group with the
// case-insensitive name already exists. excludeID skips the row with that id
// so Update can call this against the group it's mutating without
// false-positive collision against itself.
func (h *GroupHandler) nameTaken(r *http.Request, normalized string, excludeID uuid.UUID) (bool, error) {
	row, err := h.db.GetGroupByNormalizedName(r.Context(), &normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if row.ID == excludeID {
		return false, nil
	}
	return true, nil
}

// ─── Create ──────────────────────────────────────────────────────────────────

type createGroupRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Language       string  `json:"language"`
	Classification string  `json:"classification"`
	AvatarMediaKey *string `json:"avatar_media_key,omitempty"`
}

// Create handles POST /api/groups.
//
// The creator becomes the first admin in the same transaction as the group
// row, so we never end up with a structurally-impossible zero-admin group.
func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.Description == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "description is required")
		return
	}
	if len(req.Description) > 500 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "description must be at most 500 characters")
		return
	}
	if !validateGroupLanguage(req.Language) {
		writeError(w, r, http.StatusBadRequest, "bad_request", "language must be en, fr, or multi")
		return
	}
	if !validateGroupClassification(req.Classification) {
		writeError(w, r, http.StatusBadRequest, "bad_request", "classification must be sfw or nsfw")
		return
	}

	count, err := h.db.CountCreatedGroupsForUser(r.Context(), pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check group cap")
		return
	}
	if int(count) >= h.cfg.MaxGroupsPerUser {
		writeError(w, r, http.StatusConflict, "group_cap_reached",
			"You have reached the maximum number of groups you can create.")
		return
	}

	taken, err := h.nameTaken(r, strings.ToLower(req.Name), uuid.Nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check name uniqueness")
		return
	}
	if taken {
		writeError(w, r, http.StatusConflict, "group_name_taken", "A group with this name already exists.")
		return
	}

	var avatar *string
	if req.AvatarMediaKey != nil {
		trimmed := strings.TrimSpace(*req.AvatarMediaKey)
		if trimmed != "" {
			avatar = &trimmed
		}
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	qtx := h.db.WithTx(tx)

	group, err := qtx.CreateGroup(r.Context(), db.CreateGroupParams{
		Name:           req.Name,
		Description:    req.Description,
		Language:       req.Language,
		Classification: req.Classification,
		QuotaBytes:     defaultGroupQuotaBytes,
		CreatedBy:      pgtype.UUID{Bytes: uid, Valid: true},
		AvatarMediaKey: avatar,
		MemberCap:      defaultGroupMemberCap,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create group")
		return
	}

	if _, err := qtx.CreateMembership(r.Context(), db.CreateMembershipParams{
		GroupID: group.ID,
		UserID:  uid,
		Role:    "admin",
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create membership")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

// ─── List + Get ──────────────────────────────────────────────────────────────

// List handles GET /api/groups — every live group the caller is a member of.
func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	rows, err := h.db.ListGroupsForUser(r.Context(), uid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list groups")
		return
	}
	if rows == nil {
		rows = []db.ListGroupsForUserRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// Get handles GET /api/groups/{id} — visible only to members.
func (h *GroupHandler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}

	group, err := h.db.GetGroupByID(r.Context(), gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	if _, ok := h.requireMembership(w, r, gid, uid); !ok {
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// ─── Update ──────────────────────────────────────────────────────────────────

type updateGroupRequest struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	Language       *string `json:"language,omitempty"`
	Classification *string `json:"classification,omitempty"`
	// AvatarSet=true means the caller explicitly sent avatar_media_key
	// (including null to clear). When false the column is left untouched.
	AvatarSet      bool    `json:"avatar_set"`
	AvatarMediaKey *string `json:"avatar_media_key,omitempty"`
}

// Update handles PATCH /api/groups/{id} — admin only.
func (h *GroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}

	if _, err := h.db.GetGroupByID(r.Context(), gid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}

	var req updateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			writeError(w, r, http.StatusBadRequest, "bad_request", "name cannot be empty")
			return
		}
		taken, err := h.nameTaken(r, strings.ToLower(trimmed), gid)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to check name uniqueness")
			return
		}
		if taken {
			writeError(w, r, http.StatusConflict, "group_name_taken", "A group with this name already exists.")
			return
		}
		req.Name = &trimmed
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if trimmed == "" || len(trimmed) > 500 {
			writeError(w, r, http.StatusBadRequest, "bad_request", "description must be 1 to 500 characters")
			return
		}
		req.Description = &trimmed
	}
	if req.Language != nil && !validateGroupLanguage(*req.Language) {
		writeError(w, r, http.StatusBadRequest, "bad_request", "language must be en, fr, or multi")
		return
	}
	if req.Classification != nil && !validateGroupClassification(*req.Classification) {
		writeError(w, r, http.StatusBadRequest, "bad_request", "classification must be sfw or nsfw")
		return
	}

	var avatar *string
	if req.AvatarSet && req.AvatarMediaKey != nil {
		trimmed := strings.TrimSpace(*req.AvatarMediaKey)
		if trimmed != "" {
			avatar = &trimmed
		}
	}

	updated, err := h.db.UpdateGroup(r.Context(), db.UpdateGroupParams{
		ID:             gid,
		Name:           req.Name,
		Description:    req.Description,
		Language:       req.Language,
		Classification: req.Classification,
		AvatarSet:      req.AvatarSet,
		AvatarMediaKey: avatar,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to update group")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// ─── Delete + Restore ────────────────────────────────────────────────────────

// Delete handles DELETE /api/groups/{id} — soft-delete with a 30-day window.
func (h *GroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}

	if _, err := h.db.GetGroupByID(r.Context(), gid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}

	if err := h.db.SoftDeleteGroup(r.Context(), gid); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete group")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.soft_delete",
		"group:"+gid.String(), nil)
	w.WriteHeader(http.StatusNoContent)
}

// Restore handles POST /api/groups/{id}/restore — admin only, in-window only.
//
// The query's WHERE clause already enforces the 30-day window. We re-read
// after the UPDATE: if the row is still soft-deleted the window had elapsed,
// and we surface 410 Gone so the client can distinguish "expired" from
// "transient failure".
func (h *GroupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return
	}
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}

	// Membership predates the soft-delete; load the row through the
	// "including deleted" lookup so we can authz before we attempt restore.
	row, err := h.db.GetGroupByIDIncludingDeleted(r.Context(), gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "group_not_found", "Group not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	if !row.DeletedAt.Valid {
		writeError(w, r, http.StatusConflict, "group_not_deleted", "Group is not in a deleted state.")
		return
	}
	if _, ok := h.requireAdminMembership(w, r, gid, uid); !ok {
		return
	}

	if err := h.db.RestoreGroup(r.Context(), gid); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to restore group")
		return
	}

	restored, err := h.db.GetGroupByID(r.Context(), gid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusGone, "group_restore_window_elapsed",
				"The 30-day restore window has elapsed.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load group")
		}
		return
	}
	writeJSON(w, http.StatusOK, restored)
}
