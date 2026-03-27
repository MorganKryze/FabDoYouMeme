# 07 — Frontend

## Technology

| Concern              | Choice                     | Notes                                                 |
| -------------------- | -------------------------- | ----------------------------------------------------- |
| Framework            | SvelteKit (`adapter-node`) | File-based routing, SSR, hooks for auth               |
| Reactivity           | Svelte 5 runes             | `$state`, `$derived`, `$effect` in `.svelte.ts` files |
| Styling              | Tailwind CSS v4            | Utility-first; no custom CSS framework                |
| Component primitives | shadcn-svelte              | Svelte 5-compatible; accessible, unstyled base        |
| Icons                | lucide-svelte              | Consistent, tree-shaken SVG icons                     |

---

## State Architecture

Global state lives in `src/lib/state/` as reactive Svelte 5 classes (not stores). They are instantiated once and imported where needed.

### `ws.svelte.ts` — WebSocket connection

```ts
class WsState {
  status: 'connected' | 'reconnecting' | 'error' | 'closed' = $state('closed');
  retryCount = $state(0);
  // ...connect(), disconnect(), send(), onMessage()
  // Exponential backoff reconnect: 1s, 2s, 4s, 8s… capped at 30s, up to 10 retries
  // Sends client-side ping every 25s; expects pong within 10s
}
export const ws = new WsState();
```

### `room.svelte.ts` — Room & game state

```ts
class RoomState {
  code = $state<string | null>(null);
  gameType = $state<GameType | null>(null);
  players = $state<Player[]>([]);
  state = $state<'lobby' | 'playing' | 'finished'>('lobby');
  currentRound = $state<Round | null>(null);
  phase = $state<'idle' | 'countdown' | 'submitting' | 'voting' | 'results'>('idle');
  submissions = $state<Submission[]>([]);
  leaderboard = $state<LeaderboardEntry[]>([]);
  // Handles all incoming WS messages and mutates state accordingly
  handleMessage(msg: WsMessage): void { ... }
}
export const room = new RoomState();
```

### `user.svelte.ts` — Current authenticated user

```ts
class UserState {
  id = $state<string | null>(null);
  username = $state<string | null>(null);
  email = $state<string | null>(null);
  role = $state<'player' | 'admin' | null>(null);
  isAuthenticated = $derived(() => this.id !== null);
}
export const user = new UserState();
```

---

## Route Structure

Route groups are used to share layouts without polluting URLs.

```plain
src/routes/
  (public)/                         ← minimal layout: no nav bar, centered content
    +layout.svelte
    +page.svelte                    ← Landing page
    auth/
      register/
        +page.svelte                ← Invite-based registration
      magic-link/
        +page.svelte                ← Request magic link
      verify/
        +page.svelte                ← Intermediate "Log in" button

  (app)/                            ← authenticated layout: top nav + connection indicator
    +layout.svelte
    +layout.server.ts               ← session guard → redirect to /auth/magic-link
    +page.svelte                    ← App lobby (create room or enter code)
    rooms/
      [code]/
        +layout.svelte              ← Room chrome: code banner + player panel
        +layout.server.ts           ← verify player is in the room
        +page.svelte                ← Game view (dispatches to game type plugin)
    profile/
      +page.svelte                  ← Username change, email change request

  (admin)/                          ← admin layout: sidebar navigation
    +layout.svelte
    +layout.server.ts               ← admin role guard → 403 if not admin
    +page.svelte                    ← Admin dashboard (stats overview)
    users/
      +page.svelte                  ← User management table
    invites/
      +page.svelte                  ← Invite CRUD
    packs/
      +page.svelte                  ← Pack list
      [id]/
        +page.svelte                ← Pack item manager
    game-types/
      +page.svelte                  ← Game type registry (read-only)
```

---

## Pages & UX Flows

### Landing Page `/` (public)

**Goal**: get a player into a game as fast as possible from any device.

Layout:

- Vertically and horizontally centered, full-screen
- App logo / wordmark at top
- Large 4-character room code input (auto-caps, numeric keyboard hint on mobile)
- "Join Game" primary button — if not authenticated, redirects to `/auth/magic-link?next=/rooms/{code}`
- "I'm hosting →" secondary text link below → navigates to `/` (app lobby) after auth

Mobile considerations:

- Touch target minimum 44×44px for all interactive elements
- Input auto-focuses on page load
- Soft keyboard does not obscure the button (input + button rendered in upper 60% of viewport)

---

### Auth: Registration `/auth/register`

Flow: user arrives with `?invite=TOKEN` in URL (pre-filled from invite link).

