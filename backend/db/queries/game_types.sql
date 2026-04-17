-- backend/db/queries/game_types.sql

-- name: ListGameTypes :many
SELECT * FROM game_types ORDER BY name;

-- name: GetGameTypeBySlug :one
SELECT * FROM game_types WHERE slug = $1;

-- name: GetGameTypeByID :one
SELECT * FROM game_types WHERE id = $1;

-- name: UpsertGameType :one
-- Idempotent upsert used at startup to sync game_types rows from each
-- handler's manifest.yaml. Slug is the natural key; the row's UUID (id)
-- is preserved across upserts so existing rooms.game_type_id references
-- remain valid.
INSERT INTO game_types (slug, name, description, version, supports_solo, config)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (slug) DO UPDATE SET
  name          = EXCLUDED.name,
  description   = EXCLUDED.description,
  version       = EXCLUDED.version,
  supports_solo = EXCLUDED.supports_solo,
  config        = EXCLUDED.config
RETURNING *;
