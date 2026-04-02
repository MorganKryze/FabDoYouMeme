# 05 — Frontend

SvelteKit routing, state architecture, UX flows, accessibility, and error handling.

---

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

Global state lives in `src/lib/state/` as reactive Svelte 5 classes (not stores). Instantiated once, imported where needed.

### `ws.svelte.ts` — WebSocket connection

```ts
class WsState {
  status: 'connected' | 'reconnecting' | 'error' | 'closed' = $state('closed');
  retryCount = $state(0);
  // connect(), disconnect(), send(), onMessage()
  //
  // Reconnect with exponential backoff + jitter:
  //   delay = min(30, 2^(retryCount - 1)) + random(0, 1) seconds
  //   Up to 10 retries; status → 'error' after 10 failures
  //
  // Client-side ping sent every WS_PING_INTERVAL (default 25s); expects pong within 10s
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

### `studio.svelte.ts` — Studio content creation state

```ts
class StudioState {
  selectedPackId = $state<string | null>(null);
  selectedItemId = $state<string | null>(null);
  selectedVersionIds = $state<string[]>([]); // up to 2, for side-by-side comparison

  // Loaded on demand — not prefetched
  packs = $state<Pack[]>([]);
  items = $state<Item[]>([]);
  versions = $state<ItemVersion[]>([]);

  // selectPack(), selectItem(), saveVersion(), restoreVersion(), moveVersionToBin()
}
export const studio = new StudioState();
```

---

## Route Structure

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
    +page.svelte                    ← Admin dashboard (stats overview + notification badge)
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

  (app)/studio/                     ← authenticated layout (all users, not admin-only)
    +layout.svelte
    +layout.server.ts               ← session guard → redirect to /auth/magic-link
    +page.svelte                    ← Studio: pack navigator + item table + item editor
```

---

## Pages & UX Flows

### Landing Page `/` (public)

- Full-screen centered layout; app logo at top
- Large 4-character room code input (auto-caps, numeric keyboard hint on mobile)
- "Join Game" primary button — if not authenticated, redirects to `/auth/magic-link?next=/rooms/{code}`
- "I'm hosting →" secondary text link → navigates to `/` (app lobby) after auth

Mobile: touch targets minimum 44×44px; input auto-focuses on load; input + button in upper 60% of viewport so soft keyboard doesn't obscure them.

---

### Auth: Registration `/auth/register`

Flow: user arrives with `?invite=TOKEN` in URL (pre-filled from invite link).

Fields: invite token (pre-filled, editable), username (3–30 chars, alphanumeric + underscore), email.

On submit: `POST /api/auth/register`. On success: if `restricted_email` invite was used, show "Check your email for your login link"; otherwise redirect to `/auth/magic-link`. On `smtp_failure` warning: show a warning toast "Account created but login email couldn't be sent. Ask your admin to resend."

---

### Auth: Magic Link Request `/auth/magic-link`

Single field: email address. Submit → `POST /api/auth/magic-link`. Always shows "If that email is registered, a link is on its way." (no enumeration, regardless of response).

---

### Auth: Verify `/auth/verify?token=xxx`

Intermediate confirmation page — prevents email pre-fetch consuming the token.

Shows: "Welcome back. Click below to log in." + "Log In" button.

On click: `POST /api/auth/verify { token }`. On success: redirect to `?next=` param or `/`. On error (expired/used/not found): show "This link has expired. Request a new one →".

---

### App Lobby `/` (app)

Two actions side by side (stacked on mobile):

**Create Room** card:

- Select game type (dropdown with icons + descriptions)
- Select pack — populated from `GET /api/packs?game_type_id={selected_game_type_id}` (filtered by compatibility). Re-fetched whenever the selected game type changes. If no packs are returned: show inline notice "No compatible packs found for this game type."
- Mode toggle: Multiplayer / Solo (solo option hidden if `game_type.supports_solo = false`)
- Config: round count (slider 1–50), round duration (slider 15–300s), voting duration (slider 10–120s)
- "Create Room" → `POST /api/rooms` → redirect to `/rooms/{code}`

**Join Room** card:

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

- Room code displayed in monospace with copy-to-clipboard button
- Host sees: config gear (inline expand → sliders), Start Game button
- Non-host sees: "Waiting for [host] to start…" with subtle animation

**Config panel (host only)**:

- Round count, round duration, voting duration sliders with live value labels
- Changes auto-save via debounced `PATCH /api/rooms/:code/config`
- **On PATCH failure**: revert the slider to its last confirmed server value (not the in-flight value) and show an error toast. The displayed value must always reflect the last successfully saved state.

---

### Room — Countdown Phase

Triggered by `game_started` event.

- Full-screen overlay: large countdown `3 → 2 → 1 → GO!`
- Each number animates in/out (scale + fade, 1s each; collapses to opacity-only with `prefers-reduced-motion`)

