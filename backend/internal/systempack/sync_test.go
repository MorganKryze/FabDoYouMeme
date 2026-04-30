// backend/internal/systempack/sync_test.go
package systempack_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/systempack"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/testutil"
)

// fakeStorage records every Upload call and lets tests force a failure.
type fakeStorage struct {
	mu      sync.Mutex
	uploads map[string][]byte
	failOn  string // if non-empty, Upload returns an error when key contains this
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{uploads: map[string][]byte{}}
}

func (f *fakeStorage) Upload(_ context.Context, key string, body io.Reader, _ string, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOn != "" && bytes.Contains([]byte(key), []byte(f.failOn)) {
		return errSimulated
	}
	b, _ := io.ReadAll(body)
	f.uploads[key] = b
	return nil
}
func (f *fakeStorage) PresignUpload(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (f *fakeStorage) PresignDownload(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}
func (f *fakeStorage) Download(_ context.Context, _ string) (io.ReadCloser, string, int64, error) {
	return nil, "", 0, nil
}
func (f *fakeStorage) Delete(_ context.Context, _ string) error         { return nil }
func (f *fakeStorage) Purge(_ context.Context, _ string) (int64, error) { return 0, nil }
func (f *fakeStorage) Stats(_ context.Context, _ string) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeStorage) Probe(_ context.Context) error { return nil }

type simulatedError string

func (e simulatedError) Error() string { return string(e) }

const errSimulated = simulatedError("simulated upload failure")

// systemPackID is the fixed sentinel UUID for the demo pack.
// Must match systempack.SystemPackID.
func systemPackID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse("00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("parse sentinel: %v", err)
	}
	return id
}

// systemTextPackID is the sibling sentinel for the bundled text pack.
// Must match systempack.SystemTextPackID.
func systemTextPackID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse("00000000-0000-0000-0000-000000000002")
	if err != nil {
		t.Fatalf("parse sentinel: %v", err)
	}
	return id
}

// resetTextSystemPack mirrors resetSystemPack for the text demo pack so each
// text-sync test starts from a clean slate.
func resetTextSystemPack(t *testing.T, q *db.Queries) {
	t.Helper()
	ctx := context.Background()
	items, err := q.ListItemsForPack(ctx, db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 10000, Off: 0,
	})
	if err != nil {
		return
	}
	for _, it := range items {
		versions, _ := q.ListVersionsForItem(ctx, it.ID)
		for _, v := range versions {
			_ = q.HardDeleteVersion(ctx, v.ID)
		}
	}
}

