package testutil

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
)

var sharedPool *pgxpool.Pool

// SetupSuite starts a postgres:17-alpine container, runs all migrations, and
// exposes a shared pool for the test package. Call as:
//
//	os.Exit(testutil.SetupSuite(m))
//
// from TestMain in any package that needs a database.
func SetupSuite(m *testing.M) int {
	ctx := context.Background()

	pgc, err := tcpostgres.Run(ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("testutil: start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pgc); err != nil {
			log.Printf("testutil: terminate container: %v", err)
		}
	}()

	connStr, err := pgc.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("testutil: connection string: %v", err)
	}

	if err := runMigrations(connStr); err != nil {
		log.Fatalf("testutil: migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("testutil: pool: %v", err)
	}
	sharedPool = pool
	defer pool.Close()

	return m.Run()
}

// Pool returns the shared *pgxpool.Pool for handler constructors.
// Use in tests that build HTTP handlers (e.g. auth.New(testutil.Pool(), ...)).
func Pool() *pgxpool.Pool {
	return sharedPool
}

// WithTx runs fn inside a transaction that is always rolled back in t.Cleanup.
// Use for direct SQLC query tests — each test gets a clean slate with no
// cleanup code required.
func WithTx(t *testing.T, fn func(q *db.Queries)) {
	t.Helper()
	ctx := context.Background()
	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("testutil.WithTx: begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(ctx); err != nil {
			// pgx returns an error if the tx was already committed; ignore it.
			_ = err
		}
	})
	fn(db.New(tx))
}

// SeedName derives a collision-safe lowercase string from t.Name().
// Use as the base for usernames, emails, and other unique seeds in handler
// tests where the pool is passed directly to the handler.
func SeedName(t *testing.T) string {
	t.Helper()
	r := strings.NewReplacer("/", "_", " ", "_", ":", "_", "-", "_")
	slug := strings.ToLower(r.Replace(t.Name()))
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return slug
}

// repoRoot walks up from this file's directory until it finds go.mod.
func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("testutil: runtime.Caller failed")
	}
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("testutil: go.mod not found")
		}
		dir = parent
	}
}

func runMigrations(connStr string) error {
	migrationsPath := "file://" + filepath.Join(repoRoot(), "db", "migrations")
	m, err := migrate.New(migrationsPath, connStr)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}
