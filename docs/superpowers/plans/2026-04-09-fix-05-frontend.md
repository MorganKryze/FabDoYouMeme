# Frontend Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all frontend issues found in the 2026-04-09 code review (A4-* issues), including type mismatches, state initialization bugs, WS error handling, accessibility fixes, and UX improvements.

**Architecture:** SvelteKit with Svelte 5 runes. State is managed via singleton classes (`user`, `room`, `ws`, `toast`). Server routes use `+page.server.ts` for data loading and form actions.

**Tech Stack:** SvelteKit, Svelte 5, TypeScript, Tailwind CSS

---

### Task 1: Initialize user state from layout data

**Covers:** A4-C1

**Files:**
- Modify: `frontend/src/routes/(app)/+layout.svelte`

The `user` singleton (`$lib/state/user.svelte`) is never populated with the authenticated user's data. `data.user` is available from the server layout, but `user.setFrom()` is never called. The room page depends on `user.id` for host detection and self-vote guards.

- [ ] **Step 1: Call user.setFrom in the app layout**

In `frontend/src/routes/(app)/+layout.svelte`, add to the `<script>`:

```svelte
<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  // Populate reactive user singleton from server-loaded data.
  // This must run on every navigation since layout data can change.
  $effect(() => {
    user.setFrom(data.user);
  });

  const statusDot: Record<string, string> = {
    connected: 'bg-green-500 opacity-0 group-hover:opacity-100',
    reconnecting: 'bg-amber-400 animate-pulse',
    error: 'bg-red-500',
    closed: 'bg-gray-400',
  };
</script>
```

- [ ] **Step 2: Verify type compatibility**

`data.user` from `(app)/+layout.server.ts` is `locals.user`. Verify `locals.user` has the `{ id, username, email, role }` shape matching `user.setFrom()`'s parameter type. If `locals.user` has different field names, add a mapping.

- [ ] **Step 3: Verify the room page now receives user.id**

The room page (`[code]/+page.svelte`) derives `isHost` as:
```ts
const isHost = $derived(
  room.players.find((p) => p.user_id === user.id)?.is_host ?? false
);
```

With `user.id` now set, this will work — provided the backend sends `is_host` in player objects (see Task 8).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(app\)/+layout.svelte
git commit -m "fix(frontend): initialize user state singleton from layout data"
```

---

### Task 2: Fix session cookie parsing in verify handler

**Covers:** A4-C2

**Files:**
- Modify: `frontend/src/routes/(public)/auth/verify/+page.server.ts`

The regex `/^session=([^;]+)/` can fail if the `Set-Cookie` header doesn't start with `session=` or if attribute ordering changes. Use a proper cookie parser.

- [ ] **Step 1: Install the cookie package if not already present**

```bash
cd frontend && npm install cookie
```

- [ ] **Step 2: Replace regex with cookie library parsing**

In `frontend/src/routes/(public)/auth/verify/+page.server.ts`:

```typescript
import { redirect, fail } from '@sveltejs/kit';
import { parse as parseCookies } from 'cookie';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ url }) => {
  return {
    token: url.searchParams.get('token') ?? '',
    next: url.searchParams.get('next') ?? '/'
  };
};

