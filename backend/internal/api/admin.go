// backend/internal/api/admin.go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// AdminHandler handles /api/admin/* routes (users, invites, notifications).
type AdminHandler struct {
	db *db.Queries
}

func NewAdminHandler(pool *pgxpool.Pool) *AdminHandler {
	return &AdminHandler{db: db.New(pool)}
}

// ListUsers handles GET /api/admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, offset := parsePagination(r)
	users, err := h.db.ListUsers(r.Context(), db.ListUsersParams{
		Search: &q, Lim: int32(limit), Off: int32(offset),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list users")
		return
	}
	total, _ := h.db.CountUsers(r.Context(), &q)
	writeJSON(w, http.StatusOK, map[string]any{
		"data":        users,
		"total":       total,
		"next_cursor": nextCursor(len(users), limit, offset),
	})
}

// UpdateUser handles PATCH /api/admin/users/:id.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}
	var req struct {
		Role     *string `json:"role,omitempty"`
		IsActive *bool   `json:"is_active,omitempty"`
	}
	if decodeErr := decodeJSON(r, &req); decodeErr != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Role != nil {
		user, err := h.db.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{
			ID: targetID, Role: *req.Role,
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		writeJSON(w, http.StatusOK, user)
		return
	}
	if req.IsActive != nil {
		user, err := h.db.SetUserActive(r.Context(), db.SetUserActiveParams{
			ID: targetID, IsActive: *req.IsActive,
		})
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
			return
		}
		writeJSON(w, http.StatusOK, user)
		return
	}
	writeError(w, r, http.StatusBadRequest, "bad_request", "Provide role or is_active to update")
}

// ListInvites handles GET /api/admin/invites.
func (h *AdminHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	invites, err := h.db.ListInvites(r.Context(), db.ListInvitesParams{
		Lim: int32(limit), Off: int32(offset),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list invites")
		return
	}
	total, _ := h.db.CountInvites(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"data": invites, "total": total,
		"next_cursor": nextCursor(len(invites), limit, offset),
	})
}

// CreateInvite handles POST /api/admin/invites.
func (h *AdminHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var req struct {
		Token           string  `json:"token"`
		Label           *string `json:"label,omitempty"`
		RestrictedEmail *string `json:"restricted_email,omitempty"`
		MaxUses         int32   `json:"max_uses"`
		ExpiresAt       *string `json:"expires_at,omitempty"`
	}
	if decodeErr := decodeJSON(r, &req); decodeErr != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Token == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "token is required")
		return
	}

	// Parse optional expires_at (RFC3339) into pgtype.Timestamptz
	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "bad_request", "expires_at must be RFC3339")
			return
		}
		expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
	}

	creatorID, _ := uuid.Parse(u.UserID)
	invite, err := h.db.CreateInvite(r.Context(), db.CreateInviteParams{
		Token:           req.Token,
		CreatedBy:       pgtype.UUID{Bytes: creatorID, Valid: true},
		Label:           req.Label,
		RestrictedEmail: req.RestrictedEmail,
		MaxUses:         req.MaxUses,
		ExpiresAt:       expiresAt,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create invite")
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

// DeleteInvite handles DELETE /api/admin/invites/:id.
func (h *AdminHandler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	inviteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid invite ID")
		return
	}
	if err := h.db.DeleteInvite(r.Context(), inviteID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListNotifications handles GET /api/admin/notifications.
func (h *AdminHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	unreadOnly := r.URL.Query().Get("unread") == "true"
	notifications, err := h.db.ListAdminNotifications(r.Context(), db.ListAdminNotificationsParams{
		UnreadOnly: unreadOnly,
		Lim:        int32(limit),
		Off:        int32(offset),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list notifications")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":        notifications,
		"next_cursor": nextCursor(len(notifications), limit, offset),
	})
}

// MarkNotificationRead handles PATCH /api/admin/notifications/:id.
func (h *AdminHandler) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	notifID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid notification ID")
		return
	}
	n, err := h.db.MarkNotificationRead(r.Context(), notifID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
