package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

func newGroupPackHandler(t *testing.T) (*api.GroupPackHandler, *db.Queries) {
	t.Helper()
	pool := testutil.Pool()
	cfg := &config.Config{MaxGroupsPerUser: 50, MaxGroupMembershipsPerUser: 50}
	return api.NewGroupPackHandler(pool, cfg), db.New(pool)
}

// seedSourcePack inserts a non-group pack owned by owner with the given
// classification and language + one item with a payload so Duplicate has
// something to copy.
func seedSourcePack(t *testing.T, q *db.Queries, owner db.User, classification, language string) db.GamePack {
	t.Helper()
	pack, err := q.CreatePack(context.Background(), db.CreatePackParams{
		Name:       "src_" + uuid.NewString()[:8],
		OwnerID:    pgtype.UUID{Bytes: owner.ID, Valid: true},
		Visibility: "private",
		Language:   language,
	})
	if err != nil {
		t.Fatalf("create pack: %v", err)
	}
	if _, err := testutil.Pool().Exec(context.Background(),
		"UPDATE game_packs SET classification = $2 WHERE id = $1", pack.ID, classification); err != nil {
		t.Fatalf("set classification: %v", err)
	}
	pack.Classification = classification
	item, err := q.CreateItem(context.Background(), db.CreateItemParams{
		PackID: pack.ID, Name: "i1", PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	ver, err := q.CreateItemVersion(context.Background(), db.CreateItemVersionParams{
		ItemID: item.ID, Payload: json.RawMessage(`{"caption":"hi"}`),
	})
	if err != nil {
		t.Fatalf("create version: %v", err)
	}
	if _, err := q.SetCurrentVersion(context.Background(), db.SetCurrentVersionParams{
		ID: item.ID, CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		t.Fatalf("set current version: %v", err)
	}
	return pack
}

// seedGroupWithClassification is a sibling of seedGroupWithAdmin that sets a
// specific classification + language on the group.
func seedGroupWithClassification(t *testing.T, q *db.Queries, slug, classification, language string) (db.User, db.Group) {
	t.Helper()
	a := seedGroupUser(t, q, slug)
	g, err := q.CreateGroup(context.Background(), db.CreateGroupParams{
		Name:           "Gp_" + slug + "_" + uuid.NewString()[:8],
		Description:    "seed",
		Language:       language,
		Classification: classification,
		QuotaBytes:     500 * 1024 * 1024,
		MemberCap:      100,
		CreatedBy:      pgUUID(a.ID),
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := q.CreateMembership(context.Background(), db.CreateMembershipParams{
		GroupID: g.ID, UserID: a.ID, Role: "admin",
	}); err != nil {
		t.Fatalf("create admin membership: %v", err)
	}
	return a, g
}

func duplicateAs(t *testing.T, h *api.GroupPackHandler, u db.User, gid, sourceID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"source_pack_id": sourceID.String()})
	applyCtx := newChiCtx("id", gid.String())
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+gid.String()+"/packs/duplicate", bytes.NewBuffer(body)))
	req = withUser(req, u.ID.String(), u.Username, u.Email, u.Role)
	rec := httptest.NewRecorder()
	h.Duplicate(rec, req)
	return rec
}

// ─── Duplicate ──────────────────────────────────────────────────────────────

func TestDuplicate_SFWHappyPath(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "dup1", "sfw", "en")
	src := seedSourcePack(t, q, a, "sfw", "en")

	rec := duplicateAs(t, h, a, g.ID, src.ID)
	if rec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var pack db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &pack)
	if !pack.GroupID.Valid || pack.GroupID.Bytes != g.ID {
		t.Fatalf("expected group_id=%v on duplicated pack, got %+v", g.ID, pack.GroupID)
	}
	if pack.Name != src.Name {
		t.Fatalf("name mismatch: want %q got %q", src.Name, pack.Name)
	}
}

func TestDuplicate_NSFWIntoSFWGroupQueues(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "dup2", "sfw", "en")
	src := seedSourcePack(t, q, a, "nsfw", "en")

	rec := duplicateAs(t, h, a, g.ID, src.ID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d — body: %s", rec.Code, rec.Body.String())
	}
	// Group stays SFW until admin accepts.
	grp, _ := q.GetGroupByID(context.Background(), g.ID)
	if grp.Classification != "sfw" {
		t.Fatalf("group should stay sfw until accept, got %q", grp.Classification)
	}
}

func TestDuplicate_NonMemberForbidden(t *testing.T) {
	h, q := newGroupPackHandler(t)
	_, g := seedGroupWithClassification(t, q, "dup3", "sfw", "en")
	b := seedGroupUser(t, q, "dup3_b")
	src := seedSourcePack(t, q, b, "sfw", "en")

	rec := duplicateAs(t, h, b, g.ID, src.ID)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
}

func TestDuplicate_LanguageMismatch(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "dup4", "sfw", "en")
	src := seedSourcePack(t, q, a, "sfw", "fr") // mismatch

	rec := duplicateAs(t, h, a, g.ID, src.ID)
	if rec.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", rec.Code)
	}
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env["code"] != "language_mismatch" {
		t.Fatalf("want code language_mismatch, got %v", env["code"])
	}
}

// ─── Accept / Reject queue ──────────────────────────────────────────────────

