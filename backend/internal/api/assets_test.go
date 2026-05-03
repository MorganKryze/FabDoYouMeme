package api_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

func newAssetHandler(t *testing.T) *api.AssetHandler {
	t.Helper()
	cfg := &config.Config{MaxUploadSizeBytes: 1024 * 1024} // 1 MB limit for tests
	return api.NewAssetHandler(testutil.Pool(), cfg, nil)  // nil storage — only testing validation
}

// pngBase64 returns a minimal valid PNG as a base64 string.
func pngBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func uploadURLRequest(t *testing.T, mime string, sizeBytes int64, previewBase64 string) *http.Request {
	t.Helper()
	body := map[string]any{
		"pack_id":        "00000000-0000-0000-0000-000000000001",
		"item_id":        "00000000-0000-0000-0000-000000000002",
		"version_number": 1,
		"filename":       "test.png",
		"mime_type":      mime,
		"size_bytes":     sizeBytes,
		"preview_bytes":  previewBase64,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = withUser(req, "00000000-0000-0000-0000-000000000003", "assetuser", "asset@t.com", "player")
	return req
}

func TestUploadURL_MissingPreviewBytes_Returns400(t *testing.T) {
	// preview_bytes is now required — omitting it should return 400 bad_request
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "image/png", 512, "")
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "bad_request" {
		t.Errorf("want bad_request, got %s", resp["code"])
	}
}

func TestUploadURL_MagicBytesMismatch(t *testing.T) {
	// PNG bytes declared as image/jpeg → ValidateMIME returns error → 422 invalid_mime_type
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "image/jpeg", 512, pngBase64(t))
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422 for MIME mismatch, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "invalid_mime_type" {
		t.Errorf("want invalid_mime_type, got %s", resp["code"])
	}
}

func TestUploadURL_TooLarge(t *testing.T) {
	// File size exceeds MaxUploadSizeBytes → 422 file_too_large
	h := newAssetHandler(t)
	req := uploadURLRequest(t, "image/png", 2*1024*1024, "") // 2 MB > 1 MB limit
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["code"] != "file_too_large" {
		t.Errorf("want file_too_large, got %s", resp["code"])
	}
}

