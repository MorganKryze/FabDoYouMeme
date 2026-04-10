# Architectural Decisions

ADR-style record of non-obvious architectural decisions. Each entry explains what was decided, why alternatives were rejected, and the consequences.

Format: `Status: Accepted` means the decision is in effect. `Status: Superseded by ADR-NNN` means this decision was replaced.

---

## ADR-001 — Magic Links Instead of Passwords

**Status**: Accepted

**Context**: FabDoYouMeme is invite-only with a small, known player base. Passwords create attack surface: credential stuffing, brute force, password storage breaches, forgot-password flows.

**Decision**: authentication is email + magic link only. No passwords are ever stored. Tokens are one-time use, 15-minute TTL, SHA-256 hashed in DB (raw token sent only by email).

**Consequences**: eliminates the largest class of auth attacks. Users must have email access to log in. The backend does not need a key management system — raw random tokens, SHA-256 stored. The 15-minute TTL and one-time use constraint mean a forwarded or intercepted link has a very short exploit window.

---

## ADR-002 — DB-Backed Sessions Instead of JWT

**Status**: Accepted

**Context**: two common session models: (a) opaque token stored in DB, (b) self-contained JWT signed with a secret key.

**Decision**: DB-backed opaque sessions. A random 32-byte token is stored as SHA-256 in `sessions`. Every authenticated request looks up the hash.

**Consequences**: logout is immediate (delete the row). Deactivated users or demoted admins take effect on next request — no "grace period" caused by a valid JWT. No signing key to rotate. Lookup overhead is negligible at self-hosted scale. JWT would add complexity (key rotation, token replay edge cases, revocation blacklist) with no benefit at this scale.

---

## ADR-003 — chi Router Instead of Gin or Echo

**Status**: Accepted

**Context**: Go HTTP router choice. Gin and Echo are popular but wrap `net/http` with their own context types. chi uses `net/http` interfaces directly.

**Decision**: use `go-chi/chi`. Any `net/http`-compatible middleware works without adaptation. No reflection. Trivially auditable. Handler signatures are standard Go.

**Consequences**: more verbose than Gin/Echo for some patterns, but the codebase remains readable without knowing the framework's conventions. External security auditors do not need chi-specific knowledge.

---

## ADR-004 — Svelte 5 Runes Instead of Stores

**Status**: Accepted

**Context**: Svelte 4 stores require `subscribe`/`unsubscribe` ceremony. Svelte 5 runes (`$state`, `$derived`, `$effect`) work inside `.svelte.ts` files outside components — enabling shared reactive state as plain classes.

**Decision**: global state as reactive Svelte 5 classes (`WsState`, `RoomState`, `UserState`) in `.svelte.ts` files. No stores, no Pinia, no Redux.

**Consequences**: reactive state is co-located with its logic. No `$:` syntax or `get(store)` calls. The pattern is unfamiliar to Svelte 4 developers but is simpler once understood. Components import singleton instances directly.

---

## ADR-005 — In-Memory Rate Limiting (Single-Instance Trade-off)

**Status**: Accepted

**Context**: rate limits prevent abuse. The simplest implementation stores counters in Go process memory (e.g., sliding-window map keyed by IP). This works perfectly when there is exactly one backend instance.

**Decision**: in-memory rate limiting per process. All `RATE_LIMIT_*` variables control Go-internal counters. No Redis dependency.

**Consequences**: correct for the single-host Docker Compose deployment model — there is only one backend process, so there is no inter-instance state divergence. If the backend is ever scaled horizontally, in-memory limits stop working as intended: an IP can bypass limits by routing to different instances. Mitigation for multi-instance: replace with Redis-backed token bucket (e.g., `go-redis` + sliding window). This limitation is documented in `ref-env-vars.md`. It is not a bug at the current deployment scale.

---

## ADR-006 — Sentinel UUID for Hard-Deleted Users

**Status**: Accepted

**Context**: when a user is hard-deleted (GDPR erasure), their `submissions` rows must be handled. Two options: (a) make `submissions.user_id` nullable (`NULL = deleted user`), (b) use a well-known sentinel UUID row.

**Decision**: sentinel UUID. A fixed row with `id = '00000000-0000-0000-0000-000000000001'` is seeded in migration 001 with `username = '[deleted]'`, `email = 'deleted@localhost'`, `is_active = false`. It must never be deleted. Before hard-deleting a user, `UPDATE submissions SET user_id = $sentinel WHERE user_id = $target` runs atomically in the same transaction as the DELETE.

**Consequences**: `submissions.user_id` stays `NOT NULL` — the FK constraint is preserved, queries never need `IS NULL` checks, and historical round scores remain intact. The display layer shows `[deleted]` for the sentinel username. The sentinel row is inert: it cannot log in (`is_active = false`), has no email deliverable, and holds no personal data beyond the placeholder values.

The contradictory note in the pre-redesign `04-api.md` ("backend sets user_id = NULL") is superseded by this decision.

---

## ADR-007 — Room Code Uniqueness via Application Retry

**Status**: Accepted

**Context**: room codes are 4 uppercase letters (456,976 combinations). The DB has a `UNIQUE` constraint on `rooms.code`. The business logic wants to allow code reuse 24h after a room finishes. Two options: (a) partial unique index based on `state` and `finished_at`, (b) unconditional `UNIQUE` constraint + application retry.