Fields:

- Invite token (pre-filled, editable if needed)
- Username (3–30 chars, alphanumeric + underscore)
- Email

On submit: `POST /api/auth/register`. On success, show "Check your email" message if `restricted_email` invite was used (magic link auto-sent), otherwise redirect to `/auth/magic-link`.

---

### Auth: Magic Link Request `/auth/magic-link`

Single field: email address. Submit → `POST /api/auth/magic-link`. Always shows "If that email is registered, a link is on its way." (no enumeration).

---

### Auth: Verify `/auth/verify?token=xxx`

**Not** a form — it's an intermediate confirmation page to prevent email pre-fetch consuming the token.

Shows: "Welcome back. Click below to log in." + "Log In" button.

On click: `POST /api/auth/verify { token }`. On success, redirect to `?next=` param or `/`. On error (expired/used), show "This link has expired. Request a new one →".

---

### App Lobby `/` (app)

Two actions side by side (or stacked on mobile):

**Create Room** (card):

- Select game type (dropdown with icons + descriptions)
- Select pack (dropdown, filtered by compatibility)
- Mode toggle: Multiplayer / Solo (solo hidden if `game_type.supports_solo = false`)
- Config: round count (slider 1–50), round duration (slider 15–300s), voting duration (slider 10–120s)
- "Create Room" button → `POST /api/rooms` → redirect to `/rooms/{code}`

**Join Room** (card):

- Room code input (4 chars)
- "Join" button → navigate to `/rooms/{code}`

---

### Room — Lobby Phase `/rooms/[code]`

```plain
┌─────────────────────────────────────────────────┐
│  Room Code: WXYZ  [Copy]   🎮 Meme Caption  📦 Pack Name  │
├────────────────────────────────┬────────────────┤
│                                │  Players (3)   │
│   Waiting for host to start…   │  ● Alice (host)│
│                                │  ● Bob         │
│   [game type icon + rules btn] │  ● Carol       │
│                                │                │
│                                │ [Kick] per row │
│                                │  (host only)   │
└────────────────────────────────┴────────────────┘
│  HOST ONLY:  [⚙ Config]  [Start Game ▶]  (enabled ≥2 players)  │
```

- Room code displayed prominently in monospace with a copy-to-clipboard button
- Player list shows joined order, host badge, online/reconnecting indicator
- Host sees: config gear (inline expand showing sliders), Start Game button
- Non-host sees: "Waiting for [host] to start…" with subtle animation
- "How to play?" link → opens `GameRules` modal

**Config panel (host only, inline below player list)**:

- Round count, round duration, voting duration sliders with live value labels
- Changes auto-save via `PATCH /api/rooms/:code/config` with debounce

---

### Room — Countdown Phase

Triggered by `game_started` event.

- Full-screen overlay: large countdown `3 → 2 → 1 → GO!`
- Each number animates in/out (scale + fade, 1 second each)
- Background dims during countdown

---

### Room — Submission Phase

Triggered by `round_started` event.

```plain
┌──────────────────────────────────────────────────┐
│  [████████████████░░░░░░░]  42s               Round 3/10  │
├──────────────────────────────────────────────────┤
│                                                  │
│   [meme image here — full width, 16:9 cropped]  │
│                                                  │
│   Prompt: "When the CI passes on the first try"  │
│                                                  │
│   ┌──────────────────────────────────────────┐   │
│   │  Write your caption…                     │   │
│   └──────────────────────────────────────────┘   │
│                    [Submit]                      │
├──────────────────────────────────────────────────┤
│  ● Alice ✓  ● Bob ✓  ● Carol ⏳  ● Dave ⏳      │
└──────────────────────────────────────────────────┘
```

- Timer bar at top: animated progress bar + numeric countdown
- Game content (image + prompt) rendered by `SubmitForm` plugin component
- Player status pills at bottom: submitted (✓) vs pending (⏳) — no names visible for pending if game type hides them
- After submission: input locked, button becomes "Submitted ✓", player can still see timer

---

### Room — Voting Phase

Triggered by `submissions_closed` + `meme-caption:submissions_shown` events.

```plain
┌──────────────────────────────────────────────────┐
│  [████████░░░░░░░░░░░░░░]  18s             Voting  │
├──────────────────────────────────────────────────┤
│   [meme image — smaller, above captions]         │
│                                                  │
│   ┌──────────────┐  ┌──────────────┐             │
│   │ "Caption A"  │  │ "Caption B"  │             │
│   └──────────────┘  └──────────────┘             │
│   ┌──────────────┐  ┌──────────────┐             │
│   │ "Caption C"  │  │ "Caption D"  │             │
│   └──────────────┘  └──────────────┘             │
│                   [Vote ▶]                       │
└──────────────────────────────────────────────────┘
```

