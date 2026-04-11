# Overview

## What it is

FabDoYouMeme is a self-hosted, invite-only multiplayer party game platform. It runs on personal hardware via Docker Compose and is designed to host turn-based games — starting with meme captioning — for a small, known player base.

**License:** GPLv3  
**Deployment:** Single machine, Docker Compose only  
**Access model:** Invite-only; no public registration

---

## Design priorities

### 1. Least attack surface

No passwords are ever created or stored. Authentication uses magic links — one-time-use, 15-minute tokens delivered by email. The only thing stored in the database is the SHA-256 hash of the raw token; nothing crackable if the database leaks.

All secrets are injected via environment variables. No signing keys are required. Ports are not publicly exposed — a pre-existing reverse proxy routes traffic to the services.

### 2. Multi-game extensibility

Game types are self-contained handler units registered at startup. Adding a new game — trivia, drawing, anything turn-based — requires implementing a single Go interface and calling `Register()` in `main.go`. No database schema changes. No WebSocket protocol changes. No frontend rewiring.

### 3. Simplicity over scale

This is a single-machine deployment. There is no distributed systems complexity, no message queue, no service mesh. The goal is correctness and maintainability for a small, trusted player base, not horizontal scalability.

---

## Tech stack

| Layer         | Technology                      | Why                                                                                                                                        |
| ------------- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Frontend      | SvelteKit + Svelte 5            | Reactive, lightweight, SSR support; Svelte 5 runes enable shared reactive state without subscribe/unsubscribe ceremony                     |
| Styling       | Tailwind CSS v4 + shadcn-svelte | Utility-first; Svelte 5-compatible component primitives                                                                                    |
| Backend       | Go + `chi` router               | Fast, small Docker images, native goroutine concurrency for WebSockets; chi uses `net/http` directly — no framework-specific context types |
| Database      | PostgreSQL 17                   | ACID guarantees for game state; JSONB for flexible submission payloads                                                                     |
| File storage  | RustFS                          | S3-compatible, self-hostable; replaces MinIO with smaller footprint                                                                        |
| DB migrations | `golang-migrate`                | Up/down semantics, CLI + library, integrates with CI                                                                                       |
| Query layer   | `sqlc`                          | Generates type-safe Go from raw SQL; no ORM, no reflection                                                                                 |
| WebSockets    | `gorilla/websocket`             | Read limits, ping/pong, close frame control                                                                                                |
| Email         | `go-mail` (wneessen)            | Idiomatic Go SMTP with STARTTLS                                                                                                            |
| Sessions      | Opaque tokens, DB-backed        | Instantly revocable; no signing key; logout = delete row                                                                                   |

---

## Key decisions

### Why magic links instead of passwords?

On an invite-only platform with a known player base, eliminating passwords removes an entire attack class: credential stuffing, brute force, password reuse, and storage breaches. The only shared secret is the link delivered to the user's inbox. A forwarded or intercepted link is useless after 15 minutes or one use.

### Why DB-backed sessions instead of JWT?

JWT tokens are self-contained — they cannot be revoked before expiry without a blacklist. DB sessions can be revoked instantly by deleting a row. This matters for deactivating users and for admin role changes, which take effect on the next request with no grace period. At this deployment scale, the per-request session lookup is negligible overhead.

### Why `chi` over Gin or Echo?

Chi uses `net/http` interfaces directly. Any standard Go HTTP middleware works with it unchanged. Handler signatures are plain Go. An auditor unfamiliar with chi can read the code without knowing the framework's conventions.

### Why Svelte 5 runes over stores?

Svelte 5 runes work inside `.svelte.ts` files outside of components. This enables shared reactive state as plain classes (`WsState`, `RoomState`, `UserState`) with no `subscribe`/`unsubscribe` wiring. Components import singleton instances directly and the state updates propagate automatically.

### Why a sentinel UUID for deleted users?

When a user is hard-deleted (GDPR erasure), their submission history must be preserved for round integrity. Rather than making `submissions.user_id` nullable (which breaks FK constraints and requires `IS NULL` checks throughout), a fixed placeholder row (`id = 00000000-0000-0000-0000-000000000001`, `username = '[deleted]'`) is seeded in the first migration. Hard-delete replaces the user's `user_id` with the sentinel before removing the row. The sentinel is inert: it cannot log in, has no deliverable email, and holds no personal data.

Full ADR records for all ten architectural decisions live in `docs/reference/decisions.md`.

---

## What it is not

- Not a scalable SaaS platform — designed for one machine, one Docker Compose stack
- Not open to the public — every player requires an invite from an admin
- Not a general-purpose CMS — content management (packs, items) is admin-only
- Not JWT-based — session state lives in the database, not in tokens
