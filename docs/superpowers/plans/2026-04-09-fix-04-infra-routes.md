# Infrastructure & Missing Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all missing REST endpoints (items/versions, room actions), add Prometheus metrics, fix the production Compose network, add Docker healthcheck, fix health check timeout, and run-as-unprivileged-user in Dockerfiles.

**Architecture:** Items/versions handlers are added to `PackHandler` in `packs.go` (or a new `items.go` file). Room action handlers (config, leave, kick, leaderboard) are added to `RoomHandler`. Prometheus instrumentation wraps the chi router. Dockerfiles add a non-root user.

**Tech Stack:** Go, `chi`, `prometheus/client_golang`, Docker Compose

---

### Task 1: Implement items CRUD endpoints

**Covers:** A5-C1 (partial — items routes)

**Files:**
- Create: `backend/internal/api/items.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/db/queries/game_items.sql`
- Regenerate: `backend/db/sqlc/`

- [x] **Step 1: Add DB queries for items**

> **Deviation (implemented):** Phase 3 had already added most queries with slightly different names. Rather than adding duplicates, we reused existing queries and only added the missing `ReorderItems`. Specifically:
> - `ListItems` → already exists as `ListItemsForPack` (with pagination params); used as-is.
> - `GetItemByID` → already exists; used as-is.
> - `CreateItem` → already exists; used as-is.
> - `UpdateItemCurrentVersion` → already exists as `SetCurrentVersion`; used `SetCurrentVersion` throughout.
> - `DeleteItem` → already exists as hard delete (not soft delete); used as-is.
> - `ReorderItems` → added to `game_items.sql` and manually to `game_items.sql.go`.

Only `ReorderItems` was added. The SQL file already had all other queries. `sqlc generate` was not run (not installed); `ReorderItems` was manually added to `backend/db/sqlc/game_items.sql.go`.

- [x] **Step 2: Regenerate sqlc**

> **Deviation (implemented):** sqlc is not installed. `ReorderItems` was manually added directly to `backend/db/sqlc/game_items.sql.go` following the existing code patterns. No other queries needed adding.

- [ ] **Step 3: Create backend/internal/api/items.go**

```go
// backend/internal/api/items.go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
    "github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// ListItems handles GET /api/packs/{id}/items.
func (h *PackHandler) ListItems(w http.ResponseWriter, r *http.Request) {
    _, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    packID, err := uuid.Parse(chi.URLParam(r, "id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid pack ID")
        return
    }
    items, err := h.db.ListItemsForPack(r.Context(), db.ListItemsForPackParams{PackID: packID, Lim: int32(limit), Off: int32(offset)})
    // > **Deviation (implemented):** Plan used `ListItems`, but existing function is `ListItemsForPack` with pagination params. Used pagination-aware version.
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list items")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

// CreateItem handles POST /api/packs/{id}/items.
func (h *PackHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
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
    // Verify ownership
    pack, err := h.db.GetPackByID(r.Context(), packID)
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
        return
    }
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    var req struct {
        PayloadVersion int `json:"payload_version"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        req.PayloadVersion = 1
    }
    if req.PayloadVersion == 0 {
        req.PayloadVersion = 1
    }
    item, err := h.db.CreateItem(r.Context(), db.CreateItemParams{
        PackID:         packID,
        PayloadVersion: int32(req.PayloadVersion),
    })
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create item")
        return
    }
    writeJSON(w, http.StatusCreated, item)
}

