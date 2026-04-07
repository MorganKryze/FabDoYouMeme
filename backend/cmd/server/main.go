// backend/cmd/server/main.go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/MorganKryze/FabDoYouMeme/backend/db/sqlc"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/api"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/auth"
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
	queries := db.New(pool)

	// ── Startup cleanup (idempotent) ─────────────────────────────────────────
	if err := queries.FinishCrashedRooms(context.Background()); err != nil {
		logger.Warn("startup: finish crashed rooms", "error", err)
	}
	if err := queries.FinishAbandonedLobbies(context.Background()); err != nil {
		logger.Warn("startup: finish abandoned lobbies", "error", err)
	}

	// ── Email ────────────────────────────────────────────────────────────────
	emailSvc, err := email.NewService(cfg)
	if err != nil {
		logger.Error("email service init failed", "error", err)
		os.Exit(1)
	}

	// ── Auth handler ─────────────────────────────────────────────────────────
	authHandler := auth.New(pool, cfg, emailSvc, logger)

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
	manager := game.NewManager(registry, queries, cfg, logger)

	// ── Rate limiters ─────────────────────────────────────────────────────────
	authLimiter   := mw.NewRateLimiter(cfg.RateLimitAuthRPM, 60)
	inviteLimiter := mw.NewRateLimiter(cfg.RateLimitInviteRPH, 3600)
	globalLimiter := mw.NewRateLimiter(cfg.RateLimitGlobalRPM, 60)

	// ── HTTP handlers ─────────────────────────────────────────────────────────
	packHandler      := api.NewPackHandler(pool, cfg, store)
	roomHandler      := api.NewRoomHandler(pool, cfg, manager)
	assetHandler     := api.NewAssetHandler(pool, cfg, store)
	gameTypeHandler  := api.NewGameTypeHandler(pool, registry)
	adminHTTPHandler := api.NewAdminHandler(pool)
	wsHandler        := api.NewWSHandler(manager)
	healthHandler    := api.NewHealthHandler(pool)

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(mw.RequestID)
	r.Use(mw.Logger(logger))
	r.Use(mw.Session(authHandler.SessionLookupFn, logger))
	r.Use(globalLimiter.Middleware)

	// Health (no auth)
	r.Get("/api/health", healthHandler.Liveness)
	r.Get("/api/health/deep", healthHandler.Readiness)

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
	})

	// Assets
	r.With(mw.RequireAuth).Route("/api/assets", func(r chi.Router) {
		r.Post("/upload-url", assetHandler.UploadURL)
		r.Post("/download-url", assetHandler.DownloadURL)
	})

	// Rooms
	r.With(mw.RequireAuth).Route("/api/rooms", func(r chi.Router) {
		r.Post("/", roomHandler.Create)
		r.Get("/{code}", roomHandler.GetByCode)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
	}
	logger.Info("server stopped")
}
