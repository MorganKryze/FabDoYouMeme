// backend/internal/api/game_types.go
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
)

// GameTypeHTTPHandler handles /api/game-types/* routes.
type GameTypeHTTPHandler struct {
	db       *db.Queries
	registry *game.Registry
}

func NewGameTypeHandler(pool *pgxpool.Pool, registry *game.Registry) *GameTypeHTTPHandler {
	return &GameTypeHTTPHandler{db: db.New(pool), registry: registry}
}

// List handles GET /api/game-types.
func (h *GameTypeHTTPHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetSessionUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	types, err := h.db.ListGameTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list game types")
		return
	}
	writeJSON(w, http.StatusOK, types)
}

// GetBySlug handles GET /api/game-types/:slug.
func (h *GameTypeHTTPHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetSessionUser(r); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	slug := chi.URLParam(r, "slug")
	gt, err := h.db.GetGameTypeBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Game type not found")
		return
	}
	handler, ok := h.registry.Get(slug)
	var supportedVersions []int
	if ok {
		supportedVersions = handler.SupportedPayloadVersions()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                         gt.ID,
		"slug":                       gt.Slug,
		"name":                       gt.Name,
		"description":                gt.Description,
		"version":                    gt.Version,
		"supports_solo":              gt.SupportsSolo,
		"config":                     gt.Config,
		"supported_payload_versions": supportedVersions,
	})
}