// UpdateItem handles PATCH /api/packs/{id}/items/{item_id}.
func (h *PackHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
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
    itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
        return
    }
    pack, err := h.db.GetPackByID(r.Context(), packID)
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
        return
    }
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    var req struct {
        CurrentVersionID *string `json:"current_version_id,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
        return
    }
    if req.CurrentVersionID != nil {
        versionID, err := uuid.Parse(*req.CurrentVersionID)
        if err != nil {
            writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
            return
        }
        item, err := h.db.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
            ID: itemID, CurrentVersionID: pgtype.UUID{Bytes: versionID, Valid: true},
        })
        // > **Deviation (implemented):** Plan used `UpdateItemCurrentVersion`/`UpdateItemCurrentVersionParams`; existing function is `SetCurrentVersion`/`SetCurrentVersionParams`.
        if err != nil {
            writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
            return
        }
        writeJSON(w, http.StatusOK, item)
        return
    }
    writeError(w, r, http.StatusBadRequest, "bad_request", "No updatable fields provided")
}

// DeleteItem handles DELETE /api/packs/{id}/items/{item_id}.
func (h *PackHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
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
    itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
        return
    }
    pack, err := h.db.GetPackByID(r.Context(), packID)
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
        return
    }
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    if err := h.db.DeleteItem(r.Context(), itemID); err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// ReorderItems handles PATCH /api/packs/{id}/items/reorder.
func (h *PackHandler) ReorderItems(w http.ResponseWriter, r *http.Request) {
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
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    var req struct {
        Positions []struct {
            ID       string `json:"id"`
            Position int    `json:"position"`
        } `json:"positions"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
        return
    }
    for _, p := range req.Positions {
        itemID, err := uuid.Parse(p.ID)
        if err != nil {
            continue
        }
        if err := h.db.ReorderItems(r.Context(), db.ReorderItemsParams{
            ID:     itemID,
            PackID: packID,
            Position: int32(p.Position),
        }); err != nil {
            writeError(w, r, http.StatusInternalServerError, "internal_error", "Reorder failed")
            return
        }
    }
    w.WriteHeader(http.StatusNoContent)
}
```

Add missing `pgtype` import where needed.

- [ ] **Step 4: Wire item routes in main.go**

In `backend/cmd/server/main.go`, replace the packs route block:

```go
r.With(mw.RequireAuth).Route("/api/packs", func(r chi.Router) {
    r.Get("/", packHandler.List)
    r.Post("/", packHandler.Create)
    r.Get("/{id}", packHandler.GetByID)
    r.Patch("/{id}", packHandler.Update)
    r.Delete("/{id}", packHandler.Delete)
    r.With(mw.RequireAdmin).Patch("/{id}/status", packHandler.SetStatus)
    // Items
    r.Get("/{id}/items", packHandler.ListItems)
    r.Post("/{id}/items", packHandler.CreateItem)
    r.Patch("/{id}/items/reorder", packHandler.ReorderItems)
    r.Patch("/{id}/items/{item_id}", packHandler.UpdateItem)
    r.Delete("/{id}/items/{item_id}", packHandler.DeleteItem)
    // Versions
    r.Get("/{id}/items/{item_id}/versions", packHandler.ListVersions)
    r.Post("/{id}/items/{item_id}/versions", packHandler.CreateVersion)
    r.Post("/{id}/items/{item_id}/versions/{vid}/restore", packHandler.RestoreVersion)
    r.Delete("/{id}/items/{item_id}/versions/{vid}", packHandler.SoftDeleteVersion)
    r.Delete("/{id}/items/{item_id}/versions/{vid}/purge", packHandler.PurgeVersion)
})
```

- [ ] **Step 5: Verify build**

```bash
cd backend && go build ./...
```

- [ ] **Step 6: Commit items work**

```bash
git add backend/internal/api/items.go backend/db/queries/ backend/db/sqlc/ backend/cmd/server/main.go
git commit -m "feat(api): implement items CRUD endpoints (list, create, update, delete, reorder)"
```

---

### Task 2: Implement item versions endpoints

**Covers:** A5-C1 (versions portion)

**Files:**
- Create: `backend/internal/api/versions.go`
- Modify: `backend/db/queries/game_item_versions.sql`
- Regenerate: `backend/db/sqlc/`

- [x] **Step 1: Add version DB queries**

> **Deviation (implemented):** All version queries already existed in `game_items.sql` (not a separate file) under different names:
> - `ListItemVersions` → already exists as `ListVersionsForItem`; used as-is.
> - `CreateItemVersion` → already exists; used as-is.
> - `SoftDeleteVersion` → already exists; used as-is.
> - `HardDeleteVersion` → already exists; used as-is.
> No queries needed to be added. The `game_item_versions.sql` file does not exist separately — all queries live in `game_items.sql`.

- [x] **Step 2: Regenerate sqlc**

> **Deviation (implemented):** sqlc is not installed and no new queries were needed — all already existed with equivalent names. No regeneration required.

- [ ] **Step 3: Create backend/internal/api/versions.go**

```go
// backend/internal/api/versions.go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
    "github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// ListVersions handles GET /api/packs/{id}/items/{item_id}/versions.