---

### Room — Submission Phase

Triggered by `round_started` event.

```plain
┌──────────────────────────────────────────────────┐
│  [████████████████░░░░░░░]  42s               Round 3/10  │
├──────────────────────────────────────────────────┤
│   [meme image — full width, 16:9 cropped]        │
│   Prompt: "When the CI passes on the first try"  │
│   ┌──────────────────────────────────────────┐   │
│   │  Write your caption…                     │   │
│   └──────────────────────────────────────────┘   │
│                    [Submit]                      │
├──────────────────────────────────────────────────┤
│  ● Alice ✓  ● Bob ✓  ● Carol ⏳  ● Dave ⏳      │
└──────────────────────────────────────────────────┘
```

- Timer bar: animated progress bar + numeric countdown using `ends_at` absolute deadline (see Timer Display)
- Game content rendered by `SubmitForm` plugin component
- Player status pills at bottom: submitted (✓) vs pending (⏳)
- After submission: input locked, button becomes "Submitted ✓"
- **Timer note**: the Submit button disabled state is a UX convenience — the server enforces the timer via `duration_seconds` and will reject late submissions regardless

---

### Room — Voting Phase

Triggered by `submissions_closed` + `{slug}:submissions_shown` events.

- Rendered by `VoteForm` plugin component
- Author names hidden; own caption is visually distinguished:
  - **2px muted-color outline** (`text-muted-foreground` color token)
  - **"You" badge**: top-right corner of card, pill shape, `text-xs`, `bg-muted text-muted-foreground`
  - Informational only — the badge must NOT be visually prominent (not a recommendation to vote for it)
- Tap/click to select; selected card highlights with `✓` overlay; "Vote" button confirms
- After voting: card locked, voted card remains highlighted, wait for results

---

### Room — Results Phase

Triggered by `{slug}:vote_results` event.

Animated reveal sequence:

1. All submission cards appear stacked
2. Each card flips to reveal author name (300ms stagger; collapses to instant appear with `prefers-reduced-motion`)
3. Vote count appears with `+N` badge animation per card
4. Points row below each card: `+N pts`
5. Round leaderboard slides in from bottom
6. Host: "Next Round →" button (or "End Game" on final round); non-host: "Waiting for host…"

---

### Room — Game Ended

Triggered by `game_ended` event.

```plain
┌──────────────────────────────────────────────────┐
│              🏆  Game Over                       │
│   🥇 Alice   47 pts                             │
│   🥈 Bob     38 pts                             │
│   🥉 Carol   29 pts                             │
│      Dave    22 pts                             │
│   [Play Again]    [Back to Lobby]               │
└──────────────────────────────────────────────────┘
```

**Disconnect banner rules**:

- `reason = "host_disconnected"`: show prominent banner **above** the leaderboard — "Host left — game ended". Do NOT use a generic disconnect screen.
- `reason = "all_players_disconnected"`: show banner "All players disconnected — game ended."
- `reason = "pack_exhausted"`: show banner "Ran out of compatible items — game ended early."
- `reason = "completed"`: no banner.

"Play Again" → `POST /api/rooms` with same pack + game type → redirect to new room.

---

### Profile `/profile`

**Username**: current username displayed → "Edit" → text input → "Save" / "Cancel". `PATCH /api/users/me { username }`. On `409 username_taken`: show inline error "That username is taken."

**Email**: current email displayed → "Change Email" → new email input → "Send Verification". `PATCH /api/users/me { email }` → success toast "Check your new email for a verification link." The old email remains active for login until the user clicks the verification link.

---

### Studio `/studio`

Three-panel layout. All authenticated users can access. Content and actions shown depend on pack ownership and role.

```plain
┌──────────────────┬───────────────────────────┬────────────────────────────┐
│  Pack Navigator  │       Item Table           │       Item Editor          │
│                  │                            │                            │
│  ▸ Official      │  #  │ Preview │ Name │ Ver │  [Image mode / Text mode]  │
│    V1  (42 items)│  1  │  🖼     │ ...  │  3  │                            │
│    V2  (38 items)│  2  │  🖼     │ ...  │  1  │  Image: crop + draw tools  │
│    V3  (12 items)│  ...                       │  Text: extensible textarea │
│                  │                            │                            │
│  ▸ Public        │  [+ New Item]              │  [Save → new version]      │
│    user packs…   │  [Bulk Import]             │                            │
│                  │                            │  ── Version History ───    │
│  ▸ Private       │                            │  v3 · 2026-04-02 ← active  │
│    my packs…     │                            │  v2 · 2026-03-28           │
│                  │                            │  v1 · 2026-03-01           │
│  [+ New Pack]    │                            │  [Restore] [Move to Bin]   │
│                  │                            │                            │
│  ── Admin only ──│                            │  [Compare v1 vs v3]        │
│  Moderation (3)  │                            │                            │
└──────────────────┴───────────────────────────┴────────────────────────────┘
```

