// backend/internal/api/admin_quotas.go
//
// Phase 1 of the groups paradigm — see
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
//
// Platform-admin surface for the per-user platform+group invite allocation.
// Reads are flat (List); writes are upserts (Set). The actual consumption of
// the allocated budget happens in phase 2 when the platform+group invite
// flow ships. Surfacing the admin route now lets operators allocate ahead
// of phase-2 enablement.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
)

type AdminQuotaHandler struct {
	db  *db.Queries
	cfg *config.Config
}

func NewAdminQuotaHandler(pool *pgxpool.Pool, cfg *config.Config) *AdminQuotaHandler {
	return &AdminQuotaHandler{db: db.New(pool), cfg: cfg}
}

// List handles GET /api/admin/user-invite-quotas — returns every user that
// has a quota row. Mounted under RequireAdmin in main.go so the auth gate is
// already enforced before we get here.
func (h *AdminQuotaHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.ListUserInviteQuotas(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list invite quotas")
		return
	}
	if rows == nil {
		rows = []db.ListUserInviteQuotasRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

type setUserQuotaRequest struct {
	Allocated int32 `json:"allocated"`
}

// Set handles PUT /api/admin/user-invite-quotas/{userID}. Upserts the
// allocation; the CHECK (used <= allocated) constraint blocks lowering the
// allocation below current consumption — surfaced as 409 with the
// quota_below_used code.
func (h *AdminQuotaHandler) Set(w http.ResponseWriter, r *http.Request) {
	uid, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user id")
		return
	}
	var req setUserQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Allocated < 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "allocated must be at least 0")
		return
	}

	row, err := h.db.UpsertUserInviteQuota(r.Context(), db.UpsertUserInviteQuotaParams{
		UserID:    uid,
		Allocated: req.Allocated,
	})
	if err != nil {
		// Most plausible failure here is the CHECK (used <= allocated)
		// violation. We don't sniff the pg error code because there is
		// only one realistic failure mode for this single-row upsert.
		writeError(w, r, http.StatusConflict, "quota_below_used",
			"Cannot set allocation below current consumption.")
		return
	}
	writeJSON(w, http.StatusOK, row)
}