func (h *PackHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
    _, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
        return
    }
    versions, err := h.db.ListVersionsForItem(r.Context(), itemID)
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to list versions")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"data": versions})
}

// CreateVersion handles POST /api/packs/{id}/items/{item_id}/versions.
func (h *PackHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
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
    itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
        return
    }
    pack, err := h.db.GetPackByID(r.Context(), packID)
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
        return
    }
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    var req struct {
        MediaKey string          `json:"media_key"`
        Payload  json.RawMessage `json:"payload"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
        return
    }
    if req.Payload == nil {
        req.Payload = json.RawMessage(`{}`)
    }
    version, err := h.db.CreateItemVersion(r.Context(), db.CreateItemVersionParams{
        ItemID:   itemID,
        MediaKey: strPtr(req.MediaKey),
        Payload:  req.Payload,
    })
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to create version")
        return
    }
    writeJSON(w, http.StatusCreated, version)
}

// RestoreVersion handles POST /api/packs/{id}/items/{item_id}/versions/{vid}/restore.
func (h *PackHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
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
    itemID, err := uuid.Parse(chi.URLParam(r, "item_id"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid item ID")
        return
    }
    versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
        return
    }
    pack, err := h.db.GetPackByID(r.Context(), packID)
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Pack not found")
        return
    }
    ownerID, _ := uuid.Parse(u.UserID)
    if u.Role != "admin" && (!pack.OwnerID.Valid || pack.OwnerID.Bytes != ownerID) {
        writeError(w, r, http.StatusForbidden, "forbidden", "Access denied")
        return
    }
    item, err := h.db.SetCurrentVersion(r.Context(), db.SetCurrentVersionParams{
        ID:               itemID,
        CurrentVersionID: pgtype.UUID{Bytes: versionID, Valid: true},
    })
    // > **Deviation (implemented):** Plan used `UpdateItemCurrentVersion`/`UpdateItemCurrentVersionParams`, but the existing generated function is `SetCurrentVersion`/`SetCurrentVersionParams`. Used the existing name.
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Restore failed")
        return
    }
    writeJSON(w, http.StatusOK, item)
}

// SoftDeleteVersion handles DELETE /api/packs/{id}/items/{item_id}/versions/{vid}.
func (h *PackHandler) SoftDeleteVersion(w http.ResponseWriter, r *http.Request) {
    _, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
        return
    }
    if err := h.db.SoftDeleteVersion(r.Context(), versionID); err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Delete failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// PurgeVersion handles DELETE /api/packs/{id}/items/{item_id}/versions/{vid}/purge (admin).
func (h *PackHandler) PurgeVersion(w http.ResponseWriter, r *http.Request) {
    u, ok := middleware.GetSessionUser(r)
    if !ok || u.Role != "admin" {
        writeError(w, r, http.StatusForbidden, "forbidden", "Admin role required")
        return
    }
    versionID, err := uuid.Parse(chi.URLParam(r, "vid"))
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid version ID")
        return
    }
    if err := h.db.HardDeleteVersion(r.Context(), versionID); err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Purge failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Verify build and commit**

```bash
cd backend && go build ./...
git add backend/internal/api/versions.go backend/db/queries/ backend/db/sqlc/
git commit -m "feat(api): implement item version endpoints (list, create, restore, soft-delete, purge)"
```

---

### Task 3: Implement room action endpoints

**Covers:** A5-C2

**Files:**
- Create: `backend/internal/api/room_actions.go`
- Modify: `backend/db/queries/rooms.sql`
- Modify: `backend/cmd/server/main.go`
- Regenerate: `backend/db/sqlc/`

- [ ] **Step 1: Add DB queries for room actions**

In `backend/db/queries/rooms.sql`:

```sql
-- name: UpdateRoomConfig :one
UPDATE rooms SET config = $2 WHERE id = $1 AND state = 'lobby' RETURNING *;

-- name: DeleteRoomPlayer :exec
DELETE FROM room_players WHERE room_id = $1 AND user_id = $2;

-- name: GetRoomPlayer :one
SELECT * FROM room_players WHERE room_id = $1 AND user_id = $2;
```

> **Deviation (implemented):** All three queries already existed with the same names (`UpdateRoomConfig`, `GetRoomPlayer`) or equivalents (`RemoveRoomPlayer` instead of `DeleteRoomPlayer`). `RemoveRoomPlayer` was used in place of `DeleteRoomPlayer` throughout room_actions.go.

- [x] **Step 2: Regenerate sqlc**

> **Deviation (implemented):** sqlc is not installed and no new queries needed to be added. All queries already existed.

- [ ] **Step 3: Create room_actions.go**

```go
// backend/internal/api/room_actions.go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
    "github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// UpdateConfig handles PATCH /api/rooms/{code}/config.
// Only callable in lobby state; host only.
func (h *RoomHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
    u, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
        return
    }
    hostID, _ := uuid.Parse(u.UserID)
    if !room.HostID.Valid || room.HostID.Bytes != hostID {
        writeError(w, r, http.StatusForbidden, "forbidden", "Only the host can update room config")
        return
    }
    if room.State != "lobby" {
        writeError(w, r, http.StatusConflict, "game_already_started", "Room config can only be changed in lobby")
        return
    }
    var cfg json.RawMessage
    if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
        return
    }
    updated, err := h.db.UpdateRoomConfig(r.Context(), db.UpdateRoomConfigParams{
        ID: room.ID, Config: cfg,
    })
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Update failed")
        return
    }
    writeJSON(w, http.StatusOK, updated)
}

