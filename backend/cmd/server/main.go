// backend/cmd/server/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	migrations "github.com/MorganKryze/FabDoYouMeme/backend/db/migrations"
	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/clock"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/config"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/email"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/game"
	memecaption "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_caption"
	mw "github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// ── Database ─────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── Migrations ───────────────────────────────────────────────────────────
	if err := runMigrations(cfg.DatabaseURL, logger); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	queries := db.New(pool)

	// ── Startup cleanup (idempotent) ─────────────────────────────────────────
	if result, err := queries.FinishCrashedRooms(context.Background()); err != nil {
		logger.Error("startup: finish crashed rooms", "error", err)
	} else {
		logger.Info("startup cleanup", "event", "room.crash_recovery",
			"count", result.RowsAffected())
	}
	if result, err := queries.FinishAbandonedLobbies(context.Background()); err != nil {
		logger.Error("startup: finish abandoned lobbies", "error", err)
	} else {
		logger.Info("startup cleanup", "event", "room.abandoned",
			"count", result.RowsAffected())
	}

	// ── Email ────────────────────────────────────────────────────────────────
	emailSvc, err := email.NewService(cfg)
	if err != nil {
		logger.Error("email service init failed", "error", err)
		os.Exit(1)
	}

	// ── Clock (real wall clock in production) ────────────────────────────────
	clk := clock.Real{}

	// ── Auth handler ─────────────────────────────────────────────────────────
	authHandler := auth.New(pool, cfg, emailSvc, logger, clk)

	// ── First-boot admin bootstrap ────────────────────────────────────────────
	if err := authHandler.SeedAdmin(context.Background()); err != nil {
		logger.Error("admin bootstrap failed", "error", err)
		// Non-fatal — server continues; admin can be created manually
	}

	// ── Storage ──────────────────────────────────────────────────────────────
	store, err := storage.NewS3(cfg.RustFSEndpoint, cfg.RustFSAccessKey, cfg.RustFSSecretKey, cfg.RustFSBucket)
	if err != nil {
		logger.Error("storage init failed", "error", err)
		os.Exit(1)
	}

	// ── Game registry ─────────────────────────────────────────────────────────
	registry := game.NewRegistry()
	registry.Register(memecaption.New())

	// ── Game manager ──────────────────────────────────────────────────────────
	// context.Background() is the correct parent: hubs outlive individual
	// requests and are only killed when manager.Shutdown() cancels the
	// derived server-scoped context in the signal handler below.
	manager := game.NewManager(context.Background(), registry, queries, cfg, logger, clk)

	// ── Rate limiters ─────────────────────────────────────────────────────────
	// All limiters share cfg.TrustedProxies so the per-IP bucket key is the
	// real client (via ClientIP) rather than the reverse proxy's address.
	authLimiter   := mw.NewRateLimiter(cfg.RateLimitAuthRPM, 60, clk, cfg.TrustedProxies)
	inviteLimiter := mw.NewRateLimiter(cfg.RateLimitInviteRPH, 3600, clk, cfg.TrustedProxies)
	globalLimiter := mw.NewRateLimiter(cfg.RateLimitGlobalRPM, 60, clk, cfg.TrustedProxies)
	roomLimiter   := mw.NewRateLimiter(cfg.RateLimitRoomsRPH, 3600, clk, cfg.TrustedProxies)
	uploadLimiter := mw.NewRateLimiter(cfg.RateLimitUploadsRPH, 3600, clk, cfg.TrustedProxies)

	// ── HTTP handlers ─────────────────────────────────────────────────────────
	packHandler      := api.NewPackHandler(pool, cfg, store)
	roomHandler      := api.NewRoomHandler(pool, cfg, manager)
	assetHandler     := api.NewAssetHandler(pool, cfg, store)
	gameTypeHandler  := api.NewGameTypeHandler(pool, registry)
	adminHTTPHandler := api.NewAdminHandler(pool)
	wsHandler        := api.NewWSHandler(manager, cfg.AllowedOrigin)
	healthHandler    := api.NewHealthHandler(pool, store)

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(mw.RequestID)
	r.Use(mw.Logger(logger))
	r.Use(mw.Session(authHandler.SessionLookupFn, logger))
	r.Use(mw.Metrics)
	r.Use(globalLimiter.Middleware)

	// Health (no auth)
	r.Get("/api/health", healthHandler.Liveness)
	r.Get("/api/health/deep", healthHandler.Readiness)

	// /api/metrics — restricted to private IP ranges (loopback + RFC-1918).
	// Never expose this endpoint to the public internet. RequirePrivateIP
	// uses ClientIP under the hood so the gate works behind a reverse proxy
	// (where r.RemoteAddr would otherwise always look private and let
	// everything through).
	r.With(mw.RequirePrivateIP(cfg.TrustedProxies)).Handle("/api/metrics", promhttp.Handler())

	// Auth routes (rate-limited)
	r.With(authLimiter.Middleware).Route("/api/auth", func(r chi.Router) {
		r.With(inviteLimiter.Middleware).Post("/register", authHandler.Register)
		r.Post("/magic-link", authHandler.MagicLink)
		r.Post("/verify", authHandler.Verify)
		r.With(mw.RequireAuth).Post("/logout", authHandler.Logout)
		r.With(mw.RequireAuth).Get("/me", authHandler.Me)
	})

	// User profile
	r.With(mw.RequireAuth).Route("/api/users/me", func(r chi.Router) {
		r.Patch("/", authHandler.PatchMe)
		r.Get("/history", authHandler.GetHistory)
		r.Get("/export", authHandler.GetExport)
	})

	// Admin user management (delete is on auth handler for the 5-step txn)
	r.With(mw.RequireAuth, mw.RequireAdmin).Route("/api/admin", func(r chi.Router) {
		r.Get("/users", adminHTTPHandler.ListUsers)
		r.Patch("/users/{id}", adminHTTPHandler.UpdateUser)
		r.Delete("/users/{id}", authHandler.DeleteUser)
		r.Get("/invites", adminHTTPHandler.ListInvites)
		r.Post("/invites", adminHTTPHandler.CreateInvite)
		r.Delete("/invites/{id}", adminHTTPHandler.DeleteInvite)
		r.Get("/notifications", adminHTTPHandler.ListNotifications)
		r.Patch("/notifications/{id}", adminHTTPHandler.MarkNotificationRead)
	})

	// Game types
	r.With(mw.RequireAuth).Route("/api/game-types", func(r chi.Router) {
		r.Get("/", gameTypeHandler.List)
		r.Get("/{slug}", gameTypeHandler.GetBySlug)
	})

	// Packs + items
	r.With(mw.RequireAuth).Route("/api/packs", func(r chi.Router) {
		r.Get("/", packHandler.List)
		r.Post("/", packHandler.Create)
		r.Get("/{id}", packHandler.GetByID)
		r.Patch("/{id}", packHandler.Update)
		r.Delete("/{id}", packHandler.Delete)
		r.With(mw.RequireAdmin).Patch("/{id}/status", packHandler.SetStatus)

		// Items
		r.Get("/{id}/items", packHandler.ListItems)
		r.Post("/{id}/items", packHandler.CreateItem)
		r.Patch("/{id}/items/reorder", packHandler.ReorderItems)
		r.Patch("/{id}/items/{item_id}", packHandler.UpdateItem)
		r.Delete("/{id}/items/{item_id}", packHandler.DeleteItem)

		// Versions
		r.Get("/{id}/items/{item_id}/versions", packHandler.ListVersions)
		r.Post("/{id}/items/{item_id}/versions", packHandler.CreateVersion)
		r.Post("/{id}/items/{item_id}/versions/{vid}/restore", packHandler.RestoreVersion)
		r.Delete("/{id}/items/{item_id}/versions/{vid}", packHandler.SoftDeleteVersion)
		r.Delete("/{id}/items/{item_id}/versions/{vid}/purge", packHandler.PurgeVersion)
	})

	// Assets
	r.With(mw.RequireAuth).Route("/api/assets", func(r chi.Router) {
		r.With(uploadLimiter.Middleware).Post("/upload-url", assetHandler.UploadURL)
		r.Post("/download-url", assetHandler.DownloadURL)
	})

	// Rooms
	r.With(mw.RequireAuth).Route("/api/rooms", func(r chi.Router) {
		r.With(roomLimiter.Middleware).Post("/", roomHandler.Create)
		r.Get("/{code}", roomHandler.GetByCode)
		r.Patch("/{code}/config", roomHandler.UpdateConfig)
		r.Post("/{code}/leave", roomHandler.Leave)
		r.Post("/{code}/kick", roomHandler.Kick)
		r.Get("/{code}/leaderboard", roomHandler.Leaderboard)
	})

	// WebSocket
	r.With(mw.RequireAuth).Get("/api/ws/rooms/{code}", wsHandler.ServeHTTP)

	// ── Server lifecycle ──────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down")
	manager.Shutdown() // Notify WS clients before closing listeners
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	logger.Info("server stopped")
}

func runMigrations(databaseURL string, logger *slog.Logger) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	// golang-migrate's pgx/v5 driver uses "pgx5://" scheme
	migrateURL := strings.Replace(databaseURL, "postgres://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	v, _, _ := m.Version()
	logger.Info("migrations up to date", "version", v)
	return nil
}