export const actions: Actions = {
  default: async ({ request, fetch, cookies }) => {
    const data = await request.formData();
    const token = (data.get('token') as string | null) ?? '';
    const next = (data.get('next') as string | null) ?? '/';

    const res = await fetch(`${API_BASE}/api/auth/verify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token })
    });

    if (!res.ok) {
      let code = 'invalid_token';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        // ignore
      }
      return fail(400, { error: code });
    }

    // Parse the Set-Cookie header using a proper cookie parser to avoid
    // fragile regex that breaks on attribute reordering or whitespace.
    const rawCookie = res.headers.get('set-cookie') ?? '';
    const maxAgeMatch = rawCookie.match(/[Mm]ax-[Aa]ge=(\d+)/);
    // parseCookies only parses the name=value pairs; extract the session token:
    const parsed = parseCookies(rawCookie.split(';')[0]); // "session=TOKEN"
    const sessionValue = parsed['session'];

    if (sessionValue) {
      cookies.set('session', sessionValue, {
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'strict',
        maxAge: maxAgeMatch ? parseInt(maxAgeMatch[1]) : 720 * 3600
      });
    }

    throw redirect(303, next.startsWith('/') ? next : '/');
  }
};
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd frontend && npm run check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(public\)/auth/verify/+page.server.ts
git commit -m "fix(frontend): use cookie library to parse session token from Set-Cookie header"
```

---

### Task 3: Fix type mismatches in types.ts

**Covers:** A4-C3, A4-H6, A4-M7

**Files:**
- Modify: `frontend/src/lib/api/types.ts`

Three type mismatches:
1. `LeaderboardEntry.score` should not exist — use only `total_score`
2. `Submission.vote_count` → `votes_received`; `Submission.score` → `points_awarded`
3. `Round` missing `item: { payload, media_url? }` sub-object

- [ ] **Step 1: Update type definitions**

In `frontend/src/lib/api/types.ts`:

```typescript
export interface LeaderboardEntry {
  user_id: string;
  username: string;
  total_score: number;
  // removed: score (was a duplicate of total_score with wrong field name)
  rank: number;
}

export interface Round {
  round_number: number;
  ends_at: string;
  duration_seconds: number;
  item: {
    payload: unknown;
    media_url?: string | null;
  };
  // text_prompt and media_url at top level removed — now nested under item
  text_prompt?: string;   // keep for backward compat if backend flattens
  media_url?: string;     // keep for backward compat if backend flattens
}

export interface Submission {
  id: string;
  user_id: string;
  username: string;
  caption: string;
  votes_received?: number;   // was: vote_count
  points_awarded?: number;   // was: score
}
```

- [ ] **Step 2: Update all components that reference old field names**

Search for usages:

```bash
cd frontend && grep -rn "\.score\b\|vote_count\|entry\.score" src/
```

Update:
- Any `entry.score` → `entry.total_score`
- Any `submission.vote_count` → `submission.votes_received`
- Any `submission.score` → `submission.points_awarded`
- Any `round.media_url` directly (not `round.item.media_url`) — update to check both for backward compat

In `SubmitForm.svelte`, `round.media_url` is accessed directly. Update:

```svelte
{#if round.item?.media_url ?? round.media_url}
  <img src={round.item?.media_url ?? round.media_url} alt="Round prompt" ... />
{/if}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd frontend && npm run check
```

Fix any remaining type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/types.ts frontend/src/lib/games/
git commit -m "fix(frontend): correct LeaderboardEntry, Submission, and Round type fields to match backend protocol"
```

---

### Task 4: Add auth redirect on public landing page

**Covers:** A4-C4

**Files:**
- Create: `frontend/src/routes/(public)/+page.server.ts`

- [ ] **Step 1: Create the server load function**

```typescript
// frontend/src/routes/(public)/+page.server.ts
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals }) => {
  if (locals.user) {
    // Authenticated users should go to the app lobby, not the public landing page.
    throw redirect(303, '/');
  }
};
```

Note: The `(public)` group has its own layout with no `/` route by default. Check if there is a `+page.svelte` at `routes/(public)/`. If so, this adds a server load to it. If not, this won't apply to any page.

Actually looking at the route structure, `(public)` contains auth pages but the `/` route is in `(app)`. The "public landing page" the review refers to is likely `/` before authentication. Check the routes — if there's no `(public)/+page.svelte`, this task may not apply. Verify:

```bash
ls frontend/src/routes/(public)/
```

If there's no `+page.svelte` at `(public)/`, this is a non-issue. Create the file only if the public landing page exists.

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit if applicable**

```bash
git add frontend/src/routes/\(public\)/+page.server.ts
git commit -m "fix(frontend): redirect authenticated users from public landing page to app lobby"
```

---

### Task 5: Fix reorderItems payload format

**Covers:** A4-H5

**Files:**
- Modify: `frontend/src/lib/api/studio.ts`

The function sends `{ ordered_ids: [...] }` but the backend expects `{ positions: [{ id, position }, ...] }`.

- [ ] **Step 1: Update reorderItems**

In `frontend/src/lib/api/studio.ts`:

```typescript
export async function reorderItems(
  packId: string,
  positions: { id: string; position: number }[]
): Promise<void> {
  return api.patch<void>(`/api/packs/${packId}/items/reorder`, { positions });
}
```

- [ ] **Step 2: Update call sites**

Search for `reorderItems(` in the frontend:

```bash
cd frontend && grep -rn "reorderItems(" src/
```

Update any callers that pass `orderedIds: string[]` to pass the new format:

```typescript
// Before:
reorderItems(packId, items.map(item => item.id))

// After:
reorderItems(packId, items.map((item, i) => ({ id: item.id, position: i })))
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/studio.ts frontend/src/routes/
git commit -m "fix(frontend): reorderItems sends {positions:[{id,position}]} per protocol spec"
```

---

### Task 6: Handle WebSocket error messages

**Covers:** A4-H1

**Files:**
- Modify: `frontend/src/lib/state/room.svelte.ts`
- Modify: `frontend/src/lib/state/index.ts`

- [ ] **Step 1: Export toast from state index**

In `frontend/src/lib/state/index.ts`:

```typescript
export { user } from './user.svelte';
export { ws } from './ws.svelte';
export { room } from './room.svelte';
export { studio } from './studio.svelte';
export { toast } from './toast.svelte';
```

- [ ] **Step 2: Add error message handling to RoomState.handleMessage**

In `frontend/src/lib/state/room.svelte.ts`:

```typescript
import type { GameType, Player, LeaderboardEntry, Submission, Round, WsMessage } from '$lib/api/types';
import { toast } from './toast.svelte';

// ... inside handleMessage:
case 'error': {
  const d = msg.data as { code: string; message?: string };
  toast.error(d.message ?? d.code ?? 'An error occurred');
  // Reset transient UI state based on error code
  if (d.code === 'submission_closed' || d.code === 'already_submitted') {
    this.hasSubmitted = true; // Prevent retry
  }
  if (d.code === 'vote_closed' || d.code === 'already_voted') {
    this.hasVoted = true;
  }
  break;
}
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/state/room.svelte.ts frontend/src/lib/state/index.ts
git commit -m "fix(frontend): handle WS error messages with toast notifications; export toast from state index"
```

---

### Task 7: Fix pack fetch race condition on game type change

**Covers:** A4-H2, A4-M1

**Files:**
- Modify: `frontend/src/routes/(app)/+page.svelte`

Two issues: (1) rapidly switching game types can result in stale data; (2) `res.json()` is assigned directly to `Pack[]` but the backend returns a paginated response.

- [ ] **Step 1: Fix both issues in +page.svelte**

In `frontend/src/routes/(app)/+page.svelte`:

```svelte
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import type { Pack, PaginatedResponse } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let selectedGameTypeId = $state(data.gameTypes[0]?.id ?? '');
  let packs = $state<Pack[]>([]);
  let selectedPackId = $state('');
  let isSolo = $state(false);
  let roundCount = $state(5);
  let roundDuration = $state(60);
  let votingDuration = $state(30);
  let loadingPacks = $state(false);

  const selectedGameType = $derived(
    data.gameTypes.find((gt) => gt.id === selectedGameTypeId) ?? null
  );

  let currentAbortController: AbortController | null = null;

  async function loadPacks() {
    if (!selectedGameTypeId) return;

    // Cancel any in-flight request for a previous game type selection.
    currentAbortController?.abort();
    currentAbortController = new AbortController();
    const signal = currentAbortController.signal;

    loadingPacks = true;
    try {
      const res = await fetch(`/api/packs?game_type_id=${selectedGameTypeId}`, { signal });
      if (!res.ok) {
        packs = [];
        return;
      }
      const body: PaginatedResponse<Pack> = await res.json();
      packs = body.data ?? [];
      selectedPackId = packs[0]?.id ?? '';
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        packs = [];
      }
    } finally {
      loadingPacks = false;
    }
  }

  $effect(() => {
    void loadPacks();
  });
</script>
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(app\)/+page.svelte
git commit -m "fix(frontend): AbortController prevents stale pack data; extract paginated response correctly"
```

---

### Task 8: Fix host state preservation across player events

**Covers:** A4-C5

**Files:**
- Modify: `frontend/src/lib/state/room.svelte.ts`
- Modify: `frontend/src/routes/(app)/rooms/[code]/+layout.server.ts` (create if missing)

The `player_joined` and `room_state` handlers may not preserve `is_host`. Fix: when handling `player_joined`, if the incoming player data lacks `is_host`, derive it from the stored host user ID.

- [ ] **Step 1: Track host user ID in room state**

In `frontend/src/lib/state/room.svelte.ts`, add a `hostUserId` field:

```typescript
class RoomState {
  // ... existing fields ...
  hostUserId = $state<string | null>(null);

  init(data: { code: string; game_type: GameType; state: string; players: Player[]; host_id?: string }): void {
    this.code = data.code;
    this.gameType = data.game_type;
    this.state = data.state as RoomStatus;
    this.hostUserId = data.host_id ?? null;
    // Mark is_host on players
    this.players = (data.players ?? []).map(p => ({
      ...p,
      is_host: p.user_id === this.hostUserId
    }));
  }
```

- [ ] **Step 2: Preserve is_host in handleMessage**

```typescript
case 'player_joined': {
  const d = msg.data as Player;
  const enriched = { ...d, is_host: d.user_id === this.hostUserId };
  if (!this.players.find(p => p.user_id === d.user_id)) {
    this.players = [...this.players, enriched];
  }
  break;
}

case 'room_state': {
  const d = msg.data as { state: RoomStatus; players: Player[]; host_id?: string };
  this.state = d.state;
  if (d.host_id) this.hostUserId = d.host_id;
  this.players = (d.players ?? []).map(p => ({
    ...p,
    is_host: p.user_id === this.hostUserId
  }));
  break;
}
```

- [ ] **Step 3: Update room layout to pass host_id**

The room layout server (`+layout.server.ts`) needs to exist and pass `host_id` from the room data. Create it:

```typescript
// frontend/src/routes/(app)/rooms/[code]/+layout.server.ts
import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ params, fetch }) => {
  const res = await fetch(`${API_BASE}/api/rooms/${params.code}`);
  if (!res.ok) {
    // A4-M8: proper error message instead of generic 500
    throw error(res.status === 404 ? 404 : 500, 'Room not found or game ended.');
  }
  const room = await res.json();
  return { room };
};
```

- [ ] **Step 4: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/state/room.svelte.ts \
        frontend/src/routes/\(app\)/rooms/\[code\]/+layout.server.ts
git commit -m "fix(frontend): preserve is_host from host_id; add room layout server with error boundary"
```

