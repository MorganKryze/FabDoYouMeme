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

**Context**: packs are game-type-agnostic. A pack of meme images can be used for `meme-caption`, `meme-vote`, or any future type. Two options for enforcing compatibility at room creation: (a) junction table `pack_game_type_support`, (b) dynamic check via the handler's declared pack requirements at room creation.

**Decision**: no junction table. Compatibility is determined at `POST /api/rooms` by counting `game_items WHERE payload_version = ANY($role_versions)` for each role declared in the handler's `RequiredPacks()`. The frontend additionally filters the pack dropdown to only show packs with ≥1 compatible item via `GET /api/packs?game_type=<slug>&role=<role>`.

**Consequences**: admins do not need to tag packs per game type. New game types work with existing packs immediately. The API exposes `required_packs` in `GET /api/game-types/:slug` so the frontend can filter. The room creation endpoint returns per-role error codes (`image_pack_no_supported_items`, `image_pack_insufficient`, `text_pack_no_supported_items`, `text_pack_insufficient`, and the companion `*_required` / `*_not_applicable` codes — see `ref-error-codes.md`) on failure. See [ADR-013](#adr-013--multi-pack-rooms-via-explicit-role-columns) for how multi-role game types (e.g. `meme-vote`) extend this model without a junction table.

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

**Context**: the server does not implement CSRF tokens, double-submit cookies, or `Origin`-header checks on state-changing REST endpoints. The only barrier against cross-site request forgery is the `SameSite=Strict` attribute on the `session` cookie set in `backend/internal/auth/tokens.go`. This is a HIGH-severity concern because the defense is browser-enforced, invisible on the server side, and one small relaxation (e.g. lowering to `SameSite=Lax` to fix magic-link UX) silently opens the entire authenticated API to CSRF.

**Decision**: `SameSite=Strict` is the canonical CSRF control for the `session` cookie and **must not be relaxed**. Concretely:

1. Every `http.Cookie{Name: "session", ...}` in the backend sets `SameSite: http.SameSiteStrictMode`. There is no "Lax" or "None" variant.
2. CI enforces this invariant with a grep-based lint step in `.github/workflows/backend.yml` that fails the build if `SameSite:` appears on a session-cookie line with any value other than `http.SameSiteStrictMode`. The rule lives next to the file it protects (`backend/internal/auth/tokens.go`) so a reviewer seeing a cookie diff will see the lint failure in the same PR.
3. Magic-link UX constraints that "need" cross-site cookie flow are solved at the _link_ layer (server-side handoff that re-issues the cookie on the canonical domain), never by weakening `SameSite`.
4. Double-submit-cookie CSRF tokens remain a documented _future enhancement_ for defense-in-depth against non-conforming browsers, but are explicitly out of scope for the current deployment model (modern evergreen browsers only, self-hosted, known player base).

**Consequences**: the session cookie cannot be sent on any cross-site request — including clicks on external links that land on authenticated endpoints — which is the intended behavior. Users coming from an external email client that does not open the same-origin browser session must re-authenticate via magic link, which is acceptable at this scale. A future change that relaxes `SameSite` (for any reason) will fail CI and require an explicit ADR supersession of this one, preventing a quiet regression. Contributors proposing double-submit-cookie tokens should write a new ADR that supersedes this one rather than editing it in place, so the audit trail remains intact.

---

## ADR-012 — Danger-Zone Routes Unmounted (Not 403) in Production

**Status**: Accepted

**Context**: the admin UI exposes a set of destructive actions — "wipe game history", "wipe packs and media", "wipe invites", "force logout everyone", and "full reset to first boot" — that permanently delete data and empty the object storage bucket. They exist to reset a dev or preprod deployment to a known state without losing the Postgres volume. In production they have no legitimate use: every action is equivalent to data loss that would require a backup restore. The question is how to disable them in prod. Options: (a) register the routes and return `403 forbidden` when `APP_ENV=prod`, (b) skip route registration entirely when `APP_ENV=prod` so the chi router responds with its default `404 not found`, (c) ship a separate binary for prod that omits the handlers.

**Decision**: option (b). The backend conditionally registers the `/api/admin/danger/*` route group in `cmd/server/main.go` under `if cfg.AppEnv != "prod"`. When `APP_ENV=prod` (or is unset — the default), the routes are never mounted and chi's default 404 handler responds. The frontend mirrors this with a matching gate: the sidebar nav link is conditionally rendered based on `PUBLIC_APP_ENV`, and the `/admin/danger` page's `+page.ts` load function throws `error(404)` when `PUBLIC_APP_ENV=prod`. Both backend and frontend default to `prod` when their respective env vars are unset, so a partially configured deployment fails safe to hidden.

**Consequences**:

1. **No information leakage.** A prod instance returns the same 404 response for `/api/admin/danger/wipe-game-history` as for `/api/admin/does-not-exist`. An attacker cannot distinguish "this deployment has a danger zone but I'm not authorized" from "this deployment has no such feature", which removes the route from the surface worth attacking. A 403 response would confirm the feature exists and invite focused probing (credential stuffing, session hijacking, XSS targeting admin sessions).
2. **Single source of truth is the route registration.** The gate lives in `main.go` next to the handler constructor. A reviewer seeing a diff to `NewDangerHandler(...)` sees the gate on the same screen. Returning 403 from within the handler would put the gate one file removed from the route wiring, increasing the chance of a future refactor dropping the check.
3. **Defense in depth at three layers.** Backend route registration, frontend sidebar rendering, and frontend page load function all read the stage marker independently and all fail safe to hidden when unset. Breaking any one layer (e.g. forgetting `PUBLIC_APP_ENV` in a new deploy) still results in the feature being hidden — the backend routes still return 404. This is the inverse of a 403-based design where a single missing check opens the feature.
4. **Server-side phrase validation is still required.** The routes being unmounted in prod is not a substitute for per-action confirmation. Every non-prod invocation requires a typed phrase both in the UI modal and as a JSON body field, validated server-side against a hardcoded expected value. This catches two distinct failure modes: (a) an operator mis-enabling the danger zone in a shared preprod instance, and (b) a CSRF-style forged POST from a malicious site. The `SameSite=Strict` guarantee from [ADR-011](#adr-011--samesitestrict-as-the-sole-csrf-defense) already rules out (b) in conforming browsers, but the typed phrase adds a belt-and-braces confirmation that's visible in server logs.
5. **Option (c) rejected as too costly.** Shipping a separate prod binary would require a second Docker image, a second CI pipeline, and divergent build tags — all to gate ~200 lines of handler code. The conditional-route approach gives the same effective surface reduction at the cost of a single `if` in `main.go`.
6. **Audit logging is unconditional.** Every successful danger-zone invocation writes an entry to the `admin_audit` table with the action name, acting admin ID, and a JSON summary of what was deleted. This exists regardless of stage — dev and preprod use it as well, because the same logs are useful when investigating "who reset the staging env yesterday".

---

## ADR-013 — Multi-Pack Rooms via Explicit Role Columns

**Status**: Accepted

**Context**: `meme-vote` needs two content streams — the image stack (same as `meme-caption`) and a caption pack dealt to players. The schema had one `rooms.pack_id`. Three options for associating a room with N packs: (a) add a nullable `rooms.text_pack_id` column referencing `game_packs(id)` and let handlers declare the roles they consume, (b) generalise to a `room_packs(room_id, role, pack_id)` join table, (c) store the secondary pack ID inside `rooms.config` JSON.

**Decision**: option (a). `rooms.text_pack_id` is a nullable FK to `game_packs(id)`. Game-type handlers declare their pack roles via `GameTypeHandler.RequiredPacks() []PackRequirement`; the API layer iterates this list at `POST /api/rooms` to validate each pack's compatibility (count items matching `payload_version = ANY($role_versions)`) and minimum size (`MinItemsFn(cfg, maxPlayers)`). Single-role game types (`meme-caption`) leave `text_pack_id` null; two-role types (`meme-vote`) populate it. The frontend renders one pack picker per declared role using `GET /api/packs?game_type=<slug>&role=<role>`.

**Consequences**:

1. **Rooms can reference up to two packs with zero extra plumbing.** Every `rooms`-touching query stays as-is because `text_pack_id` is nullable; only queries that need the caption pack opt in via an extra join.
2. **Role-keyed error codes keep diagnostics actionable.** Validation failures surface per role (`image_pack_no_supported_items`, `text_pack_insufficient`, `text_pack_required`, `image_pack_not_applicable`, etc.) — the UI can point directly at the offending picker. See [error-codes.md](error-codes.md).
3. **A future game type needing a third role promotes the pattern to a join table.** The migration is mechanical: introduce `room_packs(room_id, role, pack_id)`, backfill from `pack_id` / `text_pack_id`, migrate queries, drop the columns. Deferring this is YAGNI — no third role is planned, and the join table would churn every existing query and test today without adding value.
4. **Storing `text_pack_id` in `rooms.config` (option c) rejected.** Packs are first-class FKs everywhere else (`submissions.pack_id` would have no DB-enforced integrity if the second pack lived in JSON). Role-keyed columns keep the invariant that every pack a room uses is discoverable and FK-protected.