// resetSystemPack clears the items from the previous test so each test starts
// from a clean slate. The pack row is left in place — Sync will upsert it
// again on the next call. Items are hard-deleted here because nothing else
// references them in these tests (no rounds); hard-delete is simpler than
// soft-delete for test teardown.
func resetSystemPack(t *testing.T, q *db.Queries) {
	t.Helper()
	ctx := context.Background()
	items, err := q.ListItemsForPack(ctx, db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 10000, Off: 0,
	})
	if err != nil {
		// If the system pack doesn't exist yet, there's nothing to clean.
		return
	}
	for _, it := range items {
		versions, _ := q.ListVersionsForItem(ctx, it.ID)
		for _, v := range versions {
			_ = q.HardDeleteVersion(ctx, v.ID)
		}
	}
	// Also collect soft-deleted items (ListItemsForPack filters them out) by
	// relying on the fact that we only ever have a handful per test. The
	// simplest route is to run the sync with a clean map and let the
	// hard-delete-on-version cascade handle the rest — but CreateItem takes a
	// non-nullable FK, so we must also drop the items themselves.
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestSync_EmptyFolder_CreatesPackWithZeroItems: an empty embedded FS still
// creates the pack row so the UI can show a "Demo Pack" entry with no items.
func TestSync_EmptyFolder_CreatesPackWithZeroItems(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	store := newFakeStorage()

	fs := fstest.MapFS{
		"demo-pack": &fstest.MapFile{Mode: 0o755 | 1<<31}, // directory marker
	}

	err := systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	pack, err := q.GetPackByID(context.Background(), systemPackID(t))
	if err != nil {
		t.Fatalf("system pack not created: %v", err)
	}
	if !pack.IsSystem {
		t.Errorf("want is_system=true, got false")
	}
	if pack.Visibility != "public" {
		t.Errorf("want visibility=public, got %q", pack.Visibility)
	}
	if pack.OwnerID.Valid {
		t.Errorf("want owner_id=NULL, got %v", pack.OwnerID)
	}

	// Retire any items that may have leaked from prior tests.
	resetSystemPack(t, q)
	// Re-sync on an empty fs should leave zero visible items.
	if err := systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	items, err := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

func TestSync_NewFile_CreatesItemV1AndUploads(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetSystemPack(t, q)
	store := newFakeStorage()

	data := []byte("fake-png-bytes-A")
	fs := fstest.MapFS{
		"demo-pack/cat.png": &fstest.MapFile{Data: data},
	}

	if err := systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Name != "cat" {
		t.Errorf("want name=cat, got %q", items[0].Name)
	}
	if items[0].MediaKey == nil || *items[0].MediaKey == "" {
		t.Errorf("want media_key set, got nil/empty")
	}
	if len(store.uploads) != 1 {
		t.Errorf("want 1 upload, got %d", len(store.uploads))
	}
	for k, b := range store.uploads {
		if !bytes.Equal(b, data) {
			t.Errorf("uploaded bytes differ at key %s", k)
		}
	}
	if sha256Hex(data) == "" {
		t.Error("sha256Hex returned empty")
	}
}

func TestSync_UnchangedFile_NoOp(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetSystemPack(t, q)
	store := newFakeStorage()

	data := []byte("fake-png-bytes-B")
	fs := fstest.MapFS{
		"demo-pack/doge.png": &fstest.MapFile{Data: data},
	}

	if err := systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	uploadsAfterFirst := len(store.uploads)

	if err := systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(store.uploads) != uploadsAfterFirst {
		t.Errorf("want no new uploads on unchanged sync, got %d→%d",
			uploadsAfterFirst, len(store.uploads))
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Errorf("want 1 item after idempotent sync, got %d", len(items))
	}
}

func TestSync_ModifiedFile_CreatesNewVersion(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetSystemPack(t, q)
	store := newFakeStorage()

	fsv1 := fstest.MapFS{
		"demo-pack/sunset.png": &fstest.MapFile{Data: []byte("v1-bytes")},
	}
	if err := systempack.SyncFS(context.Background(), q, store, fsv1, testutil.NewLogger()); err != nil {
		t.Fatalf("v1 sync: %v", err)
	}

	fsv2 := fstest.MapFS{
		"demo-pack/sunset.png": &fstest.MapFile{Data: []byte("v2-bytes")},
	}
	if err := systempack.SyncFS(context.Background(), q, store, fsv2, testutil.NewLogger()); err != nil {
		t.Fatalf("v2 sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item (same filename), got %d", len(items))
	}
	versions, err := q.ListVersionsForItem(context.Background(), items[0].ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("want 2 versions after modify, got %d", len(versions))
	}
}

func TestSync_RemovedFile_SoftDeletesItem(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetSystemPack(t, q)
	store := newFakeStorage()

	fsBefore := fstest.MapFS{
		"demo-pack/keep.png":   &fstest.MapFile{Data: []byte("stay")},
		"demo-pack/remove.png": &fstest.MapFile{Data: []byte("gone")},
	}
	if err := systempack.SyncFS(context.Background(), q, store, fsBefore, testutil.NewLogger()); err != nil {
		t.Fatalf("before sync: %v", err)
	}

	fsAfter := fstest.MapFS{
		"demo-pack/keep.png": &fstest.MapFile{Data: []byte("stay")},
	}
	if err := systempack.SyncFS(context.Background(), q, store, fsAfter, testutil.NewLogger()); err != nil {
		t.Fatalf("after sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Errorf("want 1 item visible after retire, got %d", len(items))
	}
	if len(items) == 1 && items[0].Name != "keep" {
		t.Errorf("wrong item survived: %q", items[0].Name)
	}
}

// ── Text demo pack ───────────────────────────────────────────────────────

// textFS builds a fstest.MapFS containing demo-text-pack/items.json with the
// given JSON body. Tests use this to drive SyncTextFS directly.
func textFS(body string) fstest.MapFS {
	return fstest.MapFS{
		"demo-text-pack/items.json": &fstest.MapFile{Data: []byte(body)},
	}
}

func TestSyncText_MissingFile_CreatesPackWithZeroItems(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	// fstest.MapFS without the items.json entry — readTextEntries treats this
	// as "no entries" so the pack row is upserted with zero items.
	fs := fstest.MapFS{}

	if err := systempack.SyncTextFS(context.Background(), q, fs, testutil.NewLogger()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	pack, err := q.GetPackByID(context.Background(), systemTextPackID(t))
	if err != nil {
		t.Fatalf("text pack not created: %v", err)
	}
	if !pack.IsSystem {
		t.Errorf("want is_system=true, got false")
	}
	if pack.Name != "Demo Text Pack" {
		t.Errorf("want Name=Demo Text Pack, got %q", pack.Name)
	}
	if pack.OwnerID.Valid {
		t.Errorf("want owner_id=NULL, got %v", pack.OwnerID)
	}
	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

func TestSyncText_NewEntry_CreatesItemV2WithPayload(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	body := `[{"name":"hello","text":"Hello world"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(body), testutil.NewLogger()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Name != "hello" {
		t.Errorf("want name=hello, got %q", items[0].Name)
	}
	if items[0].PayloadVersion != 2 {
		t.Errorf("want payload_version=2, got %d", items[0].PayloadVersion)
	}
	if items[0].MediaKey != nil {
		t.Errorf("want media_key=nil for text item, got %v", items[0].MediaKey)
	}
	// payload should embed both text and a sha256
	if !bytes.Contains(items[0].Payload, []byte("Hello world")) {
		t.Errorf("payload missing text: %s", items[0].Payload)
	}
	if !bytes.Contains(items[0].Payload, []byte("sha256")) {
		t.Errorf("payload missing sha256: %s", items[0].Payload)
	}
}

func TestSyncText_UnchangedEntry_NoOp(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	body := `[{"name":"keep","text":"steady"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(body), testutil.NewLogger()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	versionsBefore, _ := q.ListVersionsForItem(context.Background(), items[0].ID)

	if err := systempack.SyncTextFS(context.Background(), q, textFS(body), testutil.NewLogger()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	versionsAfter, _ := q.ListVersionsForItem(context.Background(), items[0].ID)
	if len(versionsAfter) != len(versionsBefore) {
		t.Errorf("want no new versions on idempotent sync, got %d→%d", len(versionsBefore), len(versionsAfter))
	}
}

func TestSyncText_ModifiedEntry_CreatesNewVersion(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	v1 := `[{"name":"line","text":"first take"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(v1), testutil.NewLogger()); err != nil {
		t.Fatalf("v1 sync: %v", err)
	}
	v2 := `[{"name":"line","text":"second take"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(v2), testutil.NewLogger()); err != nil {
		t.Fatalf("v2 sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item (same name), got %d", len(items))
	}
	versions, err := q.ListVersionsForItem(context.Background(), items[0].ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("want 2 versions after modify, got %d", len(versions))
	}
}

func TestSyncText_RemovedEntry_SoftDeletesItem(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	before := `[{"name":"keep","text":"stay"},{"name":"remove","text":"gone"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(before), testutil.NewLogger()); err != nil {
		t.Fatalf("before sync: %v", err)
	}
	after := `[{"name":"keep","text":"stay"}]`
	if err := systempack.SyncTextFS(context.Background(), q, textFS(after), testutil.NewLogger()); err != nil {
		t.Fatalf("after sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemTextPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Errorf("want 1 visible item after retire, got %d", len(items))
	}
	if len(items) == 1 && items[0].Name != "keep" {
		t.Errorf("wrong item survived: %q", items[0].Name)
	}
}

func TestSyncText_InvalidJSON_ReturnsError(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetTextSystemPack(t, q)

	err := systempack.SyncTextFS(context.Background(), q, textFS("not-json"), testutil.NewLogger())
	if err == nil {
		t.Fatalf("want error on invalid JSON, got nil")
	}
}

// ── Prompt demo pack ─────────────────────────────────────────────────────

func systemPromptPackID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse("00000000-0000-0000-0000-000000000004")
	if err != nil {
		t.Fatalf("parse sentinel: %v", err)
	}
	return id
}

func resetPromptSystemPack(t *testing.T, q *db.Queries) {
	t.Helper()
	ctx := context.Background()
	items, err := q.ListItemsForPack(ctx, db.ListItemsForPackParams{
		PackID: systemPromptPackID(t), Lim: 10000, Off: 0,
	})
	if err != nil {
		return
	}
	for _, it := range items {
		versions, _ := q.ListVersionsForItem(ctx, it.ID)
		for _, v := range versions {
			_ = q.HardDeleteVersion(ctx, v.ID)
		}
	}
}

func promptFS(body string) fstest.MapFS {
	return fstest.MapFS{
		"demo-prompt-pack/items.json": &fstest.MapFile{Data: []byte(body)},
	}
}

func TestSyncPrompt_NewEntry_CreatesItemV4WithPayload(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetPromptSystemPack(t, q)

	body := `[{"name":"trust-keys","prefix":"I would never trust ","suffix":" with my keys."}]`
	if err := systempack.SyncPromptFS(context.Background(), q, promptFS(body), testutil.NewLogger()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPromptPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].PayloadVersion != 4 {
		t.Errorf("want payload_version=4, got %d", items[0].PayloadVersion)
	}
	if items[0].MediaKey != nil {
		t.Errorf("want media_key=nil for prompt item, got %v", items[0].MediaKey)
	}
	if !bytes.Contains(items[0].Payload, []byte("I would never trust")) {
		t.Errorf("payload missing prefix: %s", items[0].Payload)
	}
	if !bytes.Contains(items[0].Payload, []byte("with my keys.")) {
		t.Errorf("payload missing suffix: %s", items[0].Payload)
	}
	if !bytes.Contains(items[0].Payload, []byte("sha256")) {
		t.Errorf("payload missing sha256: %s", items[0].Payload)
	}
}

func TestSyncPrompt_BlankOnlyEntry_Skipped(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetPromptSystemPack(t, q)

	// Both prefix and suffix empty means the "sentence" is just the blank —
	// the sync logs a warning and skips so the room can never deal a useless
	// item to a player.
	body := `[
	  {"name":"only-blank","prefix":"","suffix":""},
	  {"name":"valid","prefix":"It begins with ","suffix":""}
	]`
	if err := systempack.SyncPromptFS(context.Background(), q, promptFS(body), testutil.NewLogger()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPromptPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item (blank-only entry skipped), got %d", len(items))
	}
	if items[0].Name != "valid" {
		t.Errorf("want only the valid entry to land, got name=%q", items[0].Name)
	}
}

func TestSyncPrompt_ModifiedEntry_CreatesNewVersion(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetPromptSystemPack(t, q)

	v1 := `[{"name":"line","prefix":"first ","suffix":" take"}]`
	if err := systempack.SyncPromptFS(context.Background(), q, promptFS(v1), testutil.NewLogger()); err != nil {
		t.Fatalf("v1 sync: %v", err)
	}
	v2 := `[{"name":"line","prefix":"second ","suffix":" take"}]`
	if err := systempack.SyncPromptFS(context.Background(), q, promptFS(v2), testutil.NewLogger()); err != nil {
		t.Fatalf("v2 sync: %v", err)
	}

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPromptPackID(t), Lim: 100, Off: 0,
	})
	if len(items) != 1 {
		t.Fatalf("want 1 item (same name), got %d", len(items))
	}
	versions, err := q.ListVersionsForItem(context.Background(), items[0].ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("want 2 versions after edit, got %d", len(versions))
	}
}

func TestSync_UploadFailure_NoOrphanRow(t *testing.T) {
	pool := testutil.Pool()
	q := db.New(pool)
	resetSystemPack(t, q)
	store := newFakeStorage()
	store.failOn = "cat" // fail the Upload for cat.png

	fs := fstest.MapFS{
		"demo-pack/cat.png": &fstest.MapFile{Data: []byte("anything")},
	}

	// Sync returns nil (errors are logged, not returned) but must not leave a
	// visible DB row pointing at a missing upload.
	_ = systempack.SyncFS(context.Background(), q, store, fs, testutil.NewLogger())

	items, _ := q.ListItemsForPack(context.Background(), db.ListItemsForPackParams{
		PackID: systemPackID(t), Lim: 100, Off: 0,
	})
	for _, it := range items {
		if it.Name == "cat" {
			t.Errorf("orphan item row for cat after upload failure")
		}
	}
}
