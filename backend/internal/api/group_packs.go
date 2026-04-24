// backend/internal/api/group_packs.go
//
// Phase 3 of the groups paradigm — see
// docs/superpowers/specs/2026-04-22-groups-paradigm-design.md.
//
// Handler boundary for the group-pack surface. Five shapes live here:
//  1. List + duplicate (group-level reads / writes)
//  2. Item add + modify (any member)
//  3. Item delete + pack delete + pack evict (admin only)
//  4. Pending-duplication queue (admin only)
//
// Every write in (2) stamps (last_editor_user_id, last_edited_at) on the
// underlying game_items row so group admins can moderate the edit log.
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
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/pack"
)

type GroupPackHandler struct {
	pool *pgxpool.Pool
	db   *db.Queries
	cfg  *config.Config
	dup  *pack.Service
}

func NewGroupPackHandler(pool *pgxpool.Pool, cfg *config.Config) *GroupPackHandler {
	return &GroupPackHandler{
		pool: pool, db: db.New(pool), cfg: cfg,
		dup: pack.NewService(pool),
	}
}

// loadGroupPack resolves both (gid, pid) from the URL, loads the pack, and
// enforces group_id matches. Writes the appropriate 404/400 on failure.
// Returns the loaded pack + uuids so handlers don't re-parse.
func (h *GroupPackHandler) loadGroupPack(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, db.GamePack, bool) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, db.GamePack{}, false
	}
	pid, err := uuid.Parse(chi.URLParam(r, "packID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack id")
		return uuid.Nil, uuid.Nil, db.GamePack{}, false
	}
	p, err := h.db.GetGroupPack(r.Context(), pid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load pack")
		}
		return uuid.Nil, uuid.Nil, db.GamePack{}, false
	}
	if !p.GroupID.Valid || p.GroupID.Bytes != gid {
		// Cross-group access attempt — surface as 404 to avoid leaking the
		// pack's real group membership.
		writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
		return uuid.Nil, uuid.Nil, db.GamePack{}, false
	}
	return gid, pid, p, true
}

func (h *GroupPackHandler) requireGroupMembership(w http.ResponseWriter, r *http.Request, gid uuid.UUID) (uuid.UUID, db.GroupMembership, bool) {
	uid, ok := requireSessionUUID(w, r)
	if !ok {
		return uuid.Nil, db.GroupMembership{}, false
	}
	mem, err := h.db.GetMembership(r.Context(), db.GetMembershipParams{GroupID: gid, UserID: uid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, r, http.StatusForbidden, "not_group_member", "You are not a member of this group.")
		} else {
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to load membership")
		}
		return uuid.Nil, db.GroupMembership{}, false
	}
	return uid, mem, true
}

// ─── List ────────────────────────────────────────────────────────────────────

// List handles GET /api/groups/{id}/packs — visible to any member.
func (h *GroupPackHandler) List(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	if _, _, ok := h.requireGroupMembership(w, r, gid); !ok {
		return
	}
	rows, err := h.db.ListGroupPacks(r.Context(), pgtype.UUID{Bytes: gid, Valid: true})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list packs")
		return
	}
	if rows == nil {
		rows = []db.GamePack{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// ─── Duplicate ───────────────────────────────────────────────────────────────

type duplicateRequest struct {
	SourcePackID string `json:"source_pack_id"`
}

// Duplicate handles POST /api/groups/{id}/packs/duplicate. Members only;
// admin is NOT required — any member can pull content into the group. The
// NSFW→SFW path returns 202 Accepted with a pending row.
func (h *GroupPackHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	uid, _, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	var req duplicateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	sourceID, err := uuid.Parse(req.SourcePackID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid source_pack_id")
		return
	}

	out, err := h.dup.Duplicate(r.Context(), sourceID, gid, uid)
	if err != nil {
		switch {
		case errors.Is(err, pack.ErrNotMember):
			writeError(w, r, http.StatusForbidden, "not_group_member", err.Error())
		case errors.Is(err, pack.ErrGroupDeleted):
			writeError(w, r, http.StatusNotFound, "group_not_found", err.Error())
		case errors.Is(err, pack.ErrSourceUnavailable):
			writeError(w, r, http.StatusForbidden, "source_pack_unavailable",
				"You cannot duplicate that pack.")
		case errors.Is(err, pack.ErrLanguageMismatch):
			writeError(w, r, http.StatusConflict, "language_mismatch", err.Error())
		case errors.Is(err, pack.ErrQuotaExceeded):
			writeError(w, r, http.StatusConflict, "group_quota_exceeded", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to duplicate pack")
		}
		return
	}
	if out.Pending != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":  "pending_admin_approval",
			"pending": out.Pending,
		})
		return
	}
	writeJSON(w, http.StatusCreated, out.NewPack)
}

