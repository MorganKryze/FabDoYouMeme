// backend/internal/api/admin.go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// writeAuditLog is a best-effort admin audit write. Failures are intentionally
// swallowed here (unlike auth.DeleteUser which must fail the request) because
// the calling admin mutations are lower-severity than a GDPR hard-delete —
// finding 5.G in the 2026-04-10 review. Callers should still provide an
// accurate action/resource/changes payload.
func writeAuditLog(ctx context.Context, q *db.Queries, adminUserID, action, resource string, changes any) {
	adminUUID, err := uuid.Parse(adminUserID)
	if err != nil {
		return
	}
	body, _ := json.Marshal(changes)
	q.CreateAuditLog(ctx, db.CreateAuditLogParams{ //nolint:errcheck
		AdminID:  pgtype.UUID{Bytes: adminUUID, Valid: true},
		Action:   action,
		Resource: resource,
		Changes:  json.RawMessage(body),
	})
}

// AdminHandler handles /api/admin/* routes (users, invites, notifications).
type AdminHandler struct {
	db    *db.Queries
	store storage.Storage
}

func NewAdminHandler(pool *pgxpool.Pool, store storage.Storage) *AdminHandler {
	return &AdminHandler{db: db.New(pool), store: store}
}

// ListUsers handles GET /api/admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, offset := parsePagination(r)
	users, err := h.db.ListUsers(r.Context(), db.ListUsersParams{
		Search: &q, Lim: limit, Off: offset,
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
	admin, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
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
	// Fetch target up-front so we can enforce the protection guard before
	// any write. Cheap — a single indexed lookup — and it short-circuits
	// every mutation branch below. We reject protection-violating requests
	// for any mutable field (role, is_active), not just role: deactivating
	// the bootstrap admin is operationally equivalent to deleting it
	// because the admin surface stops being reachable.
	target, err := h.db.GetUserByID(r.Context(), targetID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "user_not_found", "User not found")
		return
	}
	if target.IsProtected && (req.Role != nil || req.IsActive != nil) {
		writeError(w, r, http.StatusConflict, "cannot_modify_protected_user",
			"The bootstrap admin's role and active status cannot be changed")
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
		writeAuditLog(r.Context(), h.db, admin.UserID, "update_user_role",
			fmt.Sprintf("user:%s", targetID),
			map[string]string{"new_role": *req.Role})
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
		writeAuditLog(r.Context(), h.db, admin.UserID, "set_user_active",
			fmt.Sprintf("user:%s", targetID),
			map[string]bool{"is_active": *req.IsActive})
		writeJSON(w, http.StatusOK, user)
		return
	}
	writeError(w, r, http.StatusBadRequest, "bad_request", "Provide role or is_active to update")
}

// ListInvites handles GET /api/admin/invites.
func (h *AdminHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	invites, err := h.db.ListInvites(r.Context(), db.ListInvitesParams{
		Lim: limit, Off: offset,
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
		t, genErr := auth.GenerateRawToken()
		if genErr != nil {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to generate invite token")
			return
		}
		req.Token = t
	}
	if req.MaxUses < 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "max_uses must be >= 0")
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
	admin, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	inviteID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid invite ID")
		return
	}
	if err := h.db.DeleteInvite(r.Context(), inviteID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
		return
	}
	writeAuditLog(r.Context(), h.db, admin.UserID, "revoke_invite",
		fmt.Sprintf("invite:%s", inviteID), nil)
	w.WriteHeader(http.StatusNoContent)
}

// ListNotifications handles GET /api/admin/notifications.
func (h *AdminHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	unreadOnly := r.URL.Query().Get("unread") == "true"
	notifications, err := h.db.ListAdminNotifications(r.Context(), db.ListAdminNotificationsParams{
		UnreadOnly: unreadOnly,
		Lim:        limit,
		Off:        offset,
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

// GetStats handles GET /api/admin/stats. Returns the four counters rendered
// in the admin dashboard hero row. Counts are computed per-request — at this
// scale the four COUNT(*) queries are cheap enough that caching would add
// complexity without measurable benefit. The pack count now lives in the
// storage widget (GET /api/admin/storage); the hero row surfaces games
// played instead so operators get a "lifetime activity" signal alongside
// the live counters.
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	activeRooms, err := h.db.CountActiveRooms(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count rooms")
		return
	}
	totalUsers, err := h.db.CountUsers(ctx, strPtrEmpty())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count users")
		return
	}
	gamesPlayed, err := h.db.CountFinishedRooms(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count games played")
		return
	}
	pendingInvites, err := h.db.CountPendingInvites(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count invites")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{
		"active_rooms":    activeRooms,
		"total_users":     totalUsers,
		"games_played":    gamesPlayed,
		"pending_invites": pendingInvites,
	})
}

// strPtrEmpty returns a pointer to the empty string — CountUsers takes
// *string as its search filter and interprets nil/empty as "no filter".
func strPtrEmpty() *string { s := ""; return &s }