// Leave handles POST /api/rooms/{code}/leave.
func (h *RoomHandler) Leave(w http.ResponseWriter, r *http.Request) {
    u, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
        return
    }
    userID, _ := uuid.Parse(u.UserID)
    if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
        RoomID: room.ID, UserID: userID,
    }); err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Leave failed")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

// Kick handles POST /api/rooms/{code}/kick.
func (h *RoomHandler) Kick(w http.ResponseWriter, r *http.Request) {
    u, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
        return
    }
    hostID, _ := uuid.Parse(u.UserID)
    if !room.HostID.Valid || room.HostID.Bytes != hostID {
        writeError(w, r, http.StatusForbidden, "forbidden", "Only the host can kick players")
        return
    }
    var req struct{ UserID string `json:"user_id"` }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid JSON")
        return
    }
    targetID, err := uuid.Parse(req.UserID)
    if err != nil {
        writeError(w, r, http.StatusBadRequest, "bad_request", "Invalid user_id")
        return
    }
    if err := h.db.RemoveRoomPlayer(r.Context(), db.RemoveRoomPlayerParams{
        RoomID: room.ID, UserID: targetID,
    }); err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Kick failed")
        return
    }
    // Signal hub via manager if active
    if hub, ok := h.manager.Get(room.Code); ok {
        hub.KickPlayer(req.UserID)
    }
    w.WriteHeader(http.StatusNoContent)
}

// Leaderboard handles GET /api/rooms/{code}/leaderboard.
func (h *RoomHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
    _, ok := middleware.GetSessionUser(r)
    if !ok {
        writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
        return
    }
    room, err := h.db.GetRoomByCode(r.Context(), chi.URLParam(r, "code"))
    if err != nil {
        writeError(w, r, http.StatusNotFound, "not_found", "Room not found")
        return
    }
    leaderboard, err := h.db.GetRoomLeaderboard(r.Context(), room.ID)
    if err != nil {
        writeError(w, r, http.StatusInternalServerError, "internal_error", "Failed to get leaderboard")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"data": leaderboard})
}
```

- [ ] **Step 4: Add KickPlayer method to Hub**

In `backend/internal/game/hub.go`:

```go
// KickPlayer sends a player_kicked message and closes the connection.
// Called from the HTTP kick handler. Thread-safe.
func (h *Hub) KickPlayer(userID string) {
    h.incoming <- playerMessage{
        player:  &connectedPlayer{userID: "system"},
        msgType: "system:kick",
        data:    json.RawMessage(`{"target_user_id":"` + userID + `"}`),
    }
}
```

Handle in `handleMessage`:
```go
case "system:kick":
    var d struct{ TargetUserID string `json:"target_user_id"` }
    json.Unmarshal(msg.data, &d)
    if p, ok := h.players[d.TargetUserID]; ok {
        h.safeSend(p, buildMessage("player_kicked", map[string]string{
            "user_id": d.TargetUserID,
        }))
        delete(h.players, d.TargetUserID)
        h.broadcast(buildMessage("player_kicked", map[string]string{
            "user_id": d.TargetUserID,
        }))
    }