- Rendered by `VoteForm` plugin component
- Author names hidden; own caption visually distinguished (e.g., subtle outline) but still voteable by others
- Tap/click to select; selected card highlights; "Vote" button confirms
- After voting: card locked, voted card highlighted, wait for results

---

### Room — Results Phase

Triggered by `meme-caption:vote_results` event.

Animated reveal sequence:

1. All submission cards appear stacked
2. Each card flips to reveal author name (300ms stagger)
3. Vote count appears with a +N badge animation per card
4. Points row below each card: `+N pts`
5. Round leaderboard slides in from bottom: `1. Alice +3  2. Bob +1  …`
6. Host: "Next Round →" button (or "End Game" on final round)
7. Non-host: "Waiting for host…"

---

### Room — Game Ended

Triggered by `game_ended` event.

```plain
┌──────────────────────────────────────────────────┐
│              🏆  Game Over                       │
│                                                  │
│   🥇 Alice   47 pts                             │
│   🥈 Bob     38 pts                             │
│   🥉 Carol   29 pts                             │
│      Dave    22 pts                             │
│                                                  │
│   [Play Again]    [Back to Lobby]               │
└──────────────────────────────────────────────────┘
```

- If `reason = "host_disconnected"` or `"all_players_disconnected"`: banner above leaderboard explaining why the game ended early
- "Play Again" → `POST /api/rooms` with same pack + game type → redirect to new room
- "Back to Lobby" → navigate to `/`

---

### Profile `/profile`

Two sections:

**Username**: current username displayed, "Edit" inline → text input → "Save" / "Cancel". `PATCH /api/users/me { username }`.

**Email**: current email displayed. "Change Email" → inline form: new email input → "Send Verification". `PATCH /api/users/me { email }` → shows "Check your new email for a verification link."

---

### Admin Dashboard `/admin`

Stats cards row:

- Active rooms (count of rooms in `lobby` or `playing`)
- Total users
- Total packs
- Pending invites

Recent activity list (last 10 audit log entries): `Alice's role changed to admin by Bob · 2 hours ago`.

---

### Admin: Users `/admin/users`

Table columns: Username, Email, Role, Active, Joined, Actions.

- **Role toggle**: inline dropdown `player ↔ admin`. On change: `PATCH /api/admin/users/:id { role }` + audit log entry.
- **Deactivate/Reactivate**: toggle switch in Active column. On change: `PATCH /api/admin/users/:id { is_active }`.
- **Change Email**: "Edit" icon → inline input → Save. `PATCH /api/admin/users/:id { email }`.
- **Change Username**: "Edit" icon → inline input → Save.

Pagination: 50 rows per page. Search/filter by username or email.

---

### Admin: Invites `/admin/invites`

**Create Invite** button (top right) → slide-over panel:

- Label (optional)
- Restricted email (optional)
- Max uses (number input; 0 = unlimited)
- Expiry (date picker; leave blank = never)
- "Create" → `POST /api/admin/invites` → new row appears at top

Table columns: Label, Token (masked `XXXX…`, reveal on hover), Restricted Email, Uses (N/max), Expires, Created By, Actions.

- **Revoke**: trash icon → confirm dialog → `DELETE /api/admin/invites/:id`.
- **Copy Link**: copies `{FRONTEND_URL}/auth/register?invite={token}` to clipboard.

---

### Admin: Packs `/admin/packs`

Table: Name, Description, Items, Created By, Created, Actions.

- "New Pack" → inline form row at top of table: name + description → "Create" → `POST /api/packs`.
- Pack row click → navigate to `/admin/packs/:id`.
- Delete: `DELETE /api/packs/:id` → confirmation dialog (warns that in-use packs cannot be deleted while rooms reference them).

---

### Admin: Pack Items `/admin/packs/[id]`

Header: pack name + description (editable inline) + item count.

Item table:

- Columns: Position (drag handle), Thumbnail, Payload Fields, Version, Actions
- **Add Item** button → modal:
  - Drag-and-drop image zone (or click to browse) with client-side preview
  - Payload field inputs (game-type-agnostic key/value pairs, shown as a JSON editor for now)
  - "Upload & Add" → step 1: `POST /api/packs/:id/items`, step 2: `POST /api/assets/upload-url`, step 3: PUT to Rustfs, step 4: `PATCH item { media_key }`
