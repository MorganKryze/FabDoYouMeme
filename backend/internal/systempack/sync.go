// backend/internal/systempack/sync.go
package systempack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// SystemPackID is the fixed sentinel UUID of the bundled image demo pack.
// Stable across reboots so the RustFS object keys (packs/{packID}/...) are
// also stable, which in turn keeps round history pointing at valid blobs.
var SystemPackID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// SystemTextPackID is the sibling sentinel for the bundled text demo pack.
// Same stability contract as SystemPackID — text items don't store assets,
// but rounds still reference them by item id.
var SystemTextPackID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

// SystemTextPackFRID is the sentinel for the bundled French text demo pack.
// Text content is locale-bound, so each language ships its own pack rather
// than a translated copy of the English one.
var SystemTextPackFRID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

// SystemPromptPackID and SystemPromptPackFRID seed the bundled prompt
// (sentence-with-blank, payload v4) packs in EN and FR.
var (
	SystemPromptPackID   = uuid.MustParse("00000000-0000-0000-0000-000000000004")
	SystemPromptPackFRID = uuid.MustParse("00000000-0000-0000-0000-000000000005")
)

// SystemFillerPackID and SystemFillerPackFRID seed the bundled filler
// (short noun-phrase, payload v3) packs in EN and FR.
var (
	SystemFillerPackID   = uuid.MustParse("00000000-0000-0000-0000-000000000006")
	SystemFillerPackFRID = uuid.MustParse("00000000-0000-0000-0000-000000000007")
)

const (
	systemPackName              = "Demo Pack"
	systemPackDescription       = "Bundled sample images to get you started."
	demoPackDir                 = "demo-pack"
	systemTextPackName          = "Demo Text Pack"
	systemTextPackDescription   = "Bundled sample text prompts to get you started."
	demoTextPackDir             = "demo-text-pack"
	systemTextPackFRName        = "Pack de textes de démo"
	systemTextPackFRDescription = "Prompts texte prêts à l'emploi pour commencer."
	demoTextPackFRDir           = "demo-text-pack-fr"
	demoTextPackFile            = "items.json"

	// Prompt packs (sentence-with-blank, payload v4).
	systemPromptPackName          = "Demo Prompt Pack"
	systemPromptPackDescription   = "Bundled fill-in-the-blank sentences for prompt games."
	demoPromptPackDir             = "demo-prompt-pack"
	systemPromptPackFRName        = "Pack de prompts de démo"
	systemPromptPackFRDescription = "Phrases à compléter prêtes à l'emploi pour les jeux prompt."
	demoPromptPackFRDir           = "demo-prompt-pack-fr"

	// Filler packs (short noun-phrase cards, payload v3).
	systemFillerPackName          = "Demo Filler Pack"
	systemFillerPackDescription   = "Bundled filler cards to play in prompt-showdown."
	demoFillerPackDir             = "demo-filler-pack"
	systemFillerPackFRName        = "Pack de fillers de démo"
	systemFillerPackFRDescription = "Cartes de fillers prêtes à jouer dans Prompt Choice."
	demoFillerPackFRDir           = "demo-filler-pack-fr"
)

// textPackSpec holds the per-pack parameters consumed by the shared text-style
// sync orchestration. The sync logic itself is language- and version-agnostic
// — it reads items.json from `dir` and upserts the pack identified by `id`,
// writing items at the requested `payloadVersion`.
type textPackSpec struct {
	id             uuid.UUID
	name           string
	description    string
	language       string
	dir            string
	payloadVersion int32
}

var (
	textSpecEN = textPackSpec{
		id:             SystemTextPackID,
		name:           systemTextPackName,
		description:    systemTextPackDescription,
		language:       "en",
		dir:            demoTextPackDir,
		payloadVersion: 2,
	}
	textSpecFR = textPackSpec{
		id:             SystemTextPackFRID,
		name:           systemTextPackFRName,
		description:    systemTextPackFRDescription,
		language:       "fr",
		dir:            demoTextPackFRDir,
		payloadVersion: 2,
	}
	fillerSpecEN = textPackSpec{
		id:             SystemFillerPackID,
		name:           systemFillerPackName,
		description:    systemFillerPackDescription,
		language:       "en",
		dir:            demoFillerPackDir,
		payloadVersion: 3,
	}
	fillerSpecFR = textPackSpec{
		id:             SystemFillerPackFRID,
		name:           systemFillerPackFRName,
		description:    systemFillerPackFRDescription,
		language:       "fr",
		dir:            demoFillerPackFRDir,
		payloadVersion: 3,
	}
)