---

### Task 9: Fix timer expired-on-mount and ARIA attributes

**Covers:** A4-H4, A4-M9

**Files:**
- Modify: `frontend/src/lib/games/meme-caption/SubmitForm.svelte`

- [ ] **Step 1: Handle expired deadline on mount; add ARIA**

In `SubmitForm.svelte`, update the timer section:

```svelte
<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { Round } from '$lib/api/types';

  let { round }: { round: Round } = $props();

  let caption = $state('');
  let submitted = $state(false);

  const MAX_CHARS = 300;
  const deadline = $derived(Date.parse(round.ends_at));
  const totalMs = $derived(round.duration_seconds * 1000);
  let timerMs = $state(Math.max(0, deadline - Date.now()));

  // If deadline is already past on mount, mark as expired immediately.
  const mountedExpired = $derived(deadline <= Date.now());

  $effect(() => {
    if (mountedExpired) return;
    const tick = () => {
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const progressPct = $derived(
    totalMs > 0 ? (timerMs / totalMs) * 100 : 0
  );
  const secondsLeft = $derived(Math.ceil(timerMs / 1000));
  const isExpired = $derived(timerMs <= 0 || mountedExpired);

  function submit() {
    if (submitted || isExpired || caption.trim().length === 0) return;
    ws.send('meme-caption:submit', { caption: caption.trim() });
    submitted = true;
  }
</script>

<div class="flex flex-col gap-6">
  <!-- Timer -->
  {#if mountedExpired}
    <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
      Submission window has closed.
    </div>
  {:else}
    <div class="flex items-center gap-3">
      <div
        class="flex-1 h-2 rounded-full bg-muted overflow-hidden"
        role="progressbar"
        aria-valuenow={secondsLeft}
        aria-valuemin={0}
        aria-valuemax={round.duration_seconds}
        aria-label="Time remaining"
      >
        <div
          class="h-full bg-primary transition-none rounded-full"
          style="width: {progressPct}%"
        ></div>
      </div>
      <span class="text-sm tabular-nums font-medium w-10 text-right">{secondsLeft}s</span>
      <span class="text-sm text-muted-foreground">Round {round.round_number}</span>
    </div>
  {/if}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/games/meme-caption/SubmitForm.svelte
git commit -m "fix(frontend): show 'window closed' on expired mount; add ARIA progressbar attributes"
```