- **Reorder**: drag rows to new position → batch `PATCH` to update `position` values
- **Delete**: trash icon per row → `DELETE /api/packs/:id/items/:item_id`

---

### Admin: Game Types `/admin/game-types`

Read-only list showing slug, name, description, version, supports_solo, and the config JSON (min/max/defaults). No editing — game types are seeded via migrations.

---

## Connection Status Indicator

Visible in the top-right of the app layout nav bar on all `(app)` routes:

| State          | Display                                            |
| -------------- | -------------------------------------------------- |
| `connected`    | Small green dot (hidden unless hovered)            |
| `reconnecting` | Amber pulsing dot + "Reconnecting…" tooltip        |
| `error`        | Red dot + "Connection lost" tooltip + "Retry" link |

Additionally, while `reconnecting`, a dismissible banner appears at the top of the room page:

> **Connection lost.** Reconnecting… (attempt 3 / 10) — Your progress is saved.

---

## Timer Display

The `round_started` and `submissions_closed` WS events include an `ends_at` ISO 8601 timestamp (server clock). The client countdown must use this absolute deadline rather than counting down from a local start time to avoid clock drift issues.

```ts
// room.svelte.ts
function startTimer(endsAt: string) {
  const deadline = Date.parse(endsAt);
  const tick = () => {
    const remaining = Math.max(0, deadline - Date.now());
    timerMs = remaining;
    if (remaining > 0) requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
}
```

The `Submit` button is disabled client-side when `Date.now() >= deadline`, matching the server's enforcement window. This eliminates the confusing "too late" error that would occur if the client timer ran slightly longer than the server's.

---

## Accessibility

All interactive elements must meet WCAG 2.1 AA as a baseline. The following are the most impactful requirements for this type of app:

### Touch & Click Targets

- Minimum 44×44px for all interactive elements (buttons, links, toggles)
- Player pills in the room view must not be smaller than 44px tall if they are tappable
- Room code input: tall enough for easy mobile tap (min 56px height)

### Keyboard Navigation

- All interactive elements reachable via `Tab` in logical DOM order
- Modal dialogs: focus trapped inside while open; `Escape` closes; focus returns to trigger on close
- Game voting cards: selectable via `Enter` or `Space`; arrow keys move focus between cards
- Submit/Vote buttons: accessible via `Enter` key

### ARIA

- Icon-only buttons (copy code, kick player, close modal) must have `aria-label`
- Connection status dot: `role="status"` with `aria-label="Connection: reconnecting"` etc.
- Timer bar: `role="progressbar"` with `aria-valuenow`, `aria-valuemin="0"`, `aria-valuemax={duration_seconds}`
- Live player status pills: wrap in `aria-live="polite"` region so screen readers announce when a player submits
- Phase transitions: announce to screen readers via a visually-hidden `aria-live="assertive"` element (e.g., "Round 3 started. You have 60 seconds to submit.")

### Color Independence

Color must never be the sole indicator of state:

| Element               | Color                | Supplementary indicator          |
| --------------------- | -------------------- | -------------------------------- |
| Connection dot        | green / amber / red  | Tooltip text + icon change       |
| Player submitted pill | gray → green         | ✓ checkmark added                |
| Selected voting card  | highlighted border   | ✓ checkmark overlay              |
| Timer bar             | green → yellow → red | Numeric countdown always visible |

### Motion

Respect `prefers-reduced-motion`:

- Round countdown (3-2-1): use opacity fade instead of scale animation
- Results reveal (card flip, +N badge): collapse to instant appear
- Phase transition overlays: use `opacity` transitions at 100ms max instead of slide/scale

```css
@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

### Focus Management

After phase transitions triggered by WebSocket events, move focus programmatically:

- `round_started` → focus the caption input (SubmitForm)
- `submissions_closed` / `submissions_shown` → focus the first voting card
- `vote_results` → focus the results heading
- `game_ended` → focus the "Play Again" button

---

## Navigation Summary

| Route           | Available to                | Nav element           |
| --------------- | --------------------------- | --------------------- |
| `/`             | Public (redirect if authed) | —                     |
| `/auth/*`       | Public                      | —                     |
| `/` (app)       | Authenticated               | Top nav               |
| `/rooms/[code]` | Authenticated + in room     | Top nav + room chrome |
| `/profile`      | Authenticated               | Top nav dropdown      |
| `/admin/*`      | Admin role                  | Sidebar + top nav     |

Top nav (app layout): logo left, `[Profile ▼] [Logout]` right, connection dot far right.

Admin sidebar: Users · Invites · Packs · Game Types · `← Back to App` at bottom.