```

- [ ] **Step 5: Wire room action routes in main.go**

```go
r.With(mw.RequireAuth).Route("/api/rooms", func(r chi.Router) {
    r.With(roomLimiter.Middleware).Post("/", roomHandler.Create)
    r.Get("/{code}", roomHandler.GetByCode)
    r.Patch("/{code}/config", roomHandler.UpdateConfig)
    r.Post("/{code}/leave", roomHandler.Leave)
    r.Post("/{code}/kick", roomHandler.Kick)
    r.Get("/{code}/leaderboard", roomHandler.Leaderboard)
})
```

- [ ] **Step 6: Verify build and commit**

```bash
cd backend && go build ./...
git add backend/internal/api/room_actions.go backend/internal/game/hub.go \
        backend/db/queries/ backend/db/sqlc/ backend/cmd/server/main.go
git commit -m "feat(api): implement room action endpoints (config, leave, kick, leaderboard)"
```

---

### Task 4: Add Prometheus metrics

**Covers:** A5-H2, A5-H5

**Files:**
- Modify: `backend/go.mod` (add prometheus dependency)
- Create: `backend/internal/middleware/metrics.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add prometheus dependency**

```bash
cd backend && go get github.com/prometheus/client_golang@latest
```

- [ ] **Step 2: Create metrics middleware**

```go
// backend/internal/middleware/metrics.go
package middleware

import (
    "net/http"
    "strconv"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests.",
    }, []string{"method", "path", "status"})

    httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request latency.",
        Buckets: prometheus.DefBuckets,
    }, []string{"method", "path"})
)

type responseRecorder struct {
    http.ResponseWriter
    status int
}

func (rr *responseRecorder) WriteHeader(code int) {
    rr.status = code
    rr.ResponseWriter.WriteHeader(code)
}

// Metrics wraps handlers to record Prometheus HTTP metrics.
func Metrics(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rr, r)
        duration := time.Since(start).Seconds()
        status := strconv.Itoa(rr.status)
        httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
        httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
    })
}
```

- [ ] **Step 3: Wire metrics in main.go**

Add `github.com/prometheus/client_golang/prometheus/promhttp` import and:

```go
r.Use(mw.Metrics)

// Metrics endpoint — restrict to localhost/internal only
// (IP restriction must be enforced at the reverse proxy or firewall level)
r.Handle("/api/metrics", promhttp.Handler())
```

Note: Prometheus endpoint MUST be IP-restricted at the reverse proxy or network level. Add a comment in main.go.

- [ ] **Step 4: Verify build and commit**

```bash
cd backend && go build ./...
git add backend/internal/middleware/metrics.go backend/cmd/server/main.go backend/go.mod backend/go.sum
git commit -m "feat(metrics): add Prometheus HTTP instrumentation middleware and /api/metrics endpoint"
```

---

### Task 5: Add RustFS health check probe

**Covers:** A5-H3, A5-L1

**Files:**
- Modify: `backend/internal/api/health.go`
- Modify: `backend/internal/storage/s3.go` (add Probe method to Storage interface)

- [ ] **Step 1: Add Probe to Storage interface**

In `backend/internal/storage/s3.go` or wherever the interface is defined:

```go
type Storage interface {
    PresignUpload(ctx context.Context, key string, ttl time.Duration) (string, error)
    PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error)
    DeleteObject(ctx context.Context, key string) error
    Probe(ctx context.Context) error // HEAD request to bucket root to verify connectivity
}
```