// promptPackSpec is the prompt (payload v4) counterpart of textPackSpec.
// Prompt items have a different payload shape ({prefix, suffix}), so they
// reuse the upsert/scan structure but not the create/apply helpers.
type promptPackSpec struct {
	id          uuid.UUID
	name        string
	description string
	language    string
	dir         string
}

var (
	promptSpecEN = promptPackSpec{
		id:          SystemPromptPackID,
		name:        systemPromptPackName,
		description: systemPromptPackDescription,
		language:    "en",
		dir:         demoPromptPackDir,
	}
	promptSpecFR = promptPackSpec{
		id:          SystemPromptPackFRID,
		name:        systemPromptPackFRName,
		description: systemPromptPackFRDescription,
		language:    "fr",
		dir:         demoPromptPackFRDir,
	}
)

var allowedExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// Sync runs the startup sync using the embedded demo-pack/ folder.
// Non-fatal at the call site: main.go logs errors and continues.
func Sync(ctx context.Context, q *db.Queries, store storage.Storage, logger *slog.Logger) error {
	return SyncFS(ctx, q, store, DemoPackFS, logger)
}

// SyncFS is Sync's testable core — it accepts any fs.FS rooted at the
// directory containing demo-pack/ (so fstest.MapFS with "demo-pack/foo.png"
// entries works the same as the embedded FS).
func SyncFS(ctx context.Context, q *db.Queries, store storage.Storage, srcFS fs.FS, logger *slog.Logger) error {
	start := time.Now()

	pack, err := q.UpsertSystemPack(ctx, db.UpsertSystemPackParams{
		ID:          SystemPackID,
		Name:        systemPackName,
		Description: strPtr(systemPackDescription),
		// Image content is language-agnostic — memes land in any locale, so
		// the bundled image pack is offered to hosts regardless of UI locale.
		Language: "multi",
	})
	if err != nil {
		return fmt.Errorf("upsert system pack: %w", err)
	}

	entries, err := fs.ReadDir(srcFS, demoPackDir)
	if err != nil {
		// Missing directory is a hard error when using the real embed.FS, but
		// fstest.MapFS tests that don't declare "demo-pack" would also hit
		// this path. Treat as "no files" so the upsert still wins.
		logger.Warn("systempack sync", "event", "demo_pack.read_failed", "error", err)
		entries = nil
	}

	existing, err := loadExisting(ctx, q, pack.ID)
	if err != nil {
		return fmt.Errorf("load existing items: %w", err)
	}

	seen := map[string]bool{}
	stats := syncStats{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip .gitkeep and dotfiles
		}
		ext := strings.ToLower(filepath.Ext(name))
		contentType, ok := allowedExts[ext]
		if !ok {
			logger.Warn("systempack sync", "event", "file.skipped_unsupported", "name", name)
			continue
		}

		data, err := fs.ReadFile(srcFS, demoPackDir+"/"+name)
		if err != nil {
			logger.Error("systempack sync", "event", "file.read_failed", "name", name, "error", err)
			continue
		}
		stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		seen[stem] = true

		hashHex := sha256Hex(data)

		if cur, exists := existing[stem]; exists {
			if cur.hash == hashHex {
				stats.unchanged++
				continue
			}
			if err := applyNewVersion(ctx, q, store, pack.ID, cur.itemID, name, contentType, data, hashHex); err != nil {
				logger.Error("systempack sync", "event", "item.update_failed", "name", stem, "error", err)
				continue
			}
			stats.updated++
			logger.Info("systempack sync", "event", "item.update", "name", stem, "new_hash", hashHex[:8])
			continue
		}

		if err := createNewItem(ctx, q, store, pack.ID, stem, name, contentType, data, hashHex); err != nil {
			logger.Error("systempack sync", "event", "item.create_failed", "name", stem, "error", err)
			continue
		}
		stats.created++
		logger.Info("systempack sync", "event", "item.create", "name", stem)
	}

	for stem, cur := range existing {
		if seen[stem] {
			continue
		}
		if err := q.SoftDeleteItem(ctx, cur.itemID); err != nil {
			logger.Error("systempack sync", "event", "item.retire_failed", "name", stem, "error", err)
			continue
		}
		stats.retired++
		logger.Info("systempack sync", "event", "item.retire", "name", stem)
	}

	logger.Info("systempack sync",
		"event", "summary",
		"created", stats.created,
		"updated", stats.updated,
		"retired", stats.retired,
		"unchanged", stats.unchanged,
		"duration", time.Since(start).String())

	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────