---

### Task 10: Fix countdown animation and remove local phase transition

**Covers:** A4-M5, A4-M10

**Files:**
- Modify: `frontend/src/routes/(app)/rooms/[code]/+page.svelte`

- [ ] **Step 1: Remove local phase transition; check reduced-motion**

In `[code]/+page.svelte`:

```svelte
<script lang="ts">
  // ... existing imports ...

  let prefersReducedMotion = $state(false);

  $effect(() => {
    if (typeof window !== 'undefined') {
      prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
  });

  function startCountdown() {
    countdown = 3;
    countdownInterval = setInterval(() => {
      if (countdown === null) return;
      if (countdown <= 0) {
        clearInterval(countdownInterval!);
        countdown = null;
        // DO NOT set room.phase = 'submitting' here.
        // Phase transitions are driven exclusively by WS 'round_started' messages
        // to avoid racing with the server's authoritative state.
      } else {
        countdown--;
      }
    }, 1000);
  }
```

Update the countdown div to conditionally apply animation:

```svelte
<div class="fixed inset-0 bg-background/90 z-40 flex items-center justify-center">
  <div
    class="text-9xl font-black tabular-nums {prefersReducedMotion ? '' : 'animate-bounce'}"
    aria-live="assertive"
  >
    {countdown > 0 ? countdown : 'GO!'}
  </div>
</div>
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(app\)/rooms/\[code\]/+page.svelte
git commit -m "fix(frontend): phase transitions are WS-driven only; skip bounce animation on prefers-reduced-motion"
```

