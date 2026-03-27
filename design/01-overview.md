# 01 — Project Overview

## What It Is

FabDoYouMeme is a self-hosted, invite-only **multi-game platform** that launches with meme-style games (caption, match, vote) and is architected to host any turn-based party game. Players join live multiplayer rooms or practice solo. All content (images, game packs) is managed by admins.

**License**: GPLv3
**Hosting**: Self-hosted on personal hardware, Docker Compose only
**Reverse proxy**: Pre-existing (assumed to route `/api/*` to backend and `/*` to frontend)

---

## Design Priorities

1. **Least attack surface** — no passwords, no public asset access, all secrets injected via env, minimal exposed ports
2. **Multi-game extensibility** — game types are registered handler units; adding a new type requires no schema or protocol changes
3. **Simplicity** — single-machine Docker Compose footprint; no distributed systems complexity

---

## Tech Stack

| Layer          | Technology                            | Rationale                                                                                                       |
| -------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Frontend       | SvelteKit (`adapter-node`) + Svelte 5 | Reactive, lightweight, SSR support, Svelte 5 runes enable fine-grained reactivity without heavy state libraries |
| Styling        | Tailwind CSS v4 + shadcn-svelte       | Utility-first, Svelte 5-compatible component primitives, no custom CSS framework overhead                       |
| Backend API    | Go + `chi` router                     | Fast, tiny images, goroutine concurrency for WebSockets; `chi` is stdlib-compatible with zero reflection        |
| Database       | PostgreSQL 17                         | Strongly typed, JSONB for flexible payloads, reliable ACID guarantees for game state                            |
| File storage   | Rustfs                                | S3-compatible, Rust-native, self-hostable; replaces MinIO with a smaller footprint                              |
| DB migrations  | `golang-migrate`                      | CLI + library, up/down semantics, integrates cleanly with CI                                                    |
| Query layer    | `sqlc`                                | Generates type-safe Go from raw SQL; no ORM, no reflection, no runtime surprises                                |
| WebSockets     | `gorilla/websocket`                   | De-facto Go WS library; exposes read limits, ping/pong, and close frame control                                 |
| Email          | `go-mail` (wneessen)                  | Idiomatic Go SMTP with STARTTLS; used for magic link delivery                                                   |
| Session tokens | Opaque tokens (DB-backed)             | Simpler and instantly revocable compared to JWT; no signing key to manage                                       |
| S3 client      | `aws-sdk-go-v2/s3`                    | Works with any S3-compatible backend including Rustfs                                                           |
| Container      | Docker Compose                        | Single-machine orchestration; straightforward service graph                                                     |

### Key decisions

**Why magic links instead of passwords?**
Passwords require mitigation for credential stuffing, brute force, and breach exposure. On an invite-only platform where users are known, eliminating passwords removes the entire credential-management attack surface. Magic links are one-time-use, short-lived, and rely on email as the second factor. The only thing stored is a SHA-256 token hash — nothing crackable if the DB leaks.

**Why not JWT?**
For a closed, invite-only platform on personal hardware, DB-backed sessions are simpler, instantly revocable (logout = delete row), and eliminate token-replay edge cases. Session lookup overhead is negligible at this scale.

**Why `chi` over Gin/Echo?**
Chi uses `net/http` interfaces directly — no reflection, trivially auditable, and any standard Go HTTP middleware works with it unchanged.

**Why Svelte 5 runes over stores?**
Svelte 5 runes work in `.svelte.ts` files (outside components), enabling shared reactive state without the subscribe/unsubscribe ceremony. Fine-grained reactivity avoids the "whole array invalidates on one change" problem in Svelte 4.

---

## Repository Structure

```plain
FabDoYouMeme/
├── backend/
│   ├── cmd/
│   │   └── server/              # main.go — wires everything, registers game handlers
│   ├── internal/
│   │   ├── auth/                # session management, magic link logic, invite logic
│   │   ├── game/                # game type registry, hub, room/round lifecycle
│   │   │   ├── registry.go      # Register() + Dispatch()
│   │   │   ├── hub.go           # WebSocket hub (per-room goroutine)
│   │   │   └── types/
│   │   │       └── meme_caption/ # meme-caption handler (implements GameTypeHandler)
│   │   ├── storage/             # Rustfs / S3 client wrapper (interface-backed)
│   │   ├── email/               # email template rendering + SMTP sending
│   │   ├── middleware/          # auth, rate-limit, structured logging, request ID
│   │   └── config/              # env-based config loading (all fields, defaults)
│   ├── db/
│   │   ├── migrations/          # golang-migrate .sql files (up + down)
│   │   └── queries/             # sqlc .sql files → generated Go in db/sqlc/
│   ├── Dockerfile
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/      # shared UI components (Button, Avatar, Timer, etc.)
│   │   │   ├── state/           # Svelte 5 reactive state classes (ws, room, user)
│   │   │   ├── api/             # typed fetch wrappers for each REST endpoint
│   │   │   └── games/           # game type plugins
│   │   │       └── meme-caption/
│   │   │           ├── SubmitForm.svelte
│   │   │           ├── VoteForm.svelte
│   │   │           ├── ResultsView.svelte
│   │   │           ├── GameRules.svelte
│   │   │           └── index.ts # re-exports all four components
│   │   ├── routes/              # SvelteKit file-based routing (see 07-frontend.md)
│   │   └── app.html
│   ├── Dockerfile
│   ├── svelte.config.js
│   └── package.json
├── design/                      # architecture reference (this folder)
├── docker-compose.yml
├── docker-compose.override.yml  # dev overrides (Mailpit, volume mounts)
├── CLAUDE.md
├── LICENSE
└── README.md
```