type syncStats struct {
	created, updated, retired, unchanged int
}

type existingItem struct {
	itemID uuid.UUID
	hash   string
}

func loadExisting(ctx context.Context, q *db.Queries, packID uuid.UUID) (map[string]existingItem, error) {
	rows, err := q.ListItemsForPack(ctx, db.ListItemsForPackParams{
		PackID: packID, Lim: 1000, Off: 0,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]existingItem{}
	for _, r := range rows {
		stem := strings.ToLower(r.Name)
		h := ""
		if len(r.Payload) > 0 {
			var p map[string]any
			if err := json.Unmarshal(r.Payload, &p); err == nil {
				if s, ok := p["sha256"].(string); ok {
					h = s
				}
			}
		}
		out[stem] = existingItem{itemID: r.ID, hash: h}
	}
	return out, nil
}

func createNewItem(
	ctx context.Context,
	q *db.Queries,
	store storage.Storage,
	packID uuid.UUID,
	stem, filename, contentType string,
	data []byte, hashHex string,
) error {
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID:         packID,
		Name:           stem,
		PayloadVersion: 1,
	})
	if err != nil {
		return fmt.Errorf("create item row: %w", err)
	}
	key := storage.ObjectKey(packID.String(), item.ID.String(), 1, filename)
	if err := store.Upload(ctx, key, bytes.NewReader(data), contentType, int64(len(data))); err != nil {
		// Roll back the orphan item row so the next sync retries cleanly.
		_ = q.SoftDeleteItem(ctx, item.ID)
		return fmt.Errorf("upload: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"sha256": hashHex})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   item.ID,
		MediaKey: strPtr(key),
		Payload:  payload,
	})
	if err != nil {
		return fmt.Errorf("create version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               item.ID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current version: %w", err)
	}
	return nil
}

func applyNewVersion(
	ctx context.Context,
	q *db.Queries,
	store storage.Storage,
	packID, itemID uuid.UUID,
	filename, contentType string,
	data []byte, hashHex string,
) error {
	versions, err := q.ListVersionsForItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("list versions: %w", err)
	}
	nextN := len(versions) + 1

	key := storage.ObjectKey(packID.String(), itemID.String(), nextN, filename)
	if err := store.Upload(ctx, key, bytes.NewReader(data), contentType, int64(len(data))); err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"sha256": hashHex})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   itemID,
		MediaKey: strPtr(key),
		Payload:  payload,
	})
	if err != nil {
		return fmt.Errorf("create version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               itemID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current version: %w", err)
	}
	return nil
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ── Text demo pack ───────────────────────────────────────────────────────

// SyncText runs the startup sync for the bundled English text pack using the
// embedded demo-text-pack/items.json. Symmetric to Sync but with no asset
// storage — text content lives in version.payload as `{text, sha256}`.
func SyncText(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return SyncTextFS(ctx, q, DemoTextPackFS, logger)
}

// SyncTextFR runs the startup sync for the bundled French text pack.
func SyncTextFR(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return syncTextPackFS(ctx, q, DemoTextPackFRFS, textSpecFR, logger)
}

