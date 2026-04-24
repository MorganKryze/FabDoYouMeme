package api_test

// Phase 3 follow-up — verifies that the generic /api/packs/{id}/... surface
// correctly handles group packs after access helpers moved off per-pack
// owner checks and onto canReadPack / canEditItems / canAdminPack. Each
// test sets up a group + a duplicated pack via the GroupPackHandler, then
// exercises the corresponding PackHandler route.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// newPackAndGroupPackHandlers returns both handlers backed by the same pool
// so tests can seed via GroupPackHandler and exercise via PackHandler.
func newPackAndGroupPackHandlers(t *testing.T) (packH, groupH any, q *db.Queries) {
	t.Helper()
	ph, q1 := newPackHandler(t)
	gh, _ := newGroupPackHandler(t)
	return ph, gh, q1
}

// seedDuplicatedGroupPack sets up a group + a source pack + duplicates it
// into the group, returning the admin user, the group, and the duplicated
// group pack. The duplicated pack carries one item with a current version.
func seedDuplicatedGroupPack(t *testing.T, slug string) (db.User, db.Group, db.GamePack, *db.Queries) {
	t.Helper()
	gh, q := newGroupPackHandler(t)
	admin, group := seedGroupWithClassification(t, q, slug, "sfw", "en")
	src := seedSourcePack(t, q, admin, "sfw", "en")
	rec := duplicateAs(t, gh, admin, group.ID, src.ID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("duplicate setup failed: %d — %s", rec.Code, rec.Body.String())
	}
	var pack db.GamePack
	if err := json.Unmarshal(rec.Body.Bytes(), &pack); err != nil {
		t.Fatalf("decode duplicated pack: %v", err)
	}
	return admin, group, pack, q
}

// ─── Read ───────────────────────────────────────────────────────────────────