// GetStorageStats handles GET /api/admin/storage. Returns the summary shown
// in the dashboard storage widget: pack count from the DB, and object count
// plus total bytes from a live walk of the RustFS bucket.
//
// The bucket walk is intentionally kept out of GetStats (which is four cheap
// COUNT(*) queries) so the hero-row refresh path stays fast; this endpoint
// is only hit when the dashboard renders the widget, and scales linearly
// with object count.
func (h *AdminHandler) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	packsCount, err := h.db.CountAllPacks(ctx)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to count packs")
		return
	}
	objectCount, totalBytes, err := h.store.Stats(ctx, "")
	if err != nil {
		writeError(w, r, http.StatusBadGateway, "storage_unreachable", "Failed to read storage stats")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{
		"packs_count":  packsCount,
		"assets_count": objectCount,
		"total_bytes":  totalBytes,
	})
}

// auditEntry is the wire shape for GET /api/admin/audit. We flatten the
// sqlc row so the frontend doesn't have to understand pgtype.UUID's
// {bytes, valid} envelope, and we split the `resource` string into a
// machine-readable type + id pair plus a human-readable label so the UI
// can render "set_pack_status · MyPack → flagged" instead of the raw
// "pack:<uuid>" blob.
type auditEntry struct {
	ID            string          `json:"id"`
	AdminID       *string         `json:"admin_id"`
	AdminUsername string          `json:"admin_username"`
	Action        string          `json:"action"`
	ResourceType  string          `json:"resource_type"`
	ResourceID    string          `json:"resource_id"`
	ResourceLabel string          `json:"resource_label"`
	Changes       json.RawMessage `json:"changes"`
	CreatedAt     time.Time       `json:"created_at"`
}

// parseResource splits "pack:<uuid>" into ("pack", "<uuid>"). Empty strings
// and malformed values round-trip to ("", raw) so the handler can still
// surface them without crashing.
func parseResource(raw string) (kind, id string) {
	if idx := strings.Index(raw, ":"); idx > 0 {
		return raw[:idx], raw[idx+1:]
	}
	return "", raw
}

// ListAudit handles GET /api/admin/audit?limit=N. Powers the "Recent Activity"
// card on the admin dashboard. Capped at 50 entries — the dashboard shows 10
// by default and offers no deep pagination here (admins who need more use
// the full audit export under GDPR tooling).
//
// Enrichment strategy: two batch lookups (packs, users) after the initial
// audit query. N+1 is avoided by grouping UUIDs per resource type and
// issuing a single ANY($1) query each.
func (h *AdminHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := int32(10)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 50 {
			limit = int32(n)
		}
	}
	rows, err := h.db.ListRecentAuditLogs(ctx, limit)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list audit log")
		return
	}

	// Pre-parse every row once so we can both collect IDs for the batch
	// lookups and reuse the split when assembling the response.
	type parsedRow struct {
		kind string
		id   string
		uuid uuid.UUID
		ok   bool // uuid parsed successfully
	}
	parsed := make([]parsedRow, len(rows))
	var packIDs, userIDs []uuid.UUID
	for i, row := range rows {
		k, idStr := parseResource(row.Resource)
		parsed[i] = parsedRow{kind: k, id: idStr}
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		parsed[i].uuid = id
		parsed[i].ok = true
		switch k {
		case "pack":
			packIDs = append(packIDs, id)
		case "user":
			userIDs = append(userIDs, id)
		}
	}

	// Batch fetch labels. Failures are non-fatal: missing labels just
	// fall back to the short UUID below.
	packNames := map[uuid.UUID]string{}
	if len(packIDs) > 0 {
		packs, err := h.db.GetPackNamesByIDs(ctx, packIDs)
		if err == nil {
			for _, p := range packs {
				packNames[p.ID] = p.Name
			}
		}
	}
	userNames := map[uuid.UUID]string{}
	if len(userIDs) > 0 {
		users, err := h.db.GetUsernamesByIDs(ctx, userIDs)
		if err == nil {
			for _, u := range users {
				userNames[u.ID] = u.Username
			}
		}
	}

	out := make([]auditEntry, 0, len(rows))
	for i, row := range rows {
		p := parsed[i]
		label := ""
		if p.ok {
			switch p.kind {
			case "pack":
				if n, ok := packNames[p.uuid]; ok {
					label = n
				}
			case "user":
				if n, ok := userNames[p.uuid]; ok {
					label = n
				}
			}
			if label == "" && len(p.id) >= 8 {
				// Fallback: short UUID prefix — still useful for invites
				// and for packs/users that were hard-deleted after the
				// audit row was written.
				label = p.id[:8]
			}
		}
		entry := auditEntry{
			ID:            row.ID.String(),
			AdminUsername: row.AdminUsername,
			Action:        row.Action,
			ResourceType:  p.kind,
			ResourceID:    p.id,
			ResourceLabel: label,
			Changes:       row.Changes,
			CreatedAt:     row.CreatedAt,
		}
		if row.AdminID.Valid {
			s := uuid.UUID(row.AdminID.Bytes).String()
			entry.AdminID = &s
		}
		out = append(out, entry)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
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
