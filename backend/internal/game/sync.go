// backend/internal/game/sync.go
package game

import (
	"context"
	"fmt"
	"log/slog"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

// SyncGameTypes upserts the `game_types` row for every registered handler
// from that handler's manifest.yaml. Slug is the natural key; the row's
// UUID is preserved across upserts (see the UpsertGameType query), so
// rooms.game_type_id foreign keys stay valid across restarts.
//
// This is the authoritative write path for game_types metadata at
// runtime. Migration 002 still seeds meme-freestyle on a fresh DB so the
// initial UUID is stable, but every subsequent field (name, description,
// version, supports_solo, config bounds) is reconciled from the
// in-binary manifests on each boot. A malformed manifest has already
// failed handler init before we reach this function.
func SyncGameTypes(ctx context.Context, q *db.Queries, reg *Registry, logger *slog.Logger) error {
	for _, h := range reg.All() {
		m := h.Manifest()
		if m == nil {
			return fmt.Errorf("handler %q: manifest is nil (handler init bug)", h.Slug())
		}
		cfgJSON, err := m.ConfigJSON()
		if err != nil {
			return fmt.Errorf("handler %q: marshal config: %w", h.Slug(), err)
		}
		var desc *string
		if m.Description != "" {
			d := m.Description
			desc = &d
		}
		if _, err := q.UpsertGameType(ctx, db.UpsertGameTypeParams{
			Slug:         m.Slug,
			Name:         m.Name,
			Description:  desc,
			Version:      m.Version,
			SupportsSolo: m.SupportsSolo,
			Config:       cfgJSON,
		}); err != nil {
			return fmt.Errorf("upsert game type %q: %w", m.Slug, err)
		}
		logger.Info("game type synced", "slug", m.Slug, "version", m.Version)
	}
	return nil
}