Implement `Probe` in the S3 client:

```go
func (s *S3Client) Probe(ctx context.Context) error {
    _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
    return err
}
```

- [ ] **Step 2: Add storage to HealthHandler and probe in Readiness**

```go
type HealthHandler struct {
    pool    *pgxpool.Pool
    storage storage.Storage
}

func NewHealthHandler(pool *pgxpool.Pool, store storage.Storage) *HealthHandler {
    return &HealthHandler{pool: pool, storage: store}
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second) // A5-L1: 2s not 3s
    defer cancel()

    checks := map[string]string{}
    status := http.StatusOK

    if err := h.pool.Ping(ctx); err != nil {
        checks["postgres"] = "unreachable: " + err.Error()
        status = http.StatusServiceUnavailable
    } else {
        checks["postgres"] = "ok"
    }

    if err := h.storage.Probe(ctx); err != nil {
        checks["rustfs"] = "unreachable: " + err.Error()
        status = http.StatusServiceUnavailable
    } else {
        checks["rustfs"] = "ok"
    }

    resp := map[string]any{"status": "ok", "checks": checks}
    if status != http.StatusOK {
        resp["status"] = "degraded"
    }
    writeJSON(w, status, resp)
}
```

- [ ] **Step 3: Update main.go to pass store to health handler**

```go
healthHandler := api.NewHealthHandler(pool, store)
```

- [ ] **Step 4: Verify build and commit**

```bash
cd backend && go build ./...
git add backend/internal/api/health.go backend/internal/storage/ backend/cmd/server/main.go
git commit -m "fix(health): deep check probes RustFS with 2s timeout; fix timeout from 3s to 2s"
```

---

### Task 6: Fix production Compose network and add healthcheck

**Covers:** A5-H4, A5-M3

**Files:**
- Modify: `docker/compose.prod.yml`
- Modify: `docker/compose.base.yml`

- [ ] **Step 1: Fix frontend network in prod overlay**

In `docker/compose.prod.yml`:

```yaml
services:
  backend:
    image: ghcr.io/morgankryze/fabyoumeme-backend:latest

  frontend:
    image: ghcr.io/morgankryze/fabyoumeme-frontend:latest
    networks:
      - project_network
      - pangolin

networks:
  pangolin:
    external: true
```

- [ ] **Step 2: Add backend healthcheck to base Compose**

In `docker/compose.base.yml`, add to the `backend` service:

```yaml
backend:
  healthcheck:
    test: ['CMD-SHELL', 'wget -qO- http://localhost:8080/api/health || exit 1']
    interval: 5s
    retries: 10
    start_period: 10s
```

Also update `frontend.depends_on` to use `service_healthy`:

```yaml
frontend:
  depends_on:
    backend:
      condition: service_healthy
```

- [ ] **Step 3: Commit**

```bash
git add docker/compose.prod.yml docker/compose.base.yml
git commit -m "fix(docker): add frontend to project_network in prod, add backend healthcheck"
```

---

### Task 7: Run containers as non-root user

**Covers:** A5-M2

**Files:**
- Modify: `backend/Dockerfile`
- Modify: `frontend/Dockerfile`

- [ ] **Step 1: Update backend Dockerfile**

Read the current backend Dockerfile, then add before the final `CMD`:

```dockerfile
# Create unprivileged user
RUN addgroup -S appgroup && adduser -S -G appgroup -u 1000 appuser
USER appuser
```

(Alpine-style; if Debian-based, use `addgroup --system appgroup && adduser --system --ingroup appgroup --uid 1000 appuser`)

- [ ] **Step 2: Update frontend Dockerfile similarly**

```dockerfile
RUN addgroup -S appgroup && adduser -S -G appgroup -u 1000 appuser
USER appuser
```

- [ ] **Step 3: Commit**

```bash
git add backend/Dockerfile frontend/Dockerfile
git commit -m "fix(docker): run backend and frontend as unprivileged user (uid 1000)"
```

---

### Task 8: Add missing env vars to .env.example files

