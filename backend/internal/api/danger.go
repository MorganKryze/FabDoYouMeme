// backend/internal/api/danger.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/reset"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// DangerHandler hosts the /api/admin/danger/* routes. Instances are
// created by main.go ONLY when cfg.AppEnv != "prod" — routes are
// literally unmounted in prod (404, not 403) so there is no code path
// that can serve them even if the auth middleware were misconfigured.
type DangerHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	storage storage.Storage
}

func NewDangerHandler(pool *pgxpool.Pool, st storage.Storage) *DangerHandler {
	return &DangerHandler{
		pool:    pool,
		queries: db.New(pool),
		storage: st,
	}
}

// dangerRequestBody is the wire shape for every danger endpoint.
type dangerRequestBody struct {
	Confirmation string `json:"confirmation"`
}

// errInvalidConfirmation is returned (via the error channel) from
// validateConfirmation when the phrase does not match. The HTTP
// response has already been written at that point — handlers only use
// the error as a signal to stop processing.
var errInvalidConfirmation = errors.New("invalid confirmation")

// validateConfirmation decodes the body and checks the phrase. On
// failure it writes the HTTP response itself and returns a non-nil
// error so the caller can return early without additional handling.
func (h *DangerHandler) validateConfirmation(w http.ResponseWriter, r *http.Request, expected string) error {
	var body dangerRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return err
	}
	if body.Confirmation != expected {
		writeError(w, r, http.StatusBadRequest, "invalid_confirmation", "Confirmation phrase does not match")
		return errInvalidConfirmation
	}
	return nil
}

// runInTx opens a tx, runs fn, commits on success, rolls back on
// failure. The Report is returned regardless so callers can surface
// partial counts in an error path if they want to.
func (h *DangerHandler) runInTx(ctx context.Context, fn func(tx pgx.Tx) (reset.Report, error)) (reset.Report, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return reset.Report{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	report, err := fn(tx)
	if err != nil {
		return report, err
	}
	if err := tx.Commit(ctx); err != nil {
		return report, err
	}
	return report, nil
}

// writeAuditAndRespond writes the audit log entry (best-effort — see
// writeAuditLog in admin.go) and returns the report as JSON. Called
// only on a successful wipe, never from the failure path.
func (h *DangerHandler) writeAuditAndRespond(w http.ResponseWriter, r *http.Request, action string, report reset.Report) {
	if admin, ok := middleware.GetSessionUser(r); ok {
		writeAuditLog(r.Context(), h.queries, admin.UserID, action, "system", report)
	}
	writeJSON(w, http.StatusOK, report)
}

// WipeGameHistory handles POST /api/admin/danger/wipe-game-history.
func (h *DangerHandler) WipeGameHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.validateConfirmation(w, r, "wipe game history"); err != nil {
		return
	}
	report, err := h.runInTx(r.Context(), func(tx pgx.Tx) (reset.Report, error) {
		return reset.WipeGameHistory(r.Context(), tx)
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Wipe failed")
		return
	}
	h.writeAuditAndRespond(w, r, "danger_wipe_game_history", report)
}

// WipePacksAndMedia handles POST /api/admin/danger/wipe-packs-and-media.
func (h *DangerHandler) WipePacksAndMedia(w http.ResponseWriter, r *http.Request) {
	if err := h.validateConfirmation(w, r, "wipe packs and media"); err != nil {
		return
	}
	report, err := h.runInTx(r.Context(), func(tx pgx.Tx) (reset.Report, error) {
		return reset.WipePacksAndMedia(r.Context(), tx, h.storage)
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Wipe failed")
		return
	}
	h.writeAuditAndRespond(w, r, "danger_wipe_packs_and_media", report)
}

// WipeInvites handles POST /api/admin/danger/wipe-invites.
func (h *DangerHandler) WipeInvites(w http.ResponseWriter, r *http.Request) {
	if err := h.validateConfirmation(w, r, "wipe invites"); err != nil {
		return
	}
	report, err := h.runInTx(r.Context(), func(tx pgx.Tx) (reset.Report, error) {
		return reset.WipeInvites(r.Context(), tx)
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Wipe failed")
		return
	}
	h.writeAuditAndRespond(w, r, "danger_wipe_invites", report)
}

// WipeSessions handles POST /api/admin/danger/wipe-sessions. The acting
// admin's own session is preserved so they don't log themselves out
// mid-action (the UI would have no way to show the result otherwise).
func (h *DangerHandler) WipeSessions(w http.ResponseWriter, r *http.Request) {
	if err := h.validateConfirmation(w, r, "force logout everyone"); err != nil {
		return
	}
	admin, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	actingID, err := uuid.Parse(admin.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session user id")
		return
	}
	report, err := h.runInTx(r.Context(), func(tx pgx.Tx) (reset.Report, error) {
		return reset.WipeSessions(r.Context(), tx, actingID)
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Wipe failed")
		return
	}
	h.writeAuditAndRespond(w, r, "danger_wipe_sessions", report)
}

// FullReset handles POST /api/admin/danger/full-reset. The required
// confirmation phrase is all-caps because this is the most destructive
// action — the extra caps-lock step forces the operator to consciously
// shift into "yes, restore to first boot" mode.
func (h *DangerHandler) FullReset(w http.ResponseWriter, r *http.Request) {
	if err := h.validateConfirmation(w, r, "RESET TO FIRST BOOT"); err != nil {
		return
	}
	admin, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	actingID, err := uuid.Parse(admin.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Invalid session user id")
		return
	}
	report, err := h.runInTx(r.Context(), func(tx pgx.Tx) (reset.Report, error) {
		return reset.FullReset(r.Context(), tx, h.storage, actingID)
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Full reset failed")
		return
	}
	h.writeAuditAndRespond(w, r, "danger_full_reset", report)
}
