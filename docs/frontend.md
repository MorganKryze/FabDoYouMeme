# Frontend

## Technology

| Concern              | Choice                                                    |
| -------------------- | --------------------------------------------------------- |
| Framework            | SvelteKit with `adapter-node` (produces a Node.js server) |
| Reactivity           | Svelte 5 runes (`$state`, `$derived`, `$effect`)          |
| Styling              | Tailwind CSS v4 — utility-first, no custom CSS framework  |
| Component primitives | shadcn-svelte — accessible, unstyled base components      |
| Icons                | lucide-svelte — consistent SVG icons, tree-shaken         |

---

## Route structure

Routes are organized into four layout groups. Each group has its own layout and, where needed, a server-side auth guard.

```plain
src/routes/
  (public)/                    No auth required; minimal layout (no nav bar)
    +page.svelte               Landing page — room code entry
    auth/register/             Invite-based registration
    auth/magic-link/           Request a magic link
    auth/verify/               Intermediate "Log in" button (prevents pre-fetch token consumption)
    privacy/                   Privacy policy

  (app)/                       Auth required; redirects to /auth/magic-link if not logged in
    +page.svelte               App lobby — create or join a room
    rooms/[code]/              Active room view (game plugin rendered here)
    profile/                   Username and email change
    studio/                    Pack and item management (all authenticated users)

  (admin)/                     Auth required + admin role; 403 if not admin
    +page.svelte               Admin dashboard
    users/                     User management
    invites/                   Invite CRUD
    packs/[id]/                Pack item management with moderation controls
    game-types/                Game type registry (read-only)
```

Auth guards live in `+layout.server.ts` files: the `(app)` layout redirects unauthenticated users; the `(admin)` layout returns a 403 if the user lacks the admin role.

---

## State management

Global state lives in `src/lib/state/` as Svelte 5 reactive singleton classes. They are instantiated once and imported directly by components and other modules. No subscription boilerplate — properties declared with `$state` are automatically reactive.

### `user.svelte.ts`

Holds the authenticated user's `id`, `username`, `email`, and `role`. Populated by the `(app)` layout calling `user.setFrom(data.user)` on every navigation, ensuring it stays in sync with server-loaded session data. `isAuthenticated` is a `$derived` property — true when `id` is non-null.

### `ws.svelte.ts`

Manages the WebSocket connection lifecycle. Status is one of `connected`, `reconnecting`, `error`, or `closed`. On disconnect, it retries with exponential backoff and jitter, up to 10 attempts. After 10 failed attempts, `status` transitions to `error` and the user is shown a manual retry option. The top nav bar shows a visual indicator that reflects the current WS status.

### `room.svelte.ts`

Holds the active room's state: `code`, `gameType`, `players`, room `state` (`lobby` / `playing` / `finished`), `currentRound`, `phase` (`idle` / `countdown` / `submitting` / `voting` / `results`), `submissions`, and `leaderboard`. Updated by handling incoming WebSocket messages via `room.handleMessage(msg)`.

### `studio.svelte.ts`

Holds selected pack, item, and version state for the studio editor. Data is loaded on demand — not prefetched. Supports side-by-side version comparison (up to 2 selected version IDs).

### `game-types.svelte.ts`

Caches `GET /api/game-types` for the session. Powers two studio surfaces: the new-pack form's "Used by: meme-showdown, …" hint (`compatibleGameTypeSlugs(kind)`) and the item-table's "X / Y for full room" badge (`worstCaseItemsNeeded(kind)`). The math mirrors the backend's `MinItemsFn` so the badge predicts what `POST /api/rooms` will accept. Loaded lazily via `ensureLoaded()` from `/studio/+page.svelte`.

---

## Server vs client data loading

SvelteKit's `+page.server.ts` and `+layout.server.ts` files run on the server for initial page loads and form actions. They:

- Enforce authentication (redirect if no session)
- Load data that needs to be available for SSR (user profile, room info, pack lists)
- Handle form mutations that should work without JavaScript (progressive enhancement)

Client-side navigation after the initial load uses the typed API client in `src/lib/api/` — fetch wrappers for every REST endpoint, sharing error handling and cookie passing.

---

## Game plugin architecture

The room page (`/rooms/[code]/+page.svelte`) acts as a shell. It receives `game_type_slug` from the room state and dynamically loads the matching game plugin from `src/lib/games/{slug}/`.

Each plugin exports exactly four components:

| Component     | When rendered                                                         |
| ------------- | --------------------------------------------------------------------- |
| `GameRules`   | Before the game starts (lobby phase)                                  |
| `SubmitForm`  | During the submission phase — player writes/selects their answer      |
| `VoteForm`    | During the voting phase — player sees anonymous submissions and votes |
| `ResultsView` | After voting — shows scores and reveals authors                       |

The room shell does not need to know anything about the game's content. Adding a new game type means adding a new folder here — no changes to the room page.

---

## Key UX patterns

**Timer display:** every timed phase receives both `duration_seconds` and `ends_at` from the server. The UI calculates remaining time as `ends_at - Date.now()` on each tick, which avoids clock-drift accumulation. Submit and Vote buttons disable automatically when `Date.now() >= Date.parse(ends_at)`.

**Email non-enumeration:** the magic link request page always displays "If that email is registered, a link is on its way" — never indicates whether the account exists.

**Reconnect banner:** while `ws.status === 'reconnecting'`, the room shows a banner with retry count ("Reconnecting… attempt 3/10"). Once connected, the banner disappears.

**Host disconnect:** if `game_ended` arrives with `reason = "host_disconnected"`, the UI shows a prominent "Host left — game ended" banner above the leaderboard (not a generic disconnect screen).

**Registration consent:** the consent checkbox on the registration form must be explicitly checked before the submit button activates. The form sends `consent: true` only when checked.

---

## Accessibility

- All interactive elements meet a minimum 44×44px touch target
- Form inputs include explicit `<label>` associations
- ARIA roles and `aria-live` regions used for dynamic content (WS status, round phase transitions, toast notifications)
- Keyboard navigation supported throughout — focus management on route transitions
- Svelte's default HTML escaping prevents XSS; nonce-based CSP provides defence-in-depth

---

## Content Security Policy

The frontend uses nonce-based CSP configured in `src/hooks.server.ts`. A fresh nonce is generated per request and injected into all inline scripts and styles. The baseline policy:

- `default-src 'self'`
- `script-src 'self' 'nonce-{n}'`
- `style-src 'self' 'nonce-{n}'`
- `img-src 'self' data: blob:`
- `connect-src 'self' wss: ws:`
- `frame-ancestors 'none'`

In production, `ws:` should be replaced with `wss:` only.

---

## Internationalization

The catalog lives at `frontend/messages/{en,fr}.json`. [Paraglide JS](https://inlang.com/m/gerre34r/library-inlang-paraglideJs) compiles every key into a typed function in `src/lib/paraglide/messages/`.

Usage from a component:

```ts
import * as m from '$lib/paraglide/messages';

<button>{m.common_save()}</button>
<p>{m.home_welcome_named({ username })}</p>
```

Key naming: domain-prefixed flat snake_case. Reserved prefixes:

| Prefix         | Scope                                                          |
| -------------- | -------------------------------------------------------------- |
| `common_*`     | Buttons shared across pages (save, cancel, delete, confirm)    |
| `auth_*`       | Register, login, verify, consent                               |
| `home_*`       | Landing/dashboard                                              |
| `host_*`       | Create-room flow                                               |
| `room_*`       | Lobby, waiting stage, kick/ban dialogs                         |
| `game_*`       | Shared game UI (timer, submissions, votes, results)            |
| `game_<slug>_*`| Per-game-type strings (e.g. `game_meme_showdown_prompt`)       |
| `profile_*`    | User settings                                                  |
| `admin_*`      | Admin area                                                     |
| `errors_<code>`| User-facing error messages, one per `e_*` code                 |
| `toast_*`      | Flash messages                                                 |
| `nav_*`        | Top-bar/user-menu labels                                       |

Adding a string: add the key + EN value to `messages/en.json`, add the translated FR value to `messages/fr.json` under the same key, then use `m.the_key()` at the call site. CI (`npm run i18n:check`) enforces parity and rejects `[FR]` placeholders in either file — every key must have a real FR translation before merge. See `docs/brand.md § French voice` for the FR voice rules (tu/vous register, typography, loanword handling).

Locale resolution priority (in `hooks.server.ts`): authenticated `user.locale` → `PARAGLIDE_LOCALE` cookie → `Accept-Language` header → `PUBLIC_DEFAULT_LOCALE`.

Adding a new locale: (1) migration extending every CHECK constraint in `users.locale`, `game_packs.language`, `invites.locale`; (2) add the code to `SUPPORTED_LOCALES` in `src/lib/i18n/locale.ts`; (3) add the code to `project.inlang/settings.json` `languageTags`; (4) add `messages/<code>.json`; (5) add `backend/internal/email/templates/<code>/` with full template parity (startup fails otherwise).