// SyncFiller runs the startup sync for the bundled English filler pack
// (payload v3). Same orchestration as SyncText with a different spec.
func SyncFiller(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return syncTextPackFS(ctx, q, DemoFillerPackFS, fillerSpecEN, logger)
}

// SyncFillerFR runs the startup sync for the bundled French filler pack.
func SyncFillerFR(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return syncTextPackFS(ctx, q, DemoFillerPackFRFS, fillerSpecFR, logger)
}

// SyncTextFS is SyncText's testable core (English spec).
func SyncTextFS(ctx context.Context, q *db.Queries, srcFS fs.FS, logger *slog.Logger) error {
	return syncTextPackFS(ctx, q, srcFS, textSpecEN, logger)
}

// SyncTextFRFS is SyncTextFR's testable core (French spec).
func SyncTextFRFS(ctx context.Context, q *db.Queries, srcFS fs.FS, logger *slog.Logger) error {
	return syncTextPackFS(ctx, q, srcFS, textSpecFR, logger)
}

// syncTextPackFS is the language-agnostic orchestration that both the EN and
// FR entrypoints delegate to. The spec carries the only per-language state
// (pack id, display strings, CHECK value, embedded dir).
func syncTextPackFS(ctx context.Context, q *db.Queries, srcFS fs.FS, spec textPackSpec, logger *slog.Logger) error {
	start := time.Now()

	pack, err := q.UpsertSystemPack(ctx, db.UpsertSystemPackParams{
		ID:          spec.id,
		Name:        spec.name,
		Description: strPtr(spec.description),
		Language:    spec.language,
	})
	if err != nil {
		return fmt.Errorf("upsert system text pack: %w", err)
	}

	entries, err := readTextEntries(srcFS, spec.dir, logger)
	if err != nil {
		return fmt.Errorf("read text entries: %w", err)
	}

	existing, err := loadExisting(ctx, q, pack.ID)
	if err != nil {
		return fmt.Errorf("load existing text items: %w", err)
	}

	seen := map[string]bool{}
	stats := syncStats{}

	for _, e := range entries {
		stem := strings.ToLower(strings.TrimSpace(e.Name))
		if stem == "" {
			logger.Warn("systempack sync", "event", "text.entry_skipped_no_name")
			continue
		}
		seen[stem] = true
		hashHex := sha256Hex([]byte(e.Text))

		if cur, ok := existing[stem]; ok {
			if cur.hash == hashHex {
				stats.unchanged++
				continue
			}
			if err := applyNewTextVersion(ctx, q, cur.itemID, e.Text, hashHex); err != nil {
				logger.Error("systempack sync", "event", "text.update_failed", "name", stem, "error", err)
				continue
			}
			stats.updated++
			logger.Info("systempack sync", "event", "text.update", "name", stem, "new_hash", hashHex[:8])
			continue
		}

		if err := createNewTextItem(ctx, q, pack.ID, stem, e.Text, hashHex, spec.payloadVersion); err != nil {
			logger.Error("systempack sync", "event", "text.create_failed", "name", stem, "error", err)
			continue
		}
		stats.created++
		logger.Info("systempack sync", "event", "text.create", "name", stem)
	}

	for stem, cur := range existing {
		if seen[stem] {
			continue
		}
		if err := q.SoftDeleteItem(ctx, cur.itemID); err != nil {
			logger.Error("systempack sync", "event", "text.retire_failed", "name", stem, "error", err)
			continue
		}
		stats.retired++
		logger.Info("systempack sync", "event", "text.retire", "name", stem)
	}

	logger.Info("systempack sync",
		"event", "text_summary",
		"created", stats.created,
		"updated", stats.updated,
		"retired", stats.retired,
		"unchanged", stats.unchanged,
		"duration", time.Since(start).String())

	return nil
}

