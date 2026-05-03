// backend/internal/api/items_bulk_text.go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// MaxBulkTextItems caps the number of text items per bulk request. Text
// items have no S3 step so memory cost is bounded only by the per-item text
// length (MaxBulkTextLength bytes), giving each request a hard ceiling of
// MaxBulkTextItems × MaxBulkTextLength = ~50 KiB. The cap also keeps the
// per-request transaction count predictable so a single bulk call cannot
// monopolise a connection on huge input.
const MaxBulkTextItems = 100

// MaxBulkTextLength mirrors the client-side MAX_TEXT_LENGTH used by
// validateItemText so the server reports the same ceiling. Pre-fix the
// client validated text length while the server didn't, which let API
// callers smuggle in oversize rows.
const MaxBulkTextLength = 500

// bulkTextRequest is the JSON wire shape for POST /items/bulk-text. The
// client uploads the *parsed* JSON array — the same shape parseTextItemsJson
// produces in the frontend — so the server's job is per-row validation +
// transactional persistence rather than re-parsing user files.
type bulkTextRequest struct {
	Items []struct {
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"items"`
}

// BulkCreateTextItems handles POST /api/packs/{id}/items/bulk-text.
//
// Why a single endpoint exists: the per-text-item flow used to do three
// sequential HTTP round-trips (createItem → createVersion → promote). For a
// 131-item bulk text import that's 393 requests, which saturates the
// per-user global limiter (default 100/min burst) within seconds. Worse,
// when a step fails mid-chain the client-side cleanup DELETE is itself
// rate-limited, so the failure leaves an orphan item row with NULL
// current_version_id that the studio renders as a broken row. This endpoint
// collapses the three steps into one server-side transaction per item and
// makes a whole batch cost one rate-limit token.
//
// Request: application/json — { "items": [ {name, text}, ... ] }
//
// Response: 200 OK, { "results": [ {ok, name, item?, reason?, code?}, ... ] }
//
// The HTTP status is 200 even when individual items fail — per-item outcomes
// are reported in the body. The endpoint as a whole only returns non-2xx for
// authz/parsing failures that prevent any work from happening.
func (h *PackHandler) BulkCreateTextItems(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.GetSessionUser(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	packID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack ID")
		return
	}
	pack, err := h.db.GetPackByID(r.Context(), packID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		return
	}
	if !canEditItems(r, h.db, pack, u) {
		writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
		return
	}
	if !h.ensureNotSystem(w, r, pack) {
		return
	}

	// Bound the request body. Each item has two short string fields; even at
	// MaxBulkTextLength bytes per text and MaxBulkTextItems rows the JSON
	// envelope sits comfortably under 256 KiB. Use 1 MiB as a generous
	// upper bound so a verbose JSON formatter (whitespace, escapes) cannot
	// fail innocent input.
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024)
	var req bulkTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, r, http.StatusBadRequest, "bad_request", "items array is required")
		return
	}
	if len(req.Items) > MaxBulkTextItems {
		writeError(w, r, http.StatusRequestEntityTooLarge, "too_many_items",
			fmt.Sprintf("Bulk text import accepts at most %d items per request", MaxBulkTextItems))
		return
	}

	results := make([]bulkUploadResult, len(req.Items))
	for i, it := range req.Items {
		name := strings.TrimSpace(it.Name)
		text := strings.TrimSpace(it.Text)
		results[i] = h.processBulkTextItem(r, packID, pack, u, name, text)
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// processBulkTextItem persists one text item: CreateItem + CreateItemVersion
// + SetCurrentVersion in a single transaction. A partial failure rolls back
// the whole row so no orphan can slip into the table. The returned shape
// matches the image-bulk endpoint so the client can render both flows with
// one summary toast.
func (h *PackHandler) processBulkTextItem(
	r *http.Request,
	packID uuid.UUID,
	pack db.GamePack,
	u middleware.SessionUser,
	name, text string,
) bulkUploadResult {
	res := bulkUploadResult{Filename: name}
	if name == "" {
		res.Code = "bad_request"
		res.Reason = "name is required"
		return res
	}
	if text == "" {
		res.Code = "bad_request"
		res.Reason = "text is required"
		return res
	}
	if len(text) > MaxBulkTextLength {
		res.Code = "text_too_long"
		res.Reason = fmt.Sprintf("text exceeds %d characters", MaxBulkTextLength)
		return res
	}

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to start transaction"
		return res
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	qtx := h.db.WithTx(tx)

	item, err := qtx.CreateItem(ctx, db.CreateItemParams{
		PackID:         packID,
		Name:           name,
		PayloadVersion: 2, // text items
	})
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to create item"
		return res
	}

	// Marshalling preserves any quote / unicode escaping in the text rather
	// than concatenating into a hand-rolled JSON literal that could break on
	// adversarial input.
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to marshal payload"
		return res
	}
	version, err := qtx.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   item.ID,
		MediaKey: nil,
		Payload:  payload,
	})
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to create version"
		return res
	}

	promoted, err := qtx.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               item.ID,
		CurrentVersionID: pgtype.UUID{Bytes: version.ID, Valid: true},
	})
	if err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to promote version"
		return res
	}

	if err := tx.Commit(ctx); err != nil {
		res.Code = "internal_error"
		res.Reason = "Failed to commit transaction"
		return res
	}
	committed = true

	row := db.ListItemsForPackRow{
		ID:               promoted.ID,
		PackID:           promoted.PackID,
		Name:             promoted.Name,
		Position:         promoted.Position,
		PayloadVersion:   promoted.PayloadVersion,
		CurrentVersionID: promoted.CurrentVersionID,
		CreatedAt:        promoted.CreatedAt,
		DeletedAt:        promoted.DeletedAt,
		LastEditorUserID: promoted.LastEditorUserID,
		LastEditedAt:     promoted.LastEditedAt,
		MediaKey:         nil,
		Payload:          payload,
		VersionNumber:    &version.VersionNumber,
	}
	res.OK = true
	res.Item = &enrichedItem{ListItemsForPackRow: row}
	bumpGroupEditor(r, h.db, pack, item.ID, u)
	return res
}