---

### Task 11: Fix consent checkboxes reset on registration error

**Covers:** A4-M3

**Files:**
- Modify: `frontend/src/routes/(public)/auth/register/+page.svelte`

- [ ] **Step 1: Restore checkbox state from form error data**

First, update the form action (in `+page.server.ts`) to echo back the checkbox state on failure:

In `frontend/src/routes/(public)/auth/register/+page.server.ts`, on failure, return `consent` and `age_affirmation` values:

```typescript
return fail(400, {
  error: messages[code] ?? 'Registration failed. Try again.',
  consent: formData.get('consent') === 'on',
  age_affirmation: formData.get('age_affirmation') === 'on',
});
```

Then in the `.svelte` file, initialize checkbox from form data:

```svelte
<script lang="ts">
  let consent = $state(form?.consent ?? false);
  let ageAffirmation = $state(form?.age_affirmation ?? false);
</script>

<!-- Use bind:checked with the state vars -->
<input type="checkbox" name="consent" bind:checked={consent} required />
<input type="checkbox" name="age_affirmation" bind:checked={ageAffirmation} required />
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(public\)/auth/register/
git commit -m "fix(frontend): restore consent checkbox state after registration form error"
```

---

### Task 12: Validate room existence before join redirect

**Covers:** A4-M4

**Files:**
- Modify: `frontend/src/routes/(app)/+page.server.ts`

- [ ] **Step 1: Update joinRoom action to validate room**