// TestAPI_DownloadURLAuthzMatrix is the P1.4 acceptance test from the
// 2026-04-10 review punch list. Pre-fix, /api/assets/download-url has zero
// authorization beyond "is the user logged in" — any player can download any
// media_key they can guess (finding 5.A). The matrix is a 3×3 grid:
//
//	             | private pack A | private pack B | public pack C
//	-------------|----------------|----------------|--------------
//	admin        |       200      |       200      |     200
//	owner of A   |       200      |       403      |     200
//	regular user |       403      |       403      |     200
//
// The diagonal-ish pattern is the contract under test. Pre-fix every cell is
// 200; post-fix five cells are 200 and four are 403.
func TestAPI_DownloadURLAuthzMatrix(t *testing.T) {
	q := db.New(testutil.Pool())
	ctx := context.Background()

	// Three users — admin, ownerA (owns pack A), regular (owns nothing).
	admin := testutil.MakeUser(t, "admin")
	ownerA := testutil.MakeUser(t, "player")
	regular := testutil.MakeUser(t, "player")

	// makePackWithMedia inserts a pack owned by `owner`, with one item + one
	// version, and returns the resulting media_key. Visibility = "private" or
	// "public" controls the third matrix axis.
	makePackWithMedia := func(t *testing.T, owner db.User, visibility string) string {
		t.Helper()
		pack, err := q.CreatePack(ctx, db.CreatePackParams{
			Name:       fmt.Sprintf("p_%s_%s", visibility, testutil.SeedName(t)),
			OwnerID:    pgtype.UUID{Bytes: owner.ID, Valid: true},
			Visibility: visibility,
			Language:   "en",
		})
		if err != nil {
			t.Fatalf("create pack (%s): %v", visibility, err)
		}
		item, err := q.CreateItem(ctx, db.CreateItemParams{
			PackID:         pack.ID,
			Name:           "test item",
			PayloadVersion: 1,
		})
		if err != nil {
			t.Fatalf("create item: %v", err)
		}
		key := fmt.Sprintf("media/%s.png", item.ID.String())
		if _, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
			ItemID:   item.ID,
			MediaKey: &key,
			Payload:  json.RawMessage(`{}`),
		}); err != nil {
			t.Fatalf("create item version: %v", err)
		}
		return key
	}

	mediaA := makePackWithMedia(t, ownerA, "private") // pack A — owned by ownerA
	// Pack B owned by a separate user so "ownerA" is NOT B's owner.
	ownerB := testutil.MakeUser(t, "player")
	mediaB := makePackWithMedia(t, ownerB, "private")
	// Pack C owned by a third user but visibility=public — anyone may read.
	ownerC := testutil.MakeUser(t, "player")
	mediaC := makePackWithMedia(t, ownerC, "public")

	// Build a handler with a fake storage so PresignDownload always returns a
	// stub URL. We're testing authz, not the storage layer.
	cfg := &config.Config{}
	store := testutil.NewFakeStorage()
	h := api.NewAssetHandler(testutil.Pool(), cfg, store)

	type cell struct {
		name     string
		caller   db.User
		mediaKey string
		want     int
	}
	cells := []cell{
		// admin row — all three must succeed.
		{"admin downloads private pack A", admin, mediaA, http.StatusOK},
		{"admin downloads private pack B", admin, mediaB, http.StatusOK},
		{"admin downloads public pack C", admin, mediaC, http.StatusOK},
		// ownerA row — own pack and public pack OK, foreign private pack denied.
		{"ownerA downloads own private pack A", ownerA, mediaA, http.StatusOK},
		{"ownerA downloads other private pack B", ownerA, mediaB, http.StatusForbidden},
		{"ownerA downloads public pack C", ownerA, mediaC, http.StatusOK},
		// regular user row — only the public pack OK.
		{"regular downloads private pack A", regular, mediaA, http.StatusForbidden},
		{"regular downloads private pack B", regular, mediaB, http.StatusForbidden},
		{"regular downloads public pack C", regular, mediaC, http.StatusOK},
	}

	for _, c := range cells {
		t.Run(c.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"media_key": c.mediaKey})
			req := httptest.NewRequest(http.MethodPost, "/api/assets/download-url", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withUser(req, c.caller.ID.String(), c.caller.Username, c.caller.Email, c.caller.Role)
			rec := httptest.NewRecorder()

			h.DownloadURL(rec, req)

			if rec.Code != c.want {
				t.Errorf("want %d, got %d — body: %s", c.want, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestCanDownloadMedia_GroupMember covers the new authz branch added when
// the studio reported "players can't see images during gameplay" for
// rooms backed by a group-owned pack. The pack belongs to a group whose
// members must be allowed to download the media even though the pack is
// private and not their personal property.
func TestCanDownloadMedia_GroupMember(t *testing.T) {
	q := db.New(testutil.Pool())
	ctx := context.Background()

	owner := testutil.MakeUser(t, "player")  // group admin who minted the pack
	member := testutil.MakeUser(t, "player") // joined the group later
	stranger := testutil.MakeUser(t, "player")

	grp, err := q.CreateGroup(ctx, db.CreateGroupParams{
		Name:           "g_" + testutil.SeedName(t),
		Description:    "auth coverage",
		Language:       "en",
		Classification: "sfw",
		QuotaBytes:     500 * 1024 * 1024,
		MemberCap:      100,
		CreatedBy:      pgtype.UUID{Bytes: owner.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, m := range []struct {
		uid  uuid.UUID
		role string
	}{{owner.ID, "admin"}, {member.ID, "member"}} {
		if _, err := q.CreateMembership(ctx, db.CreateMembershipParams{
			GroupID: grp.ID, UserID: m.uid, Role: m.role,
		}); err != nil {
			t.Fatalf("create membership %s: %v", m.role, err)
		}
	}

	// Group pack: insert directly so we can set group_id with no owner_id.
	// The duplication endpoint is the production path for creating these,
	// but its full surface (ratio, classification, source pack) is
	// orthogonal to the authz predicate under test.
	pack, err := q.CreatePack(ctx, db.CreatePackParams{
		Name:       "p_" + testutil.SeedName(t),
		Visibility: "private",
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("create base pack: %v", err)
	}
	if _, err := testutil.Pool().Exec(ctx,
		"UPDATE game_packs SET group_id = $1, owner_id = NULL WHERE id = $2",
		grp.ID, pack.ID); err != nil {
		t.Fatalf("re-attribute pack to group: %v", err)
	}
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID: pack.ID, Name: "x", PayloadVersion: 1,
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}
	mediaKey := fmt.Sprintf("media/%s.png", item.ID)
	if _, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID: item.ID, MediaKey: &mediaKey, Payload: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("create version: %v", err)
	}

	cases := []struct {
		name string
		uid  uuid.UUID
		want bool
	}{
		{"group admin (member by membership)", owner.ID, true},
		{"regular group member", member.ID, true},
		{"stranger outside the group", stranger.ID, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mk := mediaKey
			ok, err := q.CanUserDownloadMedia(ctx, db.CanUserDownloadMediaParams{
				MediaKey: &mk,
				UserID:   c.uid,
			})
			if err != nil {
				t.Fatalf("CanUserDownloadMedia: %v", err)
			}
			if ok != c.want {
				t.Errorf("want %v, got %v", c.want, ok)
			}
		})
	}
}

// TestCanDownloadMedia_RoomParticipant: any player in the room can read
// every media in any pack assigned to that room — even items not yet
// played in a round (covers hand-deal flows like meme-showdown where each
// player sees their dealt cards before they land). Stranger with no
// relationship to the room or pack remains 403'd.
func TestCanDownloadMedia_RoomParticipant(t *testing.T) {
	q := db.New(testutil.Pool())
	ctx := context.Background()

	host := testutil.MakeUser(t, "player")
	guest := testutil.MakeUser(t, "player") // joined the room, no other relationship
	pack := testutil.MakePack(t, host, 1)
	// Lock the pack down so only the room-participant branch can grant access.
	if _, err := q.UpdatePack(ctx, db.UpdatePackParams{
		ID: pack.ID, Name: pack.Name, Description: pack.Description,
		Visibility: "private", Language: pack.Language,
	}); err != nil {
		t.Fatalf("private update: %v", err)
	}

	room := testutil.MakeRoom(t, host, pack, "")
	if _, err := q.AddRoomPlayer(ctx, db.AddRoomPlayerParams{
		RoomID: room.ID, UserID: pgtype.UUID{Bytes: guest.ID, Valid: true},
	}); err != nil {
		t.Fatalf("add room player: %v", err)
	}
	// Read the seeded version directly — MakePack leaves current_version_id
	// unpromoted, which makes the public list return NULL media_key.
	var mk string
	if err := testutil.Pool().QueryRow(ctx,
		`SELECT giv.media_key
		 FROM game_item_versions giv JOIN game_items gi ON gi.id = giv.item_id
		 WHERE gi.pack_id = $1 LIMIT 1`, pack.ID).Scan(&mk); err != nil {
		t.Fatalf("read seeded version: %v", err)
	}

	// No round yet — the broader room_packs scope must still allow the
	// participant. This is the regression guard for "all people in the
	// game room should have access" including hand-deal previews.
	ok, err := q.CanUserDownloadMedia(ctx, db.CanUserDownloadMediaParams{
		MediaKey: &mk, UserID: guest.ID,
	})
	if err != nil {
		t.Fatalf("CanUserDownloadMedia (participant): %v", err)
	}
	if !ok {
		t.Errorf("room participant should be allowed to download any media in the room's pack — even before any round has played it")
	}

	stranger := testutil.MakeUser(t, "player")
	ok, err = q.CanUserDownloadMedia(ctx, db.CanUserDownloadMediaParams{
		MediaKey: &mk, UserID: stranger.ID,
	})
	if err != nil {
		t.Fatalf("CanUserDownloadMedia (stranger): %v", err)
	}
	if ok {
		t.Errorf("stranger with no room or group link should be denied")
	}
}

func TestUploadURL_Unauthenticated(t *testing.T) {
	h := newAssetHandler(t)
	body := `{"mime_type":"image/png","size_bytes":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/assets/upload-url", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.UploadURL(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rec.Code)
	}
}