// Members must see their group's packs through GET /api/packs/{id}; the
// canonical loader for the studio page reads through this route and would
// otherwise 403 on group-owned private packs.
func TestPackHandlerGetByID_GroupMember_CanRead(t *testing.T) {
	h, _ := newPackHandler(t)
	admin, group, pack, q := seedDuplicatedGroupPack(t, "gra")
	member := seedGroupUser(t, q, "gra_m")
	addMember(t, q, group, member, "member")
	_ = admin

	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/packs/"+pack.ID.String(), nil))
	req = withUser(req, member.ID.String(), member.Username, member.Email, member.Role)
	rec := httptest.NewRecorder()
	h.GetByID(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// Non-members of the group must get a 403 on GET /api/packs/{id} even
// though the pack exists. Group packs are private by design.
func TestPackHandlerGetByID_NonMember_Returns403(t *testing.T) {
	h, _ := newPackHandler(t)
	_, _, pack, q := seedDuplicatedGroupPack(t, "grb")
	outsider := seedGroupUser(t, q, "grb_o")

	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodGet, "/api/packs/"+pack.ID.String(), nil))
	req = withUser(req, outsider.ID.String(), outsider.Username, outsider.Email, outsider.Role)
	rec := httptest.NewRecorder()
	h.GetByID(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// ─── Item create / modify ───────────────────────────────────────────────────

// Any group member — not just admins — must be able to add items via the
// generic POST /api/packs/{id}/items route. The regular pack handler must
// honor the same rule as the group-scoped handler.
func TestPackHandlerCreateItem_GroupMember_Succeeds(t *testing.T) {
	h, _ := newPackHandler(t)
	_, group, pack, q := seedDuplicatedGroupPack(t, "gcia")
	member := seedGroupUser(t, q, "gcia_m")
	addMember(t, q, group, member, "member")

	body := []byte(`{"name":"new","payload_version":1}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/packs/"+pack.ID.String()+"/items", bytes.NewBuffer(body)))
	req = withUser(req, member.ID.String(), member.Username, member.Email, member.Role)
	rec := httptest.NewRecorder()
	h.CreateItem(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var item db.GameItem
	_ = json.Unmarshal(rec.Body.Bytes(), &item)
	// The bump must stamp the editor even through the generic path so
	// /api/groups/{gid}/packs/.../items and /api/packs/{pid}/items produce
	// identical audit state.
	row, err := q.GetItemByID(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if !row.LastEditorUserID.Valid || row.LastEditorUserID.Bytes != member.ID {
		t.Fatalf("expected last_editor_user_id=%v, got %+v", member.ID, row.LastEditorUserID)
	}
}

// A non-member who stumbles on the pack id must get 403. Mirrors the
// confidentiality guarantee of group-owned packs.
func TestPackHandlerCreateItem_NonMember_Returns403(t *testing.T) {
	h, _ := newPackHandler(t)
	_, _, pack, q := seedDuplicatedGroupPack(t, "gcib")
	outsider := seedGroupUser(t, q, "gcib_o")

	body := []byte(`{"name":"x"}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/packs/"+pack.ID.String()+"/items", bytes.NewBuffer(body)))
	req = withUser(req, outsider.ID.String(), outsider.Username, outsider.Email, outsider.Role)
	rec := httptest.NewRecorder()
	h.CreateItem(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

// ─── Item delete (admin-only for group packs) ───────────────────────────────

// Regular members cannot delete items from a group pack — canAdminPack must
// reject them even though canEditItems would let them add/modify.
func TestPackHandlerDeleteItem_GroupMember_Returns403(t *testing.T) {
	h, _ := newPackHandler(t)
	_, group, pack, q := seedDuplicatedGroupPack(t, "gdi")
	member := seedGroupUser(t, q, "gdi_m")
	addMember(t, q, group, member, "member")

	// The duplicated pack already has one item — find its id.
	items, err := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: pack.ID, Lim: 10, Off: 0,
	})
	if err != nil || len(items) == 0 {
		t.Fatalf("expected items on duplicated pack, got %v (%d)", err, len(items))
	}
	itemID := items[0].ID

	applyCtx := newChiCtxMulti(map[string]string{"id": pack.ID.String(), "item_id": itemID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/packs/"+pack.ID.String()+"/items/"+itemID.String(), nil))
	req = withUser(req, member.ID.String(), member.Username, member.Email, member.Role)
	rec := httptest.NewRecorder()
	h.DeleteItem(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member deleting group item want 403, got %d", rec.Code)
	}
}

// Group admins can delete items. Complements the member-denied case above.
func TestPackHandlerDeleteItem_GroupAdmin_Succeeds(t *testing.T) {
	h, _ := newPackHandler(t)
	admin, _, pack, q := seedDuplicatedGroupPack(t, "gda")

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: pack.ID, Lim: 10, Off: 0,
	})
	itemID := items[0].ID

	applyCtx := newChiCtxMulti(map[string]string{"id": pack.ID.String(), "item_id": itemID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/packs/"+pack.ID.String()+"/items/"+itemID.String(), nil))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.DeleteItem(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("admin deleting group item want 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// ─── Pack-metadata invariants ───────────────────────────────────────────────

// Renaming a group pack via PATCH /api/packs/{id} must NOT let the actor
// flip visibility or language, even if they send those fields. The host
// picker's language filter depends on this invariant.
func TestPackHandlerUpdate_GroupPack_PinsVisibilityAndLanguage(t *testing.T) {
	h, _ := newPackHandler(t)
	admin, _, pack, _ := seedDuplicatedGroupPack(t, "gpin")

	body := []byte(`{"name":"renamed","visibility":"public","language":"fr"}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var updated db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != "renamed" {
		t.Fatalf("rename didn't take, got %q", updated.Name)
	}
	if updated.Visibility != "private" {
		t.Fatalf("group pack visibility must stay private, got %q", updated.Visibility)
	}
	if updated.Language != pack.Language {
		t.Fatalf("group pack language must stay %q, got %q", pack.Language, updated.Language)
	}
}

// The rename-preserves-description fix: a PATCH that only sends {name}
// must leave description intact. Regression guard for a pre-existing bug
// that cleared description whenever the field was omitted.
func TestPackHandlerUpdate_NameOnly_PreservesDescription(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)
	desc := "keep this text"
	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:        "orig",
		Description: &desc,
		OwnerID:     pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility:  "private",
		Language:    "en",
	})
	if err != nil {
		t.Fatalf("seed pack: %v", err)
	}

	body := []byte(`{"name":"renamed"}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var updated db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Description == nil || *updated.Description != desc {
		t.Fatalf("description wiped: want %q, got %v", desc, updated.Description)
	}
}

// Explicit clear (description: "") must still null the column — distinguish
// "field omitted" (preserve) from "field set to empty" (clear to NULL).
func TestPackHandlerUpdate_EmptyDescription_ClearsIt(t *testing.T) {
	h, q := newPackHandler(t)
	admin := seedAdmin(t, q)
	desc := "initial"
	pack, _ := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:        "orig",
		Description: &desc,
		OwnerID:     pgtype.UUID{Bytes: admin.ID, Valid: true},
		Visibility:  "private",
		Language:    "en",
	})

	body := []byte(`{"name":"orig","description":""}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var updated db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Description != nil {
		t.Fatalf("expected description cleared to NULL, got %q", *updated.Description)
	}
}

// Silence the unused-return helper — keeps the suite tidy even when
// callers don't need the handler split right now.
var _ = newPackAndGroupPackHandlers

// ─── /api/packs list includes group packs ───────────────────────────────────

// A regular group member's GET /api/packs must include packs owned by the
// groups they belong to. Without this, the host picker and any /api/packs
// consumer is blind to duplicated group content and the
// duplicate→edit→host loop breaks for non-admins.
func TestPackHandlerList_IncludesGroupPacksForMember(t *testing.T) {
	h, _ := newPackHandler(t)
	_, group, pack, q := seedDuplicatedGroupPack(t, "gl")
	member := seedGroupUser(t, q, "gl_m")
	addMember(t, q, group, member, "member")

	req := httptest.NewRequest(http.MethodGet, "/api/packs?limit=100", nil)
	req = withUser(req, member.ID.String(), member.Username, member.Email, member.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data []db.GamePack `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	found := false
	for _, p := range body.Data {
		if p.ID == pack.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("group pack %s should appear in list for member", pack.ID)
	}
}

// Renaming a group pack writes an audit-log entry with action
// group.rename_pack scoped to the owning group — per spec, group-admin
// moderation actions flow through the audit log so platform admin can
// read them on demand.
func TestPackHandlerUpdate_GroupPackRename_WritesAudit(t *testing.T) {
	h, _ := newPackHandler(t)
	admin, group, pack, q := seedDuplicatedGroupPack(t, "gaudit")

	body := []byte(`{"name":"renamed-for-audit"}`)
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec.Code, rec.Body.String())
	}

	logs, err := q.ListAuditLogs(context.Background(), db.ListAuditLogsParams{
		Lim: 50, Off: 0,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	wantResource := "group:" + group.ID.String()
	found := false
	for _, l := range logs {
		if l.Action == "group.rename_pack" && l.Resource == wantResource {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected group.rename_pack audit entry for %s, got %d entries", wantResource, len(logs))
	}
}

// Renaming a group pack to the same value must NOT write an audit entry —
// the guard filters on an actual name delta to avoid spamming the log on
// descriptions-only edits or trivial PATCHes.
func TestPackHandlerUpdate_GroupPackNoRename_SkipsAudit(t *testing.T) {
	h, _ := newPackHandler(t)
	admin, group, pack, q := seedDuplicatedGroupPack(t, "gnoaud")

	body, _ := json.Marshal(map[string]string{"name": pack.Name}) // identical name
	applyCtx := newChiCtx("id", pack.ID.String())
	req := applyCtx(httptest.NewRequest(http.MethodPatch, "/api/packs/"+pack.ID.String(), bytes.NewBuffer(body)))
	req = withUser(req, admin.ID.String(), admin.Username, admin.Email, admin.Role)
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	logs, _ := q.ListAuditLogs(context.Background(), db.ListAuditLogsParams{Lim: 50, Off: 0})
	wantResource := "group:" + group.ID.String()
	for _, l := range logs {
		if l.Action == "group.rename_pack" && l.Resource == wantResource {
			t.Fatalf("unexpected audit entry for no-op rename")
		}
	}
}

// A non-member must NOT see the group pack in /api/packs — the visibility
// boundary has to hold even though the row exists.
func TestPackHandlerList_HidesGroupPackFromNonMember(t *testing.T) {
	h, _ := newPackHandler(t)
	_, _, pack, q := seedDuplicatedGroupPack(t, "glx")
	outsider := seedGroupUser(t, q, "glx_o")

	req := httptest.NewRequest(http.MethodGet, "/api/packs?limit=100", nil)
	req = withUser(req, outsider.ID.String(), outsider.Username, outsider.Email, outsider.Role)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body struct {
		Data []db.GamePack `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	for _, p := range body.Data {
		if p.ID == pack.ID {
			t.Fatalf("non-member should not see group pack %s", pack.ID)
		}
	}
}