**Pack Navigator** (left panel):

- Groups: Official (V1, V2, V3…), Public (other users' public packs the current user follows), Private (own packs)
- "New Pack" button at the bottom: inline name + description form → `POST /api/packs`
- Admin-only "Moderation" tab: lists packs with `status = 'flagged'`; shows pack name, owner, date flagged; actions: "Ban Pack" (`PATCH status=banned`), "Clear Flag" (`PATCH status=active`)

**Item Table** (center panel):

- Columns: drag handle (reorder), thumbnail or text preview, item name, type badge (Image / Text), version number, last modified, delete action
- **Bulk Import** (image packs only): drag-and-drop zone for multiple files → runs the 4-step upload flow for each file in sequence (POST item → POST upload-url → PUT → PATCH confirm)
- Drag reorder → batch `PATCH /api/packs/:id/items/reorder`

**Item Editor** (right panel):

- Switches between Image and Text mode via an editor type indicator (not a toggle — mode is set at item creation and cannot change)
- **Image mode**: upload/replace area + crop tool (aspect-ratio constrained, Canvas API) + freehand draw layer; "Save" creates a new `game_item_versions` row and updates `current_version_id`
- **Text mode**: extensible `TextEditor` wrapper (accepts slots/plugins for future rich text; currently renders a textarea with character count); "Save" creates a new version row
- **Version History drawer** (collapsible, bottom of editor):
  - Timeline: version number, timestamp, "active" badge on current version
  - Per-entry actions: "Restore" (sets `current_version_id`), "Move to Bin" (soft delete, `deleted_at = now()`)
  - Select two versions → "Compare" button → side-by-side view (image diff or text diff)
  - Binned versions shown with muted style and "Restoring from bin is not possible" note if `deleted_at` is set but purge has not run

**Component locations**:

```plain
src/lib/components/studio/
  PackNavigator.svelte
  ItemTable.svelte
  ItemEditor.svelte
  ImageEditor.svelte       ← Canvas-based crop + draw (no external image-edit lib)
  TextEditor.svelte        ← Extensible textarea wrapper
  VersionHistory.svelte    ← Collapsible timeline + comparison
src/lib/api/studio.ts      ← Typed fetch wrappers for all studio endpoints
```

---

### Admin Dashboard `/admin`

Stats cards row: active rooms, total users, total packs, pending invites.

Recent activity list (last 10 audit log entries): `Alice's role changed to admin by Bob · 2 hours ago`.

**Notification badge**: if there are unread `admin_notifications`, a badge count appears on the admin nav bell icon (top-right of admin layout) and on the "Moderation" tab in the Studio Pack Navigator.

---

### Admin: Users `/admin/users`

Table columns: Username, Email, Role, Active, Joined, Actions.

- **Role toggle**: inline dropdown `player ↔ admin`. `PATCH /api/admin/users/:id { role }`.
- **Deactivate/Reactivate**: toggle switch. `PATCH /api/admin/users/:id { is_active }`.
- **Change Email**: "Edit" icon → inline input → Save. `PATCH /api/admin/users/:id { email }`.
- **Change Username**: "Edit" icon → inline input → Save. Returns `409` on conflict.
- **Delete**: trash icon → confirm dialog: "This permanently removes all personal data." → `DELETE /api/admin/users/:id`.

Pagination: 50 rows per page. **Search**: text input → `GET /api/admin/users?q={term}` (debounced 300ms, case-insensitive substring match). Clears cursor when a new search term is entered.

---

### Admin: Invites `/admin/invites`

**Create Invite** button → slide-over panel: label, restricted email, max uses (0 = unlimited), expiry date. `POST /api/admin/invites`.

Table: Label, Token (masked `XXXX…`, reveal on hover), Restricted Email, Uses (N/max), Expires, Actions.

- **Revoke**: trash icon → confirm → `DELETE /api/admin/invites/:id`.
- **Copy Link**: copies `{FRONTEND_URL}/auth/register?invite={token}` to clipboard → success toast.

---

### Admin: Packs `/admin/packs`

Table: Name, Description, Items, Created By, Created, Actions.

- "New Pack" → inline form row at top: name + description → "Create" → `POST /api/packs`.
- Pack row click → `/admin/packs/:id`.
- Delete: `DELETE /api/packs/:id` → confirm dialog (warns that in-use packs will no longer be available for new rooms).

---

### Admin: Pack Items `/admin/packs/[id]`

Header: pack name + description (editable inline) + item count.

Item table columns: Position (drag handle), Thumbnail, Payload Fields, Version, Actions.

- **Add Item** → modal: drag-and-drop image zone + client-side preview (`URL.createObjectURL(file)`) + payload key/value editor. "Upload & Add" runs the 4-step flow: `POST items` → `POST upload-url` → PUT to RustFS → `PATCH item { media_key }`.
- **Reorder**: drag rows → batch `PATCH /api/packs/:id/items/reorder`.
- **Delete**: trash icon per row → `DELETE /api/packs/:id/items/:item_id`.

---

### Admin: Game Types `/admin/game-types`

Read-only list: slug, name, description, version, supports_solo, config JSON (min/max/defaults), supported_payload_versions. No editing — game types are seeded via migrations.

---

## Toast Notifications

A global toast system is available in the app layout, invoked as `toast.show(message, type)` from anywhere.

| Type      | Background               | Duration        | Dismissal                  |
| --------- | ------------------------ | --------------- | -------------------------- |
| `success` | Green (`bg-green-600`)   | 3s auto-dismiss | Also dismissible by click  |
| `warning` | Yellow (`bg-yellow-500`) | 5s auto-dismiss | Also dismissible by click  |
| `error`   | Red (`bg-red-600`)       | Persistent      | Must be manually dismissed |

Stack: up to 3 toasts visible simultaneously. Oldest pushed off when limit exceeded. Position: bottom-right, stacked upward. Z-index: above all other UI elements.

**Toast triggers**:

- Config PATCH failure → error toast
- Successful username save → success toast
- Email change link sent → success toast
- Copy invite link → success toast
- `smtp_failure` warning on registration → warning toast
- `pack_no_supported_items` or `pack_insufficient_items` on room creation → error toast with descriptive message

---

## Connection Status Indicator

Visible in the top-right of the app layout nav bar on all `(app)` routes:

| State          | Display                                            |
| -------------- | -------------------------------------------------- |
| `connected`    | Small green dot (hidden unless hovered)            |
| `reconnecting` | Amber pulsing dot + "Reconnecting…" tooltip        |
| `error`        | Red dot + "Connection lost" tooltip + "Retry" link |

While `reconnecting`, a dismissible banner appears at the top of the room page:

> **Connection lost.** Reconnecting… (attempt 3 / 10) — Your progress is saved.

---

## Timer Display

`round_started` and `submissions_closed` WS events include an `ends_at` ISO 8601 timestamp (server clock). The client countdown must use this absolute deadline — not a local start time — to avoid clock drift.

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

The `Submit` button is disabled client-side when `Date.now() >= deadline` (UX convenience only — the server enforces via `duration_seconds`).

---

## Accessibility

All interactive elements must meet WCAG 2.1 AA as a baseline.

### Touch & Click Targets

- Minimum 44×44px for all interactive elements
- Room code input: minimum 56px height
- Player pills: minimum 44px tall if tappable

### Keyboard Navigation

- All interactive elements reachable via `Tab` in logical DOM order
- Modal dialogs: focus trapped inside while open; `Escape` closes; focus returns to trigger on close
- Voting cards: selectable via `Enter` or `Space`; arrow keys move focus between cards

### ARIA

- Icon-only buttons (copy code, kick, close modal): `aria-label`
- Connection status dot: `role="status"` with `aria-label="Connection: reconnecting"` etc.
- Timer bar: `role="progressbar"` with `aria-valuenow`, `aria-valuemin="0"`, `aria-valuemax={duration_seconds}`
- Live player status pills: wrap in `aria-live="polite"` region
- Phase transitions: announce via visually-hidden `aria-live="assertive"` element (e.g., "Round 3 started. You have 60 seconds to submit.")

### Color Independence

| Element               | Color                | Supplementary indicator          |
| --------------------- | -------------------- | -------------------------------- |
| Connection dot        | green / amber / red  | Tooltip text + icon change       |
| Player submitted pill | gray → green         | ✓ checkmark added                |
| Selected voting card  | highlighted border   | ✓ checkmark overlay              |
| Timer bar             | green → yellow → red | Numeric countdown always visible |

### Motion

Respect `prefers-reduced-motion`:

- Round countdown: opacity fade instead of scale animation
- Results reveal (card flip, `+N` badge): collapse to instant appear
- Phase transition overlays: `opacity` transitions at 100ms max

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

- `round_started` → focus the caption input (`SubmitForm`)
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
| `/studio`       | Authenticated (all users)   | Top nav               |
| `/admin/*`      | Admin role                  | Sidebar + top nav     |

Top nav (app layout): logo left, `[Studio] [Profile ▼] [Logout]` right, connection dot far right. Admins also see a bell icon with unread `admin_notifications` badge count.

Admin sidebar: Users · Invites · Packs · Game Types · `← Back to App` at bottom.