// ─── Item CRUD ───────────────────────────────────────────────────────────────

type addItemRequest struct {
	Name           string          `json:"name"`
	PayloadVersion int             `json:"payload_version"`
	MediaKey       *string         `json:"media_key,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

// AddItem handles POST /api/groups/{id}/packs/{packID}/items. Any member.
//
// Mirrors the /api/packs item-creation flow: inserts game_items + a
// version-1 game_item_versions row + points current_version_id at it, all
// in one transaction. On success stamps the (actor, now) audit pair on the
// item row per spec.
func (h *GroupPackHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	gid, _, _, ok := h.loadGroupPack(w, r)
	if !ok {
		return
	}
	uid, _, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, r, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	if req.PayloadVersion == 0 {
		req.PayloadVersion = 1
	}
	if len(req.Payload) == 0 {
		req.Payload = json.RawMessage("{}")
	}

	// Need packID again — loadGroupPack returned it but we discarded; pull
	// from URL to keep this handler self-contained.
	pid, _ := uuid.Parse(chi.URLParam(r, "packID"))

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	q := h.db.WithTx(tx)

	item, err := q.CreateItem(r.Context(), db.CreateItemParams{
		PackID: pid, Name: req.Name, PayloadVersion: int32(req.PayloadVersion),
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create item")
		return
	}
	ver, err := q.CreateItemVersion(r.Context(), db.CreateItemVersionParams{
		ItemID: item.ID, MediaKey: req.MediaKey, Payload: req.Payload,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create version")
		return
	}
	if _, err := q.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
		ID: item.ID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to set current version")
		return
	}
	if err := q.BumpGroupItemEditor(r.Context(), db.BumpGroupItemEditorParams{
		ID:               item.ID,
		LastEditorUserID: pgtype.UUID{Bytes: uid, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to stamp editor")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

type modifyItemRequest struct {
	MediaKey *string         `json:"media_key,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// ModifyItem handles PATCH /api/groups/{id}/packs/{packID}/items/{itemID}.
// Any member. Creates a new version and points current_version_id at it so
// the historical trail remains intact — identical to /api/packs versioning.
func (h *GroupPackHandler) ModifyItem(w http.ResponseWriter, r *http.Request) {
	gid, pid, _, ok := h.loadGroupPack(w, r)
	if !ok {
		return
	}
	uid, _, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item id")
		return
	}
	it, err := h.db.GetItemByID(r.Context(), itemID)
	if err != nil || it.PackID != pid {
		writeError(w, r, http.StatusNotFound, "not_found", "Item not found")
		return
	}

	var req modifyItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
		return
	}
	if len(req.Payload) == 0 {
		req.Payload = json.RawMessage("{}")
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck
	q := h.db.WithTx(tx)

	ver, err := q.CreateItemVersion(r.Context(), db.CreateItemVersionParams{
		ItemID: itemID, MediaKey: req.MediaKey, Payload: req.Payload,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create version")
		return
	}
	if _, err := q.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
		ID: itemID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to set current version")
		return
	}
	if err := q.BumpGroupItemEditor(r.Context(), db.BumpGroupItemEditorParams{
		ID:               itemID,
		LastEditorUserID: pgtype.UUID{Bytes: uid, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to stamp editor")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to commit transaction")
		return
	}
	writeJSON(w, http.StatusOK, ver)
}

// DeleteItem handles DELETE /api/groups/{id}/packs/{packID}/items/{itemID}.
// Admin only — soft-delete via the existing game_items.deleted_at column.
func (h *GroupPackHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	gid, pid, _, ok := h.loadGroupPack(w, r)
	if !ok {
		return
	}
	_, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	itemID, err := uuid.Parse(chi.URLParam(r, "itemID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item id")
		return
	}
	it, err := h.db.GetItemByID(r.Context(), itemID)
	if err != nil || it.PackID != pid {
		writeError(w, r, http.StatusNotFound, "not_found", "Item not found")
		return
	}
	if err := h.db.SoftDeleteItem(r.Context(), itemID); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeletePack handles DELETE /api/groups/{id}/packs/{packID}. Admin only.
// Soft-delete; historical rooms that reference the pack continue to replay
// via the replay-redaction path.
func (h *GroupPackHandler) DeletePack(w http.ResponseWriter, r *http.Request) {
	gid, pid, _, ok := h.loadGroupPack(w, r)
	if !ok {
		return
	}
	_, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	if err := h.db.SoftDeleteGroupPack(r.Context(), db.SoftDeleteGroupPackParams{
		ID: pid, GroupID: pgtype.UUID{Bytes: gid, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to delete pack")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Evict handles POST /api/groups/{id}/packs/{packID}/evict. Admin only.
// Semantically identical to DeletePack but fires a group notification so
// members see the eviction in the in-app feed.
func (h *GroupPackHandler) Evict(w http.ResponseWriter, r *http.Request) {
	gid, pid, p, ok := h.loadGroupPack(w, r)
	if !ok {
		return
	}
	uid, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	if err := h.db.SoftDeleteGroupPack(r.Context(), db.SoftDeleteGroupPackParams{
		ID: pid, GroupID: pgtype.UUID{Bytes: gid, Valid: true},
	}); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to evict pack")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.evict_pack",
		"group:"+gid.String(), map[string]string{"pack_id": pid.String(), "pack_name": p.Name})
	payload, _ := json.Marshal(map[string]any{"pack_name": p.Name})
	_, _ = h.db.CreateGroupNotification(r.Context(), db.CreateGroupNotificationParams{
		GroupID:   gid,
		Type:      "pack_evicted",
		ActorID:   pgtype.UUID{Bytes: uid, Valid: true},
		SubjectID: pgtype.UUID{Bytes: pid, Valid: true},
		Payload:   payload,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ─── Duplication approval queue ─────────────────────────────────────────────

// ListPending handles GET /api/groups/{id}/duplication-queue. Admin only.
func (h *GroupPackHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	_, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	rows, err := h.db.ListOpenPendingDuplications(r.Context(), gid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list queue")
		return
	}
	if rows == nil {
		rows = []db.ListOpenPendingDuplicationsRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// AcceptPending handles POST /api/groups/{id}/duplication-queue/{queueID}/accept.
// Admin only. Runs the deep copy AND force-relabels the group to NSFW per spec.
func (h *GroupPackHandler) AcceptPending(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	uid, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	qid, err := uuid.Parse(chi.URLParam(r, "queueID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid queue id")
		return
	}
	// Guard: the pending row must belong to this group. GetPendingDuplication
	// returns without group-scoping, so check here.
	pending, err := h.db.GetPendingDuplication(r.Context(), qid)
	if err != nil || pending.GroupID != gid {
		writeError(w, r, http.StatusNotFound, "not_found", "Queue entry not found")
		return
	}
	if pending.ResolvedAt.Valid {
		writeError(w, r, http.StatusConflict, "duplication_already_resolved", "Queue entry is already resolved")
		return
	}
	newPack, err := h.dup.ApprovePending(r.Context(), qid, uid)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to approve duplication")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.force_relabel_on_duplication",
		"group:"+gid.String(), map[string]string{
			"queue_id":    qid.String(),
			"new_pack_id": newPack.ID.String(),
		})
	writeJSON(w, http.StatusOK, newPack)
}

// RejectPending handles POST /api/groups/{id}/duplication-queue/{queueID}/reject.
func (h *GroupPackHandler) RejectPending(w http.ResponseWriter, r *http.Request) {
	gid, ok := parseGroupID(w, r)
	if !ok {
		return
	}
	uid, mem, ok := h.requireGroupMembership(w, r, gid)
	if !ok {
		return
	}
	if mem.Role != "admin" {
		writeError(w, r, http.StatusForbidden, "not_group_admin", "Group admin role required")
		return
	}
	qid, err := uuid.Parse(chi.URLParam(r, "queueID"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid queue id")
		return
	}
	pending, err := h.db.GetPendingDuplication(r.Context(), qid)
	if err != nil || pending.GroupID != gid {
		writeError(w, r, http.StatusNotFound, "not_found", "Queue entry not found")
		return
	}
	if pending.ResolvedAt.Valid {
		writeError(w, r, http.StatusConflict, "duplication_already_resolved", "Queue entry is already resolved")
		return
	}
	if err := h.dup.RejectPending(r.Context(), qid, uid); err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to reject duplication")
		return
	}
	writeAuditLog(r.Context(), h.db, uid.String(), "group.reject_duplication",
		"group:"+gid.String(), map[string]string{"queue_id": qid.String()})
	w.WriteHeader(http.StatusNoContent)
}
