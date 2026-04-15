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

// SystemPackID is the fixed sentinel UUID of the bundled demo pack.
// Stable across reboots so the RustFS object keys (packs/{packID}/...) are
// also stable, which in turn keeps round history pointing at valid blobs.
var SystemPackID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

const (
	systemPackName        = "Demo Pack"
	systemPackDescription = "Bundled sample images to get you started."
	demoPackDir           = "demo-pack"
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
