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
	memefreestyle "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_freestyle"
	memeshowdown "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/meme_showdown"
	promptfreestyle "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/prompt_freestyle"
	promptshowdown "github.com/MorganKryze/FabDoYouMeme/backend/internal/game/types/prompt_showdown"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/groupjobs"
	mw "github.com/MorganKryze/FabDoYouMeme/backend/internal/middleware"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/storage"
	"github.com/MorganKryze/FabDoYouMeme/backend/internal/systempack"
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
	// Defence-in-depth for the bulk-upload pipeline: sweep items whose
	// version chain never completed (created over an hour ago, no
	// current_version_id, not soft-deleted). The bulk endpoint now wraps
	// create+upload+version+promote in a single transaction so new orphans
	// are impossible, but historical rows from the pre-bulk per-image flow
	// would otherwise render forever as broken thumbnails. Idempotent — once
	// every row is healed the query is a zero-cost no-op.
	if result, err := queries.SweepOrphanItems(context.Background()); err != nil {
		logger.Error("startup: sweep orphan items", "error", err)
	} else {
		logger.Info("startup cleanup", "event", "items.orphan_sweep",
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

	// ── System pack sync (idempotent, non-fatal) ─────────────────────────────
	if err := systempack.Sync(context.Background(), queries, store, logger); err != nil {
		logger.Error("startup: system pack sync", "error", err)
		// Non-fatal — a RustFS blip or migration gap shouldn't stop the server.
	}
	if err := systempack.SyncText(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system text pack sync", "error", err)
		// Non-fatal — text sync touches no external storage, but a corrupt
		// items.json shouldn't block boot.
	}
	if err := systempack.SyncTextFR(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system text pack sync (fr)", "error", err)
	}
	if err := systempack.SyncFiller(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system filler pack sync", "error", err)
	}
	if err := systempack.SyncFillerFR(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system filler pack sync (fr)", "error", err)
	}
	if err := systempack.SyncPrompt(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system prompt pack sync", "error", err)
	}
	if err := systempack.SyncPromptFR(context.Background(), queries, logger); err != nil {
		logger.Error("startup: system prompt pack sync (fr)", "error", err)
	}

	// Backfill the orientation key on any image-version row uploaded before
	// detection landed. Self-healing on every boot — once every row carries
	// the field, the query short-circuits with zero rows and the pass is a
	// no-op.
	if err := systempack.BackfillOrientation(context.Background(), queries, store, logger); err != nil {
		logger.Error("startup: orientation backfill", "error", err)
	}

	// ── Game registry ─────────────────────────────────────────────────────────
	// Handlers carry their own manifest (see each handler's manifest.yaml).
	// SyncGameTypes reconciles the game_types DB rows from those manifests
	// on every boot, so changing a bound means editing one YAML and
	// restarting — no migration needed.
	registry := game.NewRegistry()
	registry.Register(memefreestyle.New())
	registry.Register(memeshowdown.New())
	registry.Register(promptfreestyle.New())
	registry.Register(promptshowdown.New())

	if err := game.SyncGameTypes(context.Background(), queries, registry, logger); err != nil {
		logger.Error("game type sync failed", "error", err)
		os.Exit(1)
	}

	// ── Game manager ──────────────────────────────────────────────────────────
	// context.Background() is the correct parent: hubs outlive individual
	// requests and are only killed when manager.Shutdown() cancels the
	// derived server-scoped context in the signal handler below.
	manager := game.NewManager(context.Background(), registry, queries, cfg, logger, clk)

	// ── Rate limiters ─────────────────────────────────────────────────────────
	// All limiters share cfg.TrustedProxies so the per-IP bucket key is the
	// real client (via ClientIP) rather than the reverse proxy's address.
	authLimiter := mw.NewRateLimiter(cfg.RateLimitAuthRPM, 60, clk, cfg.TrustedProxies)
	inviteLimiter := mw.NewRateLimiter(cfg.RateLimitInviteRPH, 3600, clk, cfg.TrustedProxies)
	globalLimiter := mw.NewRateLimiter(cfg.RateLimitGlobalRPM, 60, clk, cfg.TrustedProxies)
	roomLimiter := mw.NewRateLimiter(cfg.RateLimitRoomsRPH, 3600, clk, cfg.TrustedProxies)
	uploadLimiter := mw.NewRateLimiter(cfg.RateLimitUploadsRPH, 3600, clk, cfg.TrustedProxies)
	// Per-user bucket on GDPR export: 5/hour/user is ample for a humane
	// use case (Art. 20 data portability) while preventing a single
	// logged-in account from scraping the endpoint (finding 5.H).
	exportLimiter := mw.NewRateLimiter(5, 3600, clk, cfg.TrustedProxies)

	// ── HTTP handlers ─────────────────────────────────────────────────────────
	packHandler := api.NewPackHandler(pool, cfg, store, registry)
	roomHandler := api.NewRoomHandler(pool, cfg, manager, logger)
	assetHandler := api.NewAssetHandler(pool, cfg, store)
	gameTypeHandler := api.NewGameTypeHandler(pool, registry)
	adminHTTPHandler := api.NewAdminHandler(pool, store, logger)
	wsHandler := api.NewWSHandler(manager, queries, cfg.AllowedOrigins)
	healthHandler := api.NewHealthHandler(pool, store, emailSvc.Probe)
	groupHandler := api.NewGroupHandler(pool, cfg)
	groupMemberHandler := api.NewGroupMemberHandler(pool, cfg)
	groupInviteHandler := api.NewGroupInviteHandler(pool, cfg)
	groupPackHandler := api.NewGroupPackHandler(pool, cfg)
	groupNotificationHandler := api.NewGroupNotificationHandler(pool, cfg)
	adminQuotaHandler := api.NewAdminQuotaHandler(pool, cfg)
	adminGroupsHandler := api.NewAdminGroupsHandler(pool)

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(mw.RequestID)
	r.Use(mw.Logger(logger))
	r.Use(mw.Session(authHandler.SessionLookupFn, logger))
	r.Use(mw.Metrics)

	// /api/assets/media is mounted at the top-level mux without the global
	// per-user limiter. Reason: every studio / room page fans out 1 GET per
	// visible item (80+ thumbnails on a fully-loaded pack), all in parallel
	// from the browser, against a default budget of 100/min. The render
	// quickly exhausts the burst and the leftover thumbnails 429, leaving
	// broken-image rows scattered through the grid — exactly the "some
	// thumbnails work, some don't" symptom that triggered this whole
	// investigation. The endpoint is cheap (one authz query + one S3 GET),
	// idempotent, browser-cached for 1h via Cache-Control, and already runs
	// its own per-fetch authorization (CanUserDownloadMedia /
	// CanGuestDownloadMedia inside the handler), so a per-user request cap
	// adds no defensive value here.
	//
	// Mounted via With(...) (an inline Group of one) rather than after a
	// later r.Use(globalLimiter), because chi panics if Use is called once
	// any route is registered on the same mux. Everything that *should* be
	// rate-limited lives inside the limitedRoutes Group below.
	r.Get("/api/assets/media", assetHandler.GetMedia)

	// Group for everything else — the global per-user rate limiter applies
	// inside this group only. PerUserMiddleware (not Middleware) so
	// authenticated traffic is keyed by user ID. Behind the prod
	// reverse-proxy → SvelteKit → backend topology, every SSR-side fetch
	// (proxyToBackend, hydrateSession, apiFetch) originates from the
	// SvelteKit container's docker IP — so IP keying collapses every
	// logged-in user into one shared bucket. Anonymous traffic falls back
	// to the IP bucket (documented behaviour of PerUserMiddleware) so the
	// gate can never be silently bypassed.
	r.Group(func(r chi.Router) {
		r.Use(globalLimiter.PerUserMiddleware)

		// Health (no auth)
		r.Get("/api/health", healthHandler.Liveness)
		r.Get("/api/health/deep", healthHandler.Readiness)

	// /api/metrics — restricted to private IP ranges (loopback + RFC-1918).
	// Never expose this endpoint to the public internet. RequirePrivateIP
	// uses ClientIP under the hood so the gate works behind a reverse proxy
	// (where r.RemoteAddr would otherwise always look private and let
	// everything through).
	r.With(mw.RequirePrivateIP(cfg.TrustedProxies)).Handle("/api/metrics", promhttp.Handler())

	// Auth routes.
	// /api/auth/me is deliberately *outside* the strict auth limiter —
	// every authenticated SvelteKit page hit rehydrates the session via
	// this endpoint, so capping it at RATE_LIMIT_AUTH_RPM (default 10/min)
	// would soft-log-out users on a few quick refreshes. It still inherits
	// the global limiter set at the root.
	r.Route("/api/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Middleware)
			r.With(inviteLimiter.Middleware).Post("/register", authHandler.Register)
			r.Post("/magic-link", authHandler.MagicLink)
			r.Post("/verify", authHandler.Verify)
			r.With(mw.RequireAuth).Post("/logout", authHandler.Logout)
		})
		r.With(mw.RequireAuth).Get("/me", authHandler.Me)
	})

	// User profile
	r.With(mw.RequireAuth).Route("/api/users/me", func(r chi.Router) {
		r.Patch("/", authHandler.PatchMe)
		r.Get("/history", authHandler.GetHistory)
		r.Get("/active-room", authHandler.GetActiveRoom)
		r.With(exportLimiter.PerUserMiddleware).Get("/export", authHandler.GetExport)
	})

	// Admin user management (delete is on auth handler for the 5-step txn)
	r.With(mw.RequireAuth, mw.RequireAdmin).Route("/api/admin", func(r chi.Router) {
		r.Get("/users", adminHTTPHandler.ListUsers)
		r.Patch("/users/{id}", adminHTTPHandler.UpdateUser)
		r.Delete("/users/{id}", authHandler.DeleteUser)
		r.Post("/users/{id}/magic-link", authHandler.SendMagicLink)
		r.Get("/invites", adminHTTPHandler.ListInvites)
		r.Post("/invites", adminHTTPHandler.CreateInvite)
		r.Delete("/invites/{id}", adminHTTPHandler.DeleteInvite)
		r.Get("/notifications", adminHTTPHandler.ListNotifications)
		r.Patch("/notifications/{id}", adminHTTPHandler.MarkNotificationRead)
		r.Get("/stats", adminHTTPHandler.GetStats)
		r.Get("/storage", adminHTTPHandler.GetStorageStats)
		r.Get("/audit", adminHTTPHandler.ListAudit)

		// Phase-1 groups: per-user platform+group invite allocation. Each
		// handler short-circuits to 404 when FEATURE_GROUPS is off, so
		// mounting unconditionally keeps the route table stable across
		// runtime flips of the flag.
		r.Get("/user-invite-quotas", adminQuotaHandler.List)
		r.Put("/user-invite-quotas/{userID}", adminQuotaHandler.Set)

		// Phase 5 — platform-wide group overview + per-group overrides.
		r.Get("/groups", adminGroupsHandler.List)
		r.Get("/groups/{gid}", adminGroupsHandler.Get)
		r.Patch("/groups/{gid}/quota", adminGroupsHandler.SetQuota)
		r.Patch("/groups/{gid}/member_cap", adminGroupsHandler.SetMemberCap)
	})

	// Destructive admin actions ("danger zone"). Mounted ONLY when
	// the service is running in dev or preprod — in prod the routes
	// literally do not exist (404, not 403), so a leaked admin token
	// cannot hit them. Prod resets go through `make prod-clean`.
	if cfg.AppEnv != "prod" {
		dangerHandler := api.NewDangerHandler(pool, store)
		r.With(mw.RequireAuth, mw.RequireAdmin).Route("/api/admin/danger", func(r chi.Router) {
			r.Post("/wipe-game-history", dangerHandler.WipeGameHistory)
			r.Post("/wipe-packs-and-media", dangerHandler.WipePacksAndMedia)
			r.Post("/wipe-invites", dangerHandler.WipeInvites)
			r.Post("/wipe-sessions", dangerHandler.WipeSessions)
			r.Post("/full-reset", dangerHandler.FullReset)
		})
	}

	// Game types — GET /{slug} is pre-auth so the /rooms/{code}?as=guest
	// layout load can resolve the game type name for guests who arrived via
	// a shared link and have no session cookie. List stays auth-gated because
	// it's used by the in-app studio/admin flows only.
	r.Route("/api/game-types", func(r chi.Router) {
		r.With(mw.RequireAuth).Get("/", gameTypeHandler.List)
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
		// Bulk image / text import — folds what used to be 3-4 round-trips
		// per item into one transactional request. See items_bulk.go and
		// items_bulk_text.go.
		//
		// Intentionally NOT layered with uploadLimiter: the studio chunks
		// large image batches at one file per request to fit through tight
		// upstream ingress body caps without operator config (see
		// BULK_UPLOAD_CHUNK_SIZE in frontend/src/lib/api/studio.ts), so an
		// 83-image import would burn 83 upload-rate tokens against the
		// default 50/h burst and fail the second half. The root globalLimiter
		// (100/min burst per user) is the only gate; the per-user-per-minute
		// budget covers a typical bulk import even with the page's ambient
		// API chatter.
		r.Post("/{id}/items/bulk", packHandler.BulkCreateImageItems)
		r.Post("/{id}/items/bulk-text", packHandler.BulkCreateTextItems)
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
	//
	// GET /media is mounted at the router root above (before globalLimiter)
	// because the studio fans out one fetch per thumbnail and a fully-
	// loaded pack would otherwise exhaust the per-user burst. Authz lives
	// in the handler itself (CanUserDownloadMedia / CanGuestDownloadMedia)
	// so there is no security regression from the limiter bypass. The
	// remaining /api/assets/* routes are auth-gated mutations (uploads,
	// presigned URLs) and stay inside the limited group below.
	r.Route("/api/assets", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAuth)
			// Per-user keying: SSR-side asset uploads originate from the
			// SvelteKit container, so IP keying would pool every uploader
			// into one bucket (see globalLimiter comment above).
			r.With(uploadLimiter.PerUserMiddleware).Post("/upload-url", assetHandler.UploadURL)
			r.With(uploadLimiter.PerUserMiddleware).Post("/upload", assetHandler.UploadDirect)
			r.Post("/download-url", assetHandler.DownloadURL)
		})
	})

	// Rooms
	r.Route("/api/rooms", func(r chi.Router) {
		r.With(mw.RequireAuth, roomLimiter.PerUserMiddleware).Post("/", roomHandler.Create)

		// Pre-auth reads: the layout load for /rooms/{code}?as=guest runs
		// server-side during SSR and has no guest token to forward (the token
		// lives in the client's localStorage, set by the guest-join response).
		// Opening GetByCode mirrors the guest-join endpoint below — both expose
		// only public room metadata (code, state, game_type_slug, config), and
		// existence is already inferable from a successful guest-join. The WS
		// handshake remains the real identity gate (see ws.go:resolveIdentity).
		r.Get("/{code}", roomHandler.GetByCode)

		// Pre-auth guest join — visitors without an account can join a room
		// via a shared code. Keyed per *room code* (not IP) so a single
		// shared SvelteKit container IP, or a misconfigured TRUSTED_PROXIES,
		// cannot ceiling the whole platform's guest onboarding. The cap
		// still defends against enumeration of any one code at 10/hour.
		// The WS handshake verifies the returned guest token against the
		// room, so a successful call here does not yet grant access — it
		// must be paired with a matching WS upgrade.
		r.With(roomLimiter.PerKeyMiddleware(func(r *http.Request) string {
			if code := chi.URLParam(r, "code"); code != "" {
				return "room:" + code
			}
			return "" // empty → middleware falls back to IP bucket
		})).Post("/{code}/guest-join", roomHandler.GuestJoin)

		r.Group(func(r chi.Router) {
			r.Use(mw.RequireAuth)
			r.Patch("/{code}/config", roomHandler.UpdateConfig)
			r.Post("/{code}/leave", roomHandler.Leave)
			r.Post("/{code}/kick", roomHandler.Kick)
			r.Post("/{code}/end", roomHandler.End)
			r.Get("/{code}/leaderboard", roomHandler.Leaderboard)
			r.Get("/{code}/replay", roomHandler.GetReplay)
		})
	})

	// Groups (phase 1). Routes are mounted unconditionally; each handler
	// returns 404 when FEATURE_GROUPS is off so toggling the flag at
	// runtime works without a router rebuild.
	r.With(mw.RequireAuth).Route("/api/groups", func(r chi.Router) {
		r.Get("/", groupHandler.List)
		r.Post("/", groupHandler.Create)
		r.Get("/{id}", groupHandler.Get)
		r.Patch("/{id}", groupHandler.Update)
		r.Delete("/{id}", groupHandler.Delete)
		r.Post("/{id}/restore", groupHandler.Restore)

		r.Get("/{id}/members", groupMemberHandler.List)
		r.Delete("/{id}/members/{userID}", groupMemberHandler.Kick)
		r.Post("/{id}/members/{userID}/promote", groupMemberHandler.Promote)
		r.Post("/{id}/members/self/demote", groupMemberHandler.SelfDemote)
		r.Delete("/{id}/members/self", groupMemberHandler.Leave)

		r.Post("/{id}/bans", groupMemberHandler.Ban)
		r.Delete("/{id}/bans/{userID}", groupMemberHandler.Unban)
		r.Get("/{id}/bans", groupMemberHandler.ListBans)

		// Phase 2 — invites. Mint paths share the per-(admin, group) rate
		// limits inside the handler. Redemption of group_join codes lives
		// at /api/groups/invites/redeem because the URL has no group id at
		// redemption time (the token implies it).
		r.Get("/{id}/invites", groupInviteHandler.List)
		r.Post("/{id}/invites", groupInviteHandler.MintGroupJoin)
		r.Post("/{id}/invites/platform_plus", groupInviteHandler.MintPlatformPlus)
		r.Delete("/{id}/invites/{inviteID}", groupInviteHandler.Revoke)
		r.Post("/invites/redeem", groupInviteHandler.Redeem)

		// Phase 3 — group packs + duplication queue. Duplicate lives under
		// the group route because the source pack is the payload; add/modify
		// member paths live under a specific pack. The approval queue is
		// the admin-only terminus for NSFW→SFW duplications.
		r.Get("/{id}/packs", groupPackHandler.List)
		r.Post("/{id}/packs/duplicate", groupPackHandler.Duplicate)
		r.Post("/{id}/packs/{packID}/items", groupPackHandler.AddItem)
		r.Patch("/{id}/packs/{packID}/items/{itemID}", groupPackHandler.ModifyItem)
		r.Delete("/{id}/packs/{packID}/items/{itemID}", groupPackHandler.DeleteItem)
		r.Delete("/{id}/packs/{packID}", groupPackHandler.DeletePack)
		r.Post("/{id}/packs/{packID}/evict", groupPackHandler.Evict)

		r.Get("/{id}/duplication-queue", groupPackHandler.ListPending)
		r.Post("/{id}/duplication-queue/{queueID}/accept", groupPackHandler.AcceptPending)
		r.Post("/{id}/duplication-queue/{queueID}/reject", groupPackHandler.RejectPending)

		// Phase 5 — group notifications feed (admin-only reads, mark-read).
		r.Get("/{id}/notifications", groupNotificationHandler.List)
		r.Patch("/{id}/notifications/{nid}/read", groupNotificationHandler.MarkRead)
	})

	// Phase 2 — preview is intentionally unauthenticated. A platform+group
	// invite recipient has no account yet; a group_join recipient may be
	// logged out when they click the link. The endpoint returns only
	// public-shape group identity (name, description, classification),
	// matching the data the receiver would see on the join page anyway.
	r.Get("/api/groups/invites/preview", groupInviteHandler.Preview)

	// Startup auto-promotion sweep. Idempotent and best-effort: failures
	// are logged but never block boot. Runs unconditionally now that the
	// groups paradigm is GA — the sweep is cheap on an empty dataset.
	if rep, err := groupjobs.PromoteDormantAdmins(context.Background(), pool, logger); err != nil {
		logger.Error("startup: promote dormant admins", "error", err)
	} else {
		logger.Info("startup cleanup", "event", "group.auto_promote_pass",
			"scanned", rep.Scanned, "promoted", rep.Promoted, "no_candidate", rep.NoCandidate)
	}

		// WebSocket — intentionally not gated by RequireAuth. The handler
		// resolves identity itself (session cookie OR guest_token query
		// param) and rejects unauthenticated upgrades with a 401. See
		// api.WSHandler.resolveIdentity. The handshake itself stays under
		// the global limiter; the connection's per-message budget lives in
		// the hub (WSRateLimit).
		r.Get("/api/ws/rooms/{code}", wsHandler.ServeHTTP)
	}) // close globalLimiter Group

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
	// Tear down rate-limiter eviction loops after HTTP has drained so no
	// late request reaches a stopped limiter. Each Stop() blocks until its
	// goroutine returns, giving goleak-based tests a clean exit.
	authLimiter.Stop()
	inviteLimiter.Stop()
	globalLimiter.Stop()
	roomLimiter.Stop()
	uploadLimiter.Stop()
	exportLimiter.Stop()
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
