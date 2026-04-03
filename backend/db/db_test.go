//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

func testDB(t *testing.T) *db.Queries {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return db.New(pool)
}

func TestSentinelRowExists(t *testing.T) {
	q := testDB(t)
	u, err := q.GetSentinelUser(context.Background())
	if err != nil {
		t.Fatalf("sentinel not found: %v", err)
	}
	if u.Username != "[deleted]" {
		t.Errorf("expected [deleted], got %s", u.Username)
	}
	if u.IsActive {
		t.Error("sentinel must be inactive")
	}
}

func TestMemeCaptionGameTypeSeeded(t *testing.T) {
	q := testDB(t)
	gt, err := q.GetGameTypeBySlug(context.Background(), "meme-caption")
	if err != nil {
		t.Fatalf("game type not found: %v", err)
	}
	if gt.Slug != "meme-caption" {
		t.Errorf("unexpected slug: %s", gt.Slug)
	}
}
