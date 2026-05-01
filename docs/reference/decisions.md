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

**Context**: packs are game-type-agnostic. A pack of meme images can be used for `meme-freestyle`, `meme-showdown`, or any future type. Two options for enforcing compatibility at room creation: (a) junction table `pack_game_type_support`, (b) dynamic check via the handler's declared pack requirements at room creation.

**Decision**: no junction table. Compatibility is determined at `POST /api/rooms` by counting `game_items WHERE payload_version = ANY($role_versions)` for each role declared in the handler's `RequiredPacks()`. The frontend additionally filters the pack dropdown to only show packs with ≥1 compatible item via `GET /api/packs?game_type=<slug>&role=<role>`.

**Consequences**: admins do not need to tag packs per game type. New game types work with existing packs immediately. The API exposes `required_packs` in `GET /api/game-types/:slug` so the frontend can filter. The room creation endpoint returns per-role error codes (`image_pack_no_supported_items`, `image_pack_insufficient`, `text_pack_no_supported_items`, `text_pack_insufficient`, and the companion `*_required` / `*_not_applicable` codes — see `ref-error-codes.md`) on failure. See [ADR-013](#adr-013--multi-pack-rooms-via-explicit-role-columns) for how multi-role game types (e.g. `meme-showdown`) extend this model without a junction table.

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

**Context**: `meme-showdown` needs two content streams — the image stack (same as `meme-freestyle`) and a caption pack dealt to players. The schema had one `rooms.pack_id`. Three options for associating a room with N packs: (a) add a nullable `rooms.text_pack_id` column referencing `game_packs(id)` and let handlers declare the roles they consume, (b) generalise to a `room_packs(room_id, role, pack_id)` join table, (c) store the secondary pack ID inside `rooms.config` JSON.

**Decision**: option (a). `rooms.text_pack_id` is a nullable FK to `game_packs(id)`. Game-type handlers declare their pack roles via `GameTypeHandler.RequiredPacks() []PackRequirement`; the API layer iterates this list at `POST /api/rooms` to validate each pack's compatibility (count items matching `payload_version = ANY($role_versions)`) and minimum size (`MinItemsFn(cfg, maxPlayers)`). Single-role game types (`meme-freestyle`) leave `text_pack_id` null; two-role types (`meme-showdown`) populate it. The frontend renders one pack picker per declared role using `GET /api/packs?game_type=<slug>&role=<role>`.

**Consequences**:

1. **Rooms can reference up to two packs with zero extra plumbing.** Every `rooms`-touching query stays as-is because `text_pack_id` is nullable; only queries that need the caption pack opt in via an extra join.
2. **Role-keyed error codes keep diagnostics actionable.** Validation failures surface per role (`image_pack_no_supported_items`, `text_pack_insufficient`, `text_pack_required`, `image_pack_not_applicable`, etc.) — the UI can point directly at the offending picker. See [error-codes.md](error-codes.md).
3. **A future game type needing a third role promotes the pattern to a join table.** The migration is mechanical: introduce `room_packs(room_id, role, pack_id)`, backfill from `pack_id` / `text_pack_id`, migrate queries, drop the columns. Deferring this is YAGNI — no third role is planned, and the join table would churn every existing query and test today without adding value.
4. **Storing `text_pack_id` in `rooms.config` (option c) rejected.** Packs are first-class FKs everywhere else (`submissions.pack_id` would have no DB-enforced integrity if the second pack lived in JSON). Role-keyed columns keep the invariant that every pack a room uses is discoverable and FK-protected.

---

## ADR-014 — Internationalization Architecture (EN + FR)

**Status**: Accepted

**Context**: the platform is English-only. Adding a second language (French, initial target) touches four distinct string surfaces — UI chrome, rich authored copy (`tonePools`, brand voice), transactional emails, and user-generated pack content — each with different translation semantics. A naïve single-mechanism approach (e.g. one i18n library for everything) either forces literal translation of voice-sensitive copy or leaves transactional backend strings untranslated. The decision must also answer four orthogonal questions: who picks the locale (operator vs per-user vs hybrid), how it is carried at runtime (cookie vs URL prefix vs subdomain), how pack content is handled (translated vs language-tagged vs mixed), and how the design extends to future locales without schema reshape.

**Decision**: a layered design matched to each string surface.

1. **Locale source — hybrid.** New env var `PUBLIC_DEFAULT_LOCALE` sets the deployment-wide default; a new `users.locale` column (`CHECK (locale IN ('en', 'fr'))`, default `'en'`) overrides it per authenticated user. Unauthenticated requests resolve via cookie → `Accept-Language` → default, in that order.
2. **Runtime carrier — cookie.** `hooks.server.ts` sets `event.locals.locale` on every request, composing after the existing session-loading hook. URLs stay locale-free (invite/magic-link tokens remain portable across languages). Paraglide's SvelteKit adapter inlines the chosen locale into the SSR response so first paint never flickers.
3. **Frontend mechanism — Paraglide JS (inlang).** Single flat JSON per locale at `frontend/messages/{en,fr}.json`, domain-prefixed snake_case keys (`admin_invites_create_button`, `errors_e_room_full`). Compiled output committed for deterministic builds. Error codes emitted by the backend stay English (diagnostic fallback); the frontend maps `code → m['errors_' + code.toLowerCase()]()`.
4. **Rich authored copy — parallel pools, not translations.** `tonePools` becomes `Record<Locale, Record<Tone, string[]>>`, sampled by the current locale. FR entries are hand-authored by native speakers against a new voice guide in `docs/brand.md` (tu throughout, *second degré* register, native idioms, same 5-tone arc). Machine translation is explicitly rejected — voice is distinctive enough that DeepL'd pools would be worse than no FR.
5. **Backend emails — locale-indexed template tree.** `backend/internal/email/templates/{en,fr}/*.html|txt` + `subjects.json`, loaded at startup into `map[locale]map[name]*template.Template`. A boot-time check fails fast if any locale is missing a template name another locale has. Send-site locale = `users.locale` for authenticated flows, register-form value for registration, inviter-chosen `invites.locale` for invitations, `PUBLIC_DEFAULT_LOCALE` for seed-admin bootstrap.
6. **User-generated packs — language-tagged, filtered at pick.** New `game_packs.language` column (same CHECK constraint), required on create, mutable+audited post-hoc. Pack picker shows the viewer's locale packs first; remainder collapses under "Other languages / Autres langues". Mixed-language rooms allowed silently (respects host agency). Items inherit `pack.language`; no per-item override.
7. **Schema delta — three columns, one migration.** `users.locale`, `game_packs.language`, `invites.locale`. Nothing else reshapes. Adding a future locale = extend the CHECK constraints, add `messages/<code>.json`, add `tonePools.<code>`, add `templates/<code>/` tree, optionally document voice rules. Purely additive.

**Consequences**:

1. **Each surface gets the right tool.** UI chrome stays lean via Paraglide's compile-time keys; tonePools stay native-voice via parallel authoring; emails stay Go-idiomatic via locale-indexed templates; pack content stays authored-as-typed via the language tag. No surface pays for another surface's requirements.
2. **Invariants preserved.** Single-machine simplicity (no translation-memory service, no crowdsource tool, no runtime key lookup overhead), least attack surface (no third-party translation API calls, no auto-detected user data leaving the host), pluggable-handler pattern unchanged (new game types still add one handler + `Register()`; they now also add two catalog keys per locale — `game_<slug>_name`, `game_<slug>_description`).
3. **Voice integrity protected.** Explicitly rejecting machine translation is load-bearing. The `tonePools` + email templates carry the brand, and the FR voice guide in `brand.md` is the reviewer's rubric. A future contributor cannot silently DeepL their way through a catalog — the PR will fail the voice-guide review.
4. **Parity enforced in CI.** `npm run i18n:check` fails on missing keys, `[FR]` TODO markers, or tonePool bucket under-population (<5 per tone). Backend template tree is validated at server startup and in a unit test. Drift cannot land.
5. **Mixed-language rooms are a feature, not a bug.** Host picks packs across languages deliberately (e.g. French pack for an EN group practicing French). UI chrome still renders per-viewer locale, pack content renders in authored language. The design respects host agency rather than enforcing locale purity.
6. **Tradeoff — authoring burden is real.** Roughly 300–500 UI keys × 2 locales, 40+ `tonePools` entries × 2 locales, 6 email templates × 2 locales, and a FR voice-guide section. Phase 0 extracts the existing EN strings; subsequent phases author FR content and wire the UX affordances. Estimated solo effort is ~3–4 weeks. Accepted because the alternative — bolting i18n on later per-request — becomes exponentially more expensive as surface grows.
7. **Rejected alternatives.** URL-prefixed locales (`/fr/home`, `/en/home`) rejected because this app is not SEO-sensitive (only `/` is public), and prefix-stripping for invite/magic-link tokens adds routing complexity for no gain. Subdomain-per-locale rejected as overkill for single-machine self-hosted. `i18next` / `svelte-i18n` rejected in favor of Paraglide because compile-time keys produce smaller bundles, tree-shake unused strings, and surface missing keys as TypeScript errors rather than runtime fallbacks. A single unified mechanism across all four surfaces (UI, tonePools, emails, packs) rejected because tonePools are sampled (not keyed) and pack content is language-tagged (not translatable) — forcing one tool on all four produces worse results everywhere.


---

## ADR-015 — Per-Room Player Cap Drives Pack-Size Validation

**Status**: Accepted

**Context**: Pack-size requirements (`MinItemsFn` per pack role, e.g. `hand_size × max_players + (round_count − 1) × max_players` for `meme-showdown` captions) were always evaluated against the handler's manifest cap (12 players). A 4-friend group hosting a `meme-showdown` room therefore needed a 432-item caption pack to pass `POST /api/rooms` validation, even though their actual lobby would never exceed 4 players. The `MaxPlayers()` enforcement at the WebSocket join hook used the same manifest cap, so there was no way to communicate "I plan a small room" to the validator.

**Decision**: hosts choose a per-room player cap at room creation, stored on `rooms.config.max_players` and clamped to `[manifest.min_players, manifest.max_players]`. Both `ValidatePackRequirements` (in `backend/internal/api/rooms.go`) and the join-time cap check (in `backend/internal/game/hub.go`) read this value instead of the manifest cap. When `config.max_players` is absent (legacy rows) or zero, both call sites fall back to `handler.MaxPlayers()` so existing rooms keep working without a backfill migration.

**Consequences**:

1. **Smaller packs unlock smaller rooms.** A 4-player `meme-showdown` lobby with default config now needs `5 × 4 + (5−1) × 4 = 36` captions instead of `5 × 12 + (5−1) × 12 = 108`. The same pack can serve a max-cap room or a friend-sized room — the host picks the bound at create time.
2. **Single field, no schema migration.** `max_players` lives in `rooms.config` (JSONB) and is enforced by the existing `ValidateAndFill` machinery. No `ALTER TABLE`, no sqlc regeneration, no rewrite of every room query. The per-room cap travels through `RoomConfig` to the hub via `Manager.GetOrLoad`, which decodes the JSON once at hub init.
3. **The hub's join-cap is now informed.** `hub.handleRegister` reads `Hub.effectiveMaxPlayers` first (set from the room row) and only falls back to the handler cap when zero. A host who picks `max_players=4` sees a 5th invitee bounced from the lobby with the existing `room_full` error code.
4. **Hosts must commit to a cap up front.** "We'll see how many show up" rooms are slightly less ergonomic — the host has to think about the cap before validation. The platform's invite-only nature makes this a soft constraint, but it is real. Mitigated by the studio surfacing pack ↔ game-type compatibility and a worst-case items counter so the host knows what they're signing up for.
5. **Rejected alternative — dedicated `rooms.max_players` column.** Considered, would unlock indexed queries (`WHERE max_players >= ?`), but no current path needs that and it would have forced touching every room SQL file plus a regeneration. The JSONB field reuses the existing partial-PATCH lifecycle (`PATCH /api/rooms/:code/config` accepts `max_players` for free) and keeps `ValidateAndFill` as the single bounds-enforcement seam.
6. **Rejected alternative — pack-level `accepted_payload_versions` declaration.** Would let the pack constrain itself ("this is for prompt-showdown") but would prevent legitimate pack reuse across game types (image v1 already serves both `meme-freestyle` and `meme-showdown`). The discoverability story is solved instead by surfacing `handler.RequiredPacks()` in the studio (the kind → game-types reverse index), which costs nothing schema-side.


---

## ADR-016 — Weighted Multi-Pack Rooms via `room_packs` Join Table

**Status**: Accepted

**Context**: The room→pack relationship was strictly one-to-one per role: `rooms.pack_id` (primary) and `rooms.text_pack_id` (secondary, nullable). A host who wanted to mix sources — "60% house image pack, 25% group's hand-curated set, 15% NSFW spice" — had to duplicate items into a single mega-pack. ADR-013 named the future direction ("introduce `room_packs(room_id, role, pack_id)`, backfill, drop the columns"); this ADR cashes that in and adds per-pack `weight` so the host can bias the mix without ever editing pack contents.

**Decision**: replace the two pack columns on `rooms` with a join table `room_packs(room_id, role, pack_id, weight)`, PK `(room_id, role, pack_id)`. Weights are positive integers with relative semantics — `3:1:1` and `30:10:10` produce the same sampler. The validator (`game.ValidatePackRequirements`) takes `map[PackRole][]WeightedPackRef` and uses the **pool model** for capacity: `MinItemsFn` is checked against the SUM of compatible items across the role's packs (a 5%-weighted 10-item pack can ride alongside a 95%-weighted 500-item pack). Each individual pack must still hold at least one compatible item — that's a misconfig, not a weight question.

Runtime selection:
- Primary role (per-round prompt/image): the pack list is shuffled by `-ln(rand)/weight`; the first pack with an unplayed item wins. Subsequent packs are the fallback order.
- Secondary role (hand-deck): the union of all listed packs' items is keyed the same way, sorted, and consumed by the existing `handState.drawOne` (which pops the tail). Marginal distribution matches the weights; refill keeps reading from the same shuffled deck.

**Consequences**:

1. **Hosts can mix without editing packs.** The studio remains the place to author content; the host page is now the place to compose the *room's* content from existing packs at runtime.
2. **Single-pack rooms stay one click.** The host page's per-role picker degrades to today's UX when the host adds no extra rows; weights default to `1` and the pool model collapses to the old single-pack check.
3. **Hard-cut migration, not additive.** Project is in active dev with no live production data. Migration `016_room_packs.up.sql` creates the join table, backfills from the existing columns at `weight=1` for the four shipped game types, and drops both columns in the same transaction. The down migration restores the columns by collapsing each role's mix to its highest-weighted pack — lossy by design, faithful to "best-effort restoration".
4. **`text_pack_id` finally dies.** The historical artefact named in ADR-013 is gone. The replay header still exposes a `text_pack_name` field for backwards-compat with the JSON shape but it's always empty now; clients that already treat it as optional keep working.
5. **Trade-off — discoverability of the multi-pack feature.** Until the host opens the picker and sees "+ Add pack", they may not know the feature exists. Mitigated by the studio's existing capacity pill: a small pack tagged green tells the host "this alone covers a full room" and an amber pack invites the question "could I combine this with another?". Real onboarding lives in product copy, not architecture.
6. **Rejected alternatives.** A `pack_weights JSONB` column on `rooms` (no schema migration) would have worked but loses FK integrity on each pack id and forces every reader to JSON-decode just to learn what packs the room uses. A pack-level `accepted_payload_versions` declaration was considered and rejected in ADR-016 design notes (would prevent legitimate cross-game-type pack reuse, e.g. image v1 already serving both meme variants). Percentages-summing-to-100 weight UX was rejected in favour of relative integers — adding a pack auto-renormalises, no validation copy, no float drift in the DB.