```typescript
joinRoom: async ({ request, fetch }) => {
  const data = await request.formData();
  const code = ((data.get('code') as string) ?? '').trim().toUpperCase();
  if (code.length !== 4)
    return fail(400, { joinError: 'Enter a 4-character room code.' });

  // Validate room exists before redirecting.
  const res = await fetch(`${API_BASE}/api/rooms/${code}`);
  if (!res.ok) {
    return fail(404, { joinError: 'Room not found. Check the code and try again.' });
  }

  throw redirect(303, `/rooms/${code}`);
}
```

- [ ] **Step 2: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(app\)/+page.server.ts
git commit -m "fix(frontend): validate room exists before join redirect; show inline error on 404"
```

---

### Task 13: Add copy room code feedback

**Covers:** A4-L1

**Files:**
- Modify: `frontend/src/routes/(app)/rooms/[code]/+layout.svelte`

- [ ] **Step 1: Add copy feedback state**

In `[code]/+layout.svelte`:

```svelte
<script lang="ts">
  // ... existing imports ...
  let copied = $state(false);
  let copyTimeout: ReturnType<typeof setTimeout> | null = null;

  async function copyRoomCode() {
    await navigator.clipboard.writeText(data.room.code);
    copied = true;
    if (copyTimeout) clearTimeout(copyTimeout);
    copyTimeout = setTimeout(() => { copied = false; }, 2000);
  }
</script>

<!-- Button -->
<button
  type="button"
  onclick={copyRoomCode}
  class="text-xs text-muted-foreground hover:text-foreground transition-colors"
  title="Copy room code"
>
  {copied ? 'Copied!' : 'Copy'}
</button>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/\(app\)/rooms/\[code\]/+layout.svelte
git commit -m "fix(frontend): show Copied! feedback after copying room code"
```

---

### Task 14: Fix admin API type and admin dashboard endpoints

**Covers:** A4-L3, A4-M2

**Files:**
- Modify: `frontend/src/lib/api/admin.ts`
- Modify: `frontend/src/routes/(admin)/admin/+page.server.ts`

- [ ] **Step 1: Add email and username to updateUser type**

In `frontend/src/lib/api/admin.ts`, find the `updateUser` function and expand the body type:

```typescript
export async function updateUser(
  id: string,
  body: {
    role?: 'player' | 'admin';
    is_active?: boolean;
    email?: string;
    username?: string;
  }
): Promise<User> {
  return api.patch<User>(`/api/admin/users/${id}`, body);
}
```

- [ ] **Step 2: Fix admin dashboard fetching nonexistent endpoints**

In `frontend/src/routes/(admin)/admin/+page.server.ts`, replace calls to `/api/admin/stats` and `/api/admin/audit-log` with real endpoints or placeholder data:

```typescript
import type { PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  // Fetch unread notification count for the badge (A4-M6)
  const notifRes = await fetch(`${API_BASE}/api/admin/notifications?unread=true&limit=1`);
  const notifData = notifRes.ok ? await notifRes.json() : { total: 0 };
  const unreadCount = notifData.total ?? 0;

  return {
    unreadCount,
    // Stats endpoint not yet implemented — use null to render placeholder
    stats: null,
  };
};
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api/admin.ts frontend/src/routes/\(admin\)/admin/+page.server.ts
git commit -m "fix(frontend): add email/username to updateUser type; remove calls to unimplemented admin endpoints"
```

---

### Task 15: Add admin notification badge

**Covers:** A4-M6

**Files:**
- Modify: `frontend/src/routes/(admin)/+layout.svelte`
- Modify: `frontend/src/routes/(admin)/+layout.server.ts` (create if missing)

- [ ] **Step 1: Create admin layout server to fetch unread count**

```typescript
// frontend/src/routes/(admin)/+layout.server.ts
import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ locals, fetch }) => {
  if (!locals.user || locals.user.role !== 'admin') {
    throw redirect(303, '/');
  }
  const res = await fetch(`${API_BASE}/api/admin/notifications?unread=true&limit=1`);
  const data = res.ok ? await res.json() : { total: 0 };
  return { unreadNotifications: data.total ?? 0 };
};
```

- [ ] **Step 2: Show badge in admin layout nav**

In `frontend/src/routes/(admin)/+layout.svelte`, add unread badge to the notifications nav link:

```svelte
<script lang="ts">
  import type { LayoutData } from './$types';
  let { children, data }: { children: any; data: LayoutData } = $props();
