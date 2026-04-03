//go:build integration

package testutil

import (
	"context"
	"os"
	"testing"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewDB opens a test DB connection. Skips if DATABASE_URL is not set.
func NewDB(t *testing.T) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("testutil.NewDB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool, db.New(pool)
}
