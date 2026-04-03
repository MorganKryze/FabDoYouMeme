-- backend/db/queries/game_types.sql

-- name: ListGameTypes :many
SELECT * FROM game_types ORDER BY name;

-- name: GetGameTypeBySlug :one
SELECT * FROM game_types WHERE slug = $1;

-- name: GetGameTypeByID :one
SELECT * FROM game_types WHERE id = $1;