</script>

<!-- In the nav, find the notifications link and add: -->
<a href="/admin/notifications" class="relative ...">
  Notifications
  {#if data.unreadNotifications > 0}
    <span class="absolute -top-1 -right-1 h-4 w-4 rounded-full bg-red-500 text-xs text-white flex items-center justify-center leading-none">
      {data.unreadNotifications > 9 ? '9+' : data.unreadNotifications}
    </span>
  {/if}
</a>
```

- [ ] **Step 3: Verify TypeScript**

```bash
cd frontend && npm run check
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(admin\)/+layout.server.ts frontend/src/routes/\(admin\)/+layout.svelte
git commit -m "feat(frontend): admin nav shows unread notification badge count"
```

---

### Task 16: Upload progress reset on error; profile export error handling

**Covers:** A4-H3, A4-L7

**Files:**
- Modify: `frontend/src/lib/components/studio/ItemTable.svelte`
- Modify: `frontend/src/routes/(app)/profile/+page.svelte`

- [ ] **Step 1: Reset uploadProgress on error in ItemTable.svelte**

Find the bulk upload loop. Before each `continue` on error, reset the progress for that item:

```svelte
for (const file of files) {
  try {
    await uploadImageItem(packId, file.name, file);
    uploadProgress = /* update progress */;
  } catch (err) {
    uploadProgress = /* reset to 0 or skip item */;
    toast.error(`Failed to upload ${file.name}`);
  }
}
// Ensure clean state after loop exits
uploadProgress = 0;
```

- [ ] **Step 2: Add specific catch for blob creation in profile export**

In `profile/+page.svelte`, in the export handler:

```svelte
async function exportData() {
  try {
    const res = await fetch('/api/users/me/export');
    if (!res.ok) throw new Error(`Export failed: ${res.status}`);
    let blob: Blob;
    try {
      blob = await res.blob();
    } catch (blobErr) {
      toast.error('Could not prepare download file. Try again.');
      return;
    }
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'my-fabyoumeme-data.json';
    a.click();
    URL.revokeObjectURL(url);
  } catch (err) {
    toast.error('Export failed. Please try again.');
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/studio/ItemTable.svelte \
        frontend/src/routes/\(app\)/profile/+page.svelte
git commit -m "fix(frontend): reset upload progress on error; specific catch for blob creation in profile export"
```

---

### Task 17: Studio pagination and version comparison stub

**Covers:** A4-L4, A4-L5

**Files:**
- Modify: `frontend/src/routes/(app)/studio/+page.server.ts`
- Modify: `frontend/src/lib/components/studio/VersionHistory.svelte`

- [ ] **Step 1: Add load-more cursor pagination to studio pack list**

In `frontend/src/routes/(app)/studio/+page.server.ts`:

```typescript
export const load: PageServerLoad = async ({ fetch, url }) => {
  const cursor = url.searchParams.get('cursor') ?? '';
  const q = cursor ? `?after=${cursor}` : '';
  const res = await fetch(`${API_BASE}/api/packs${q}`);
  const body = res.ok ? await res.json() : { data: [], next_cursor: null };
  return {
    packs: body.data ?? [],
    nextCursor: body.next_cursor ?? null,
  };
};
```

Add a "Load more" button in the page component that navigates with the cursor.

- [ ] **Step 2: Remove or disable version comparison button**

In `VersionHistory.svelte`, find the compare button (lines 87-95) and either remove it or replace with a tooltip:

```svelte
<!-- Version comparison is not yet implemented. -->
<!-- <button onclick={compare}>Compare</button> -->
```

Or disable with a title:

```svelte
<button disabled title="Side-by-side comparison coming soon" class="opacity-50 cursor-not-allowed ...">
  Compare
</button>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/\(app\)/studio/+page.server.ts \
        frontend/src/lib/components/studio/VersionHistory.svelte
git commit -m "fix(frontend): add cursor-based pagination for studio pack list; disable unimplemented version compare"
```