type textEntry struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// readTextEntries loads and parses items.json from the given dir. A missing
// file is treated as "no entries" so a fresh checkout without bundled content
// still upserts the pack row (consistent with how the image sync handles a
// missing folder).
func readTextEntries(srcFS fs.FS, dir string, logger *slog.Logger) ([]textEntry, error) {
	data, err := fs.ReadFile(srcFS, dir+"/"+demoTextPackFile)
	if err != nil {
		logger.Warn("systempack sync", "event", "text.read_failed", "error", err)
		return nil, nil
	}
	var entries []textEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse items.json: %w", err)
	}
	return entries, nil
}

func createNewTextItem(
	ctx context.Context,
	q *db.Queries,
	packID uuid.UUID,
	stem, text, hashHex string,
	payloadVersion int32,
) error {
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID:         packID,
		Name:           stem,
		PayloadVersion: payloadVersion,
	})
	if err != nil {
		return fmt.Errorf("create text item row: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{"text": text, "sha256": hashHex})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   item.ID,
		MediaKey: nil,
		Payload:  payload,
	})
	if err != nil {
		// Roll back the orphan item row so the next sync retries cleanly.
		_ = q.SoftDeleteItem(ctx, item.ID)
		return fmt.Errorf("create text version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               item.ID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current text version: %w", err)
	}
	return nil
}

func applyNewTextVersion(
	ctx context.Context,
	q *db.Queries,
	itemID uuid.UUID,
	text, hashHex string,
) error {
	payload, _ := json.Marshal(map[string]string{"text": text, "sha256": hashHex})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   itemID,
		MediaKey: nil,
		Payload:  payload,
	})
	if err != nil {
		return fmt.Errorf("create text version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               itemID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current text version: %w", err)
	}
	return nil
}

// ── Prompt demo pack ─────────────────────────────────────────────────────
//
// Prompt items have payload_version 4 and a `{prefix, suffix}` shape — at
// least one of the two is non-empty (the blank can sit at start, middle, or
// end of the rendered sentence). Same idempotent upsert + hash skip pattern
// as the text pack sync.

// SyncPrompt runs the startup sync for the bundled English prompt pack.
func SyncPrompt(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return syncPromptPackFS(ctx, q, DemoPromptPackFS, promptSpecEN, logger)
}

// SyncPromptFR runs the startup sync for the bundled French prompt pack.
func SyncPromptFR(ctx context.Context, q *db.Queries, logger *slog.Logger) error {
	return syncPromptPackFS(ctx, q, DemoPromptPackFRFS, promptSpecFR, logger)
}

// SyncPromptFS / SyncPromptFRFS expose the testable cores so unit tests can
// inject an fstest.MapFS without wiring through the embedded FS.
func SyncPromptFS(ctx context.Context, q *db.Queries, srcFS fs.FS, logger *slog.Logger) error {
	return syncPromptPackFS(ctx, q, srcFS, promptSpecEN, logger)
}
func SyncPromptFRFS(ctx context.Context, q *db.Queries, srcFS fs.FS, logger *slog.Logger) error {
	return syncPromptPackFS(ctx, q, srcFS, promptSpecFR, logger)
}

type promptEntry struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