**Covers:** A5-M4

**Files:**
- Modify: `.env.example`
- Modify: `.env.dev.example` (if it exists)

- [ ] **Step 1: Add optional vars to .env.example**

Append to `.env.example`:

```bash
# Rate limiting (optional — defaults shown)
# RATE_LIMIT_AUTH_RPM=10
# RATE_LIMIT_ROOMS_RPH=10
# RATE_LIMIT_UPLOADS_RPH=50
# RATE_LIMIT_GLOBAL_RPM=100

# WebSocket tuning (optional — defaults shown)
# WS_RATE_LIMIT=20
# WS_READ_LIMIT_BYTES=4096
# WS_READ_DEADLINE=60s
# WS_PING_INTERVAL=25s

# Session tuning (optional — default shown)
# SESSION_RENEW_INTERVAL=60m
```

- [ ] **Step 2: Commit**

```bash
git add .env.example
git commit -m "docs: document optional env vars with defaults in .env.example"
```

---

### Task 9: Graceful shutdown of WebSocket connections

**Covers:** A3-L3

**Files:**
- Modify: `backend/internal/game/manager.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Add Shutdown method to Manager**

In `backend/internal/game/manager.go`:

```go
// Shutdown broadcasts "server restarting" to all active hubs and waits for them to drain.
// Call before srv.Shutdown() for clean WebSocket teardown.
func (m *Manager) Shutdown() {
    m.mu.RLock()
    hubs := make([]*Hub, 0, len(m.hubs))
    for _, h := range m.hubs {
        hubs = append(hubs, h)
    }
    m.mu.RUnlock()

    for _, h := range hubs {
        h.broadcast(buildMessage("server_restarting", map[string]string{
            "message": "Server is restarting. Please reconnect in a few moments.",
        }))
    }
    // Brief pause to allow messages to be flushed before connections are torn down.
    time.Sleep(1 * time.Second)
}
```

Note: `buildMessage` is in `hub.go` (same package), so it's accessible.

- [ ] **Step 2: Call manager.Shutdown() before srv.Shutdown() in main.go**

```go
<-quit
logger.Info("shutting down")
manager.Shutdown() // Notify WS clients before closing listeners
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    logger.Error("shutdown error", "error", err)
}
logger.Info("server stopped")
```

- [ ] **Step 3: Verify build and commit**

```bash
cd backend && go build ./...
git add backend/internal/game/manager.go backend/cmd/server/main.go
git commit -m "fix(server): broadcast server_restarting to WS clients before graceful shutdown"
```

---

### Task 10: Fix startup cleanup logging

**Covers:** A3-L4

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/db/queries/rooms.sql` (if queries don't return counts)

- [ ] **Step 1: Update cleanup queries to return affected counts (if not already)**

Check if `FinishCrashedRooms` and `FinishAbandonedLobbies` return counts. If they are `:exec`, change them to `:execresult` to get `pgconn.CommandTag`:

In rooms.sql:
```sql
-- name: FinishCrashedRooms :execresult
UPDATE rooms SET state = 'finished', finished_at = now()
WHERE state = 'playing';

-- name: FinishAbandonedLobbies :execresult
UPDATE rooms SET state = 'finished', finished_at = now()
WHERE state = 'lobby' AND created_at < now() - interval '24 hours';
```

Regenerate and update main.go to use the result:

```go
if result, err := queries.FinishCrashedRooms(context.Background()); err != nil {
    logger.Error("startup: finish crashed rooms", "error", err)
} else {
    logger.Info("startup cleanup", "event", "room.crash_recovery",
        "count", result.RowsAffected())
}
if result, err := queries.FinishAbandonedLobbies(context.Background()); err != nil {
    logger.Error("startup: finish abandoned lobbies", "error", err)
} else {
    logger.Info("startup cleanup", "event", "room.abandoned",
        "count", result.RowsAffected())
}
```

- [ ] **Step 2: Regenerate sqlc if query type changed**

```bash
cd backend && sqlc generate && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go backend/db/queries/ backend/db/sqlc/
git commit -m "fix(server): emit structured info logs with counts for startup room cleanup"
```
