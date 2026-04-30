// backend/internal/systempack/orientation_backfill.go
package systempack

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

// BackfillOrientation enriches every existing image-version row whose payload
// is missing the `orientation` key. Detection happens server-side, so older
// versions uploaded before this field existed self-heal on the next boot.
//
// One pass on startup, sequential — the row count is bounded (one per
// non-deleted image item version) and the work is mostly I/O against
// RustFS. Failures on a single row are logged and skipped so a single
// missing or corrupt blob can't stall the rest of the backfill.
func BackfillOrientation(ctx context.Context, q *db.Queries, store storage.Storage, logger *slog.Logger) error {
	start := time.Now()

	rows, err := q.ListVersionsMissingOrientation(ctx)
	if err != nil {
		return fmt.Errorf("list versions missing orientation: %w", err)
	}
	if len(rows) == 0 {
		logger.Info("orientation backfill", "event", "skip_no_rows")
		return nil
	}

	updated, skipped := 0, 0
	for _, r := range rows {
		if r.MediaKey == nil || *r.MediaKey == "" {
			skipped++
			continue
		}
		body, _, _, err := store.Download(ctx, *r.MediaKey)
		if err != nil {
			logger.Warn("orientation backfill", "event", "download_failed",
				"version_id", r.ID, "media_key", *r.MediaKey, "error", err)
			skipped++
			continue
		}
		// Cap the read so a corrupt or oversized object can't OOM the
		// backfill. 8 MiB is well above the upload limit (default 2 MiB)
		// and large enough that the image header sits in the first chunk.
		data, err := io.ReadAll(io.LimitReader(body, 8*1024*1024))
		_ = body.Close()
		if err != nil {
			logger.Warn("orientation backfill", "event", "read_failed",
				"version_id", r.ID, "error", err)
			skipped++
			continue
		}
		orient, err := storage.DetectOrientation(data)
		if err != nil {
			logger.Warn("orientation backfill", "event", "detect_failed",
				"version_id", r.ID, "media_key", *r.MediaKey, "error", err)
			skipped++
			continue
		}
		if err := q.SetVersionOrientation(ctx, db.SetVersionOrientationParams{
			ID:          r.ID,
			Orientation: orient,
		}); err != nil {
			logger.Warn("orientation backfill", "event", "update_failed",
				"version_id", r.ID, "error", err)
			skipped++
			continue
		}
		updated++
	}

	logger.Info("orientation backfill",
		"event", "summary",
		"updated", updated,
		"skipped", skipped,
		"duration", time.Since(start).String())
	return nil
}