**Decision**: keep the unconditional `UNIQUE` constraint. On room creation, the backend generates a random 4-letter code and attempts `INSERT`. On `23505` (unique violation), it retries with a new code, up to 10 attempts. The 24h reuse logic is a display-level hint to the user, not a DB enforcement rule.

**Consequences**: the DB guarantee is simple and correct: two live rooms can never share a code simultaneously. The application retry is fast and virtually never needed at self-hosted scale (a small invite-only player base will never approach 456,976 simultaneous rooms). A partial unique index would add complexity with no practical benefit.

---

## ADR-008 — Pack–Game Type Compatibility at Application Layer

**Status**: Accepted

**Context**: packs are game-type-agnostic. A pack of meme images can be used for `meme-caption`, `meme-vote`, or any future type. Two options for enforcing compatibility at room creation: (a) junction table `pack_game_type_support`, (b) dynamic check via handler's `SupportedPayloadVersions()` at room creation.

**Decision**: no junction table. Compatibility is determined at `POST /api/rooms` by counting `game_items WHERE payload_version = ANY($supported_versions)`. The frontend additionally filters the pack dropdown to only show packs with ≥1 compatible item.

**Consequences**: admins do not need to tag packs per game type. New game types work with existing packs immediately. The API must expose `supported_payload_versions` in `GET /api/game-types/:slug` so the frontend can filter. The room creation endpoint returns `pack_no_supported_items` or `pack_insufficient_items` (see `ref-error-codes.md`) on failure.

---

## ADR-009 — No Host Transfer on Disconnect

**Status**: Accepted

**Context**: when the host disconnects during a game and the grace window expires, options are: (a) end the game, (b) transfer host role to another player.

**Decision**: end the game. Broadcast `game_ended` with `reason: "host_disconnected"`. No host transfer.

**Consequences**: simplicity over completeness. Host-initiated flow control (`start`, `next_round`) is lost when the host leaves. Acceptable at self-hosted scale with a small, known player base — the host is typically the person running the game session, not an anonymous stranger. Implementing host transfer would require UI changes, WS protocol additions, and edge-case handling not worth the complexity.

---

## ADR-010 — Two-Pass Item Reorder Strategy

**Status**: Accepted

**Context**: `UNIQUE (pack_id, position)` makes naive in-place position swaps fail because the intermediate state creates duplicates. Two options: (a) two-pass update (shift all to `position + 10000`, then set final values), (b) deferred constraint (`DEFERRABLE INITIALLY DEFERRED`).

**Decision**: two-pass update. No schema changes. Both passes run in a single transaction.

**Consequences**: simpler than deferred constraints and equally correct. The shift-by-10000 trick is a one-liner in SQL. Deferred constraints require an `ALTER TABLE` and add per-transaction overhead. The two-pass approach is also more portable if the DB engine changes.

---

## ADR-011 — `SameSite=Strict` as the Sole CSRF Defense

**Status**: Accepted

**Context**: the server does not implement CSRF tokens, double-submit cookies, or `Origin`-header checks on state-changing REST endpoints. The only barrier against cross-site request forgery is the `SameSite=Strict` attribute on the `session` cookie set in `backend/internal/auth/tokens.go`. The 2026-04-10 review (finding 5.D) flagged this as a HIGH-severity concern because the defense is browser-enforced, invisible on the server side, and one small relaxation (e.g. lowering to `SameSite=Lax` to fix magic-link UX) silently opens the entire authenticated API to CSRF.

**Decision**: `SameSite=Strict` is the canonical CSRF control for the `session` cookie and **must not be relaxed**. Concretely:

1. Every `http.Cookie{Name: "session", ...}` in the backend sets `SameSite: http.SameSiteStrictMode`. There is no "Lax" or "None" variant.
2. CI enforces this invariant with a grep-based lint step in `.github/workflows/backend.yml` that fails the build if `SameSite:` appears on a session-cookie line with any value other than `http.SameSiteStrictMode`. The rule lives next to the file it protects (`backend/internal/auth/tokens.go`) so a reviewer seeing a cookie diff will see the lint failure in the same PR.
3. Magic-link UX constraints that "need" cross-site cookie flow are solved at the _link_ layer (server-side handoff that re-issues the cookie on the canonical domain), never by weakening `SameSite`.
4. Double-submit-cookie CSRF tokens remain a documented _future enhancement_ for defense-in-depth against non-conforming browsers, but are explicitly out of scope for the current deployment model (modern evergreen browsers only, self-hosted, known player base).

**Consequences**: the session cookie cannot be sent on any cross-site request — including clicks on external links that land on authenticated endpoints — which is the intended behavior. Users coming from an external email client that does not open the same-origin browser session must re-authenticate via magic link, which is acceptable at this scale. A future change that relaxes `SameSite` (for any reason) will fail CI and require an explicit ADR supersession of this one, preventing a quiet regression. Contributors proposing double-submit-cookie tokens should write a new ADR that supersedes this one rather than editing it in place, so the audit trail remains intact.

**Related**: finding 5.D in `docs/review/2026-04-10/05-security.md`.