func TestAcceptPending_ForceRelabelsGroupToNSFW(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "acc", "sfw", "en")
	src := seedSourcePack(t, q, a, "nsfw", "en")
	// Enqueue by firing the duplicate path first.
	rec := duplicateAs(t, h, a, g.ID, src.ID)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("enqueue failed: %d", rec.Code)
	}
	var resp struct {
		Pending db.GroupDuplicationPending `json:"pending"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "queueID": resp.Pending.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost,
		"/api/groups/"+g.ID.String()+"/duplication-queue/"+resp.Pending.ID.String()+"/accept", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec2 := httptest.NewRecorder()
	h.AcceptPending(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rec2.Code, rec2.Body.String())
	}
	grp, _ := q.GetGroupByID(context.Background(), g.ID)
	if grp.Classification != "nsfw" {
		t.Fatalf("expected group force-relabeled nsfw, got %q", grp.Classification)
	}
}

func TestRejectPending_KeepsGroupLabel(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "rej", "sfw", "en")
	src := seedSourcePack(t, q, a, "nsfw", "en")
	rec := duplicateAs(t, h, a, g.ID, src.ID)
	var resp struct {
		Pending db.GroupDuplicationPending `json:"pending"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "queueID": resp.Pending.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost,
		"/api/groups/"+g.ID.String()+"/duplication-queue/"+resp.Pending.ID.String()+"/reject", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec2 := httptest.NewRecorder()
	h.RejectPending(rec2, req)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d — body: %s", rec2.Code, rec2.Body.String())
	}
	grp, _ := q.GetGroupByID(context.Background(), g.ID)
	if grp.Classification != "sfw" {
		t.Fatalf("expected group to remain sfw on reject, got %q", grp.Classification)
	}
}

// ─── Item CRUD ──────────────────────────────────────────────────────────────

func TestAddItem_MemberCreates(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "ai", "sfw", "en")
	src := seedSourcePack(t, q, a, "sfw", "en")
	rec := duplicateAs(t, h, a, g.ID, src.ID)
	var pack db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &pack)

	body, _ := json.Marshal(map[string]any{
		"name": "new_item", "payload_version": 1, "payload": json.RawMessage(`{"caption":"yo"}`),
	})
	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "packID": pack.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/packs/"+pack.ID.String()+"/items", bytes.NewBuffer(body)))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec2 := httptest.NewRecorder()
	h.AddItem(rec2, req)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rec2.Code, rec2.Body.String())
	}
	var item db.GameItem
	_ = json.Unmarshal(rec2.Body.Bytes(), &item)
	// Audit pair stamped?
	row, _ := q.GetItemByID(context.Background(), item.ID)
	if !row.LastEditorUserID.Valid || row.LastEditorUserID.Bytes != a.ID {
		t.Fatalf("expected last_editor_user_id stamped, got %+v", row.LastEditorUserID)
	}
}

func TestDeletePack_AdminOnly(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "dp", "sfw", "en")
	src := seedSourcePack(t, q, a, "sfw", "en")
	rec := duplicateAs(t, h, a, g.ID, src.ID)
	var pack db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &pack)

	// Member (non-admin) cannot delete.
	b := seedGroupUser(t, q, "dp_b")
	addMember(t, q, g, b, "member")
	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "packID": pack.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/packs/"+pack.ID.String(), nil))
	req = withUser(req, b.ID.String(), b.Username, b.Email, b.Role)
	rec2 := httptest.NewRecorder()
	h.DeletePack(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("member delete want 403, got %d", rec2.Code)
	}

	// Admin succeeds.
	req2 := applyCtx(httptest.NewRequest(http.MethodDelete, "/api/groups/"+g.ID.String()+"/packs/"+pack.ID.String(), nil))
	req2 = withUser(req2, a.ID.String(), a.Username, a.Email, a.Role)
	rec3 := httptest.NewRecorder()
	h.DeletePack(rec3, req2)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("admin delete want 204, got %d", rec3.Code)
	}
}

func TestEvict_NotifiesGroup(t *testing.T) {
	h, q := newGroupPackHandler(t)
	a, g := seedGroupWithClassification(t, q, "ev", "sfw", "en")
	src := seedSourcePack(t, q, a, "sfw", "en")
	rec := duplicateAs(t, h, a, g.ID, src.ID)
	var pack db.GamePack
	_ = json.Unmarshal(rec.Body.Bytes(), &pack)

	applyCtx := newChiCtxMulti(map[string]string{"id": g.ID.String(), "packID": pack.ID.String()})
	req := applyCtx(httptest.NewRequest(http.MethodPost, "/api/groups/"+g.ID.String()+"/packs/"+pack.ID.String()+"/evict", nil))
	req = withUser(req, a.ID.String(), a.Username, a.Email, a.Role)
	rec2 := httptest.NewRecorder()
	h.Evict(rec2, req)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec2.Code)
	}
	rows, err := q.ListGroupNotifications(context.Background(), db.ListGroupNotificationsParams{
		GroupID: g.ID, Lim: 10, Off: 0,
	})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(rows) != 1 || rows[0].Type != "pack_evicted" {
		t.Fatalf("expected one pack_evicted notification, got %+v", rows)
	}
}