func syncPromptPackFS(ctx context.Context, q *db.Queries, srcFS fs.FS, spec promptPackSpec, logger *slog.Logger) error {
	start := time.Now()

	pack, err := q.UpsertSystemPack(ctx, db.UpsertSystemPackParams{
		ID:          spec.id,
		Name:        spec.name,
		Description: strPtr(spec.description),
		Language:    spec.language,
	})
	if err != nil {
		return fmt.Errorf("upsert system prompt pack: %w", err)
	}

	entries, err := readPromptEntries(srcFS, spec.dir, logger)
	if err != nil {
		return fmt.Errorf("read prompt entries: %w", err)
	}

	existing, err := loadExisting(ctx, q, pack.ID)
	if err != nil {
		return fmt.Errorf("load existing prompt items: %w", err)
	}

	seen := map[string]bool{}
	stats := syncStats{}

	for _, e := range entries {
		stem := strings.ToLower(strings.TrimSpace(e.Name))
		if stem == "" {
			logger.Warn("systempack sync", "event", "prompt.entry_skipped_no_name")
			continue
		}
		if strings.TrimSpace(e.Prefix) == "" && strings.TrimSpace(e.Suffix) == "" {
			logger.Warn("systempack sync", "event", "prompt.entry_skipped_blank_only", "name", stem)
			continue
		}
		seen[stem] = true
		// Hash the canonical concatenation so a tweak to either side bumps the
		// hash (and a no-op edit doesn't).
		hashHex := sha256Hex([]byte(e.Prefix + "" + e.Suffix))

		if cur, ok := existing[stem]; ok {
			if cur.hash == hashHex {
				stats.unchanged++
				continue
			}
			if err := applyNewPromptVersion(ctx, q, cur.itemID, e.Prefix, e.Suffix, hashHex); err != nil {
				logger.Error("systempack sync", "event", "prompt.update_failed", "name", stem, "error", err)
				continue
			}
			stats.updated++
			logger.Info("systempack sync", "event", "prompt.update", "name", stem, "new_hash", hashHex[:8])
			continue
		}

		if err := createNewPromptItem(ctx, q, pack.ID, stem, e.Prefix, e.Suffix, hashHex); err != nil {
			logger.Error("systempack sync", "event", "prompt.create_failed", "name", stem, "error", err)
			continue
		}
		stats.created++
		logger.Info("systempack sync", "event", "prompt.create", "name", stem)
	}

	for stem, cur := range existing {
		if seen[stem] {
			continue
		}
		if err := q.SoftDeleteItem(ctx, cur.itemID); err != nil {
			logger.Error("systempack sync", "event", "prompt.retire_failed", "name", stem, "error", err)
			continue
		}
		stats.retired++
		logger.Info("systempack sync", "event", "prompt.retire", "name", stem)
	}

	logger.Info("systempack sync",
		"event", "prompt_summary",
		"created", stats.created,
		"updated", stats.updated,
		"retired", stats.retired,
		"unchanged", stats.unchanged,
		"duration", time.Since(start).String())

	return nil
}

func readPromptEntries(srcFS fs.FS, dir string, logger *slog.Logger) ([]promptEntry, error) {
	data, err := fs.ReadFile(srcFS, dir+"/"+demoTextPackFile)
	if err != nil {
		logger.Warn("systempack sync", "event", "prompt.read_failed", "error", err)
		return nil, nil
	}
	var entries []promptEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse items.json: %w", err)
	}
	return entries, nil
}

func createNewPromptItem(
	ctx context.Context,
	q *db.Queries,
	packID uuid.UUID,
	stem, prefix, suffix, hashHex string,
) error {
	item, err := q.CreateItem(ctx, db.CreateItemParams{
		PackID:         packID,
		Name:           stem,
		PayloadVersion: 4,
	})
	if err != nil {
		return fmt.Errorf("create prompt item row: %w", err)
	}
	payload, _ := json.Marshal(map[string]string{
		"prefix": prefix,
		"suffix": suffix,
		"sha256": hashHex,
	})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   item.ID,
		MediaKey: nil,
		Payload:  payload,
	})
	if err != nil {
		_ = q.SoftDeleteItem(ctx, item.ID)
		return fmt.Errorf("create prompt version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               item.ID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current prompt version: %w", err)
	}
	return nil
}

func applyNewPromptVersion(
	ctx context.Context,
	q *db.Queries,
	itemID uuid.UUID,
	prefix, suffix, hashHex string,
) error {
	payload, _ := json.Marshal(map[string]string{
		"prefix": prefix,
		"suffix": suffix,
		"sha256": hashHex,
	})
	ver, err := q.CreateItemVersion(ctx, db.CreateItemVersionParams{
		ItemID:   itemID,
		MediaKey: nil,
		Payload:  payload,
	})
	if err != nil {
		return fmt.Errorf("create prompt version: %w", err)
	}
	if _, err := q.SetCurrentVersion(ctx, db.SetCurrentVersionParams{
		ID:               itemID,
		CurrentVersionID: pgtype.UUID{Bytes: ver.ID, Valid: true},
	}); err != nil {
		return fmt.Errorf("set current prompt version: %w", err)
	}
	return nil
}
