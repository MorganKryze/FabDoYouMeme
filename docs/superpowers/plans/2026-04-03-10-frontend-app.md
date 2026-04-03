# Frontend App Routes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the authenticated app routes: lobby (`/`), room game view (`/rooms/[code]`), and user profile (`/profile`).

**Architecture:** All routes live under the `(app)` SvelteKit route group with a shared layout that enforces authentication, shows the top-nav connection indicator, and provides the global toast system. Room page dispatches to game-type plugin components (meme-caption) based on the room's `game_type.slug`. WebSocket state is managed via `WsState` class from Phase 8.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, Tailwind CSS v4, shadcn-svelte, gorilla/websocket, `$lib/state/*` from Phase 8

---

## Files

| File                                                       | Role                                                                 |
| ---------------------------------------------------------- | -------------------------------------------------------------------- |
| `frontend/src/routes/(app)/+layout.svelte`                 | App layout: nav bar, connection indicator, toast stack               |
| `frontend/src/routes/(app)/+layout.server.ts`              | Session guard — redirect unauthenticated users to `/auth/magic-link` |
| `frontend/src/routes/(app)/+page.svelte`                   | App lobby: create room card + join room card                         |
| `frontend/src/routes/(app)/+page.server.ts`                | Load game types; create room action                                  |
| `frontend/src/routes/(app)/rooms/[code]/+layout.svelte`    | Room chrome: code banner + player panel                              |
| `frontend/src/routes/(app)/rooms/[code]/+layout.server.ts` | WS URL injection + room code validation                              |
| `frontend/src/routes/(app)/rooms/[code]/+page.svelte`      | Game view: dispatches to game type component                         |
| `frontend/src/routes/(app)/profile/+page.svelte`           | Profile: username/email change + data export                         |
| `frontend/src/routes/(app)/profile/+page.server.ts`        | Load current user; patch username/email actions                      |
| `frontend/src/lib/components/Toast.svelte`                 | Global toast component                                               |
| `frontend/src/lib/state/toast.svelte.ts`                   | Toast state class                                                    |
| `frontend/src/lib/games/meme-caption/SubmitForm.svelte`    | Meme caption submission form                                         |
| `frontend/src/lib/games/meme-caption/VoteForm.svelte`      | Meme caption vote form                                               |
| `frontend/src/lib/games/meme-caption/ResultsView.svelte`   | Meme caption round results reveal                                    |
| `frontend/src/lib/games/meme-caption/GameRules.svelte`     | Meme caption rules display                                           |

---

## Task 1: Toast System

**Files:**

- Create: `frontend/src/lib/state/toast.svelte.ts`
- Create: `frontend/src/lib/components/Toast.svelte`

- [ ] **Step 1: Write the toast state class**

```ts
// frontend/src/lib/state/toast.svelte.ts
export type ToastType = 'success' | 'warning' | 'error';

interface ToastItem {
  id: number;
  message: string;
  type: ToastType;
  /** Duration in ms. 0 = persistent (manual dismiss required). */
  duration: number;
}

class ToastState {
  #items = $state<ToastItem[]>([]);
  #nextId = 0;

  get items(): ToastItem[] {
    return this.#items;
  }

  show(message: string, type: ToastType = 'success'): void {
    const duration = type === 'error' ? 0 : type === 'warning' ? 5000 : 3000;
    const item: ToastItem = { id: this.#nextId++, message, type, duration };

    // Max 3 visible — drop oldest
    if (this.#items.length >= 3) {
      this.#items = this.#items.slice(1);
    }
    this.#items = [...this.#items, item];

    if (duration > 0) {
      setTimeout(() => this.dismiss(item.id), duration);
    }
  }

  dismiss(id: number): void {
    this.#items = this.#items.filter(t => t.id !== id);
  }
}

export const toast = new ToastState();
```

- [ ] **Step 2: Write the Toast component**

```svelte
<!-- frontend/src/lib/components/Toast.svelte -->
<script lang="ts">
  import { toast } from '$lib/state/toast.svelte';

  const typeClasses: Record<string, string> = {
    success: 'bg-green-600 text-white',
    warning: 'bg-yellow-500 text-white',
    error: 'bg-red-600 text-white',
  };
</script>

<div
  class="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 pointer-events-none"
  aria-live="polite"
  aria-atomic="false"
>
  {#each toast.items as item (item.id)}
    <div
      class="flex items-start gap-3 rounded-lg px-4 py-3 shadow-lg text-sm max-w-sm pointer-events-auto {typeClasses[item.type]}"
      role="alert"
    >
      <span class="flex-1 leading-snug">{item.message}</span>
      <button
        type="button"
        onclick={() => toast.dismiss(item.id)}
        class="shrink-0 text-current opacity-70 hover:opacity-100 transition-opacity text-lg leading-none"
        aria-label="Dismiss"
      >
        ×
      </button>
    </div>
  {/each}
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/state/toast.svelte.ts frontend/src/lib/components/Toast.svelte
git commit -m "feat(frontend): add global toast system (success/warning/error)"
```

---

## Task 2: App Layout

**Files:**

- Create: `frontend/src/routes/(app)/+layout.server.ts`
- Create: `frontend/src/routes/(app)/+layout.svelte`

- [ ] **Step 1: Write the session guard**

```ts
// frontend/src/routes/(app)/+layout.server.ts
import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals, url }) => {
  if (!locals.user) {
    const next = url.pathname + url.search;
    throw redirect(303, `/auth/magic-link?next=${encodeURIComponent(next)}`);
  }
  return { user: locals.user };
};
```

- [ ] **Step 2: Write the app layout**

```svelte
<!-- frontend/src/routes/(app)/+layout.svelte -->
<script lang="ts">
  import '../../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const statusDot: Record<string, string> = {
    connected: 'bg-green-500 opacity-0 group-hover:opacity-100',
    reconnecting: 'bg-amber-400 animate-pulse',
    error: 'bg-red-500',
    closed: 'bg-gray-400',
  };
</script>

<div class="min-h-screen flex flex-col bg-background text-foreground">
  <nav class="h-14 border-b border-border flex items-center px-4 gap-4">
    <a href="/" class="font-bold text-lg tracking-tight">FabDoYouMeme</a>
    <div class="flex-1" />

    <!-- Connection status indicator -->
    <div class="group relative flex items-center gap-1.5 cursor-default" title={ws.status}>
      <span class="h-2.5 w-2.5 rounded-full transition-all {statusDot[ws.status]}"></span>
      {#if ws.status === 'reconnecting'}
        <span class="text-xs text-amber-600 hidden sm:inline">Reconnecting…</span>
      {:else if ws.status === 'error'}
        <span class="text-xs text-red-600 hidden sm:inline">Connection lost</span>
        <button
          type="button"
          onclick={() => ws.connect()}
          class="text-xs underline text-red-600 hover:text-red-800"
        >
          Retry
        </button>
      {/if}
    </div>

    <a href="/profile" class="text-sm text-muted-foreground hover:text-foreground transition-colors">
      {data.user.username}
    </a>
    {#if data.user.role === 'admin'}
      <a href="/admin" class="text-sm text-muted-foreground hover:text-foreground transition-colors">
        Admin
      </a>
    {/if}
  </nav>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <Toast />
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(app\)/+layout.server.ts frontend/src/routes/\(app\)/+layout.svelte
git commit -m "feat(frontend): add app layout with session guard and connection status indicator"
```

---

## Task 3: App Lobby Page

**Files:**

- Create: `frontend/src/routes/(app)/+page.server.ts`
- Create: `frontend/src/routes/(app)/+page.svelte`

- [ ] **Step 1: Write the server load and create-room action**

```ts
// frontend/src/routes/(app)/+page.server.ts
import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { GameType, Pack } from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/game-types');
  const gameTypes: GameType[] = res.ok ? await res.json() : [];
  return { gameTypes };
};

export const actions: Actions = {
  createRoom: async ({ request, fetch }) => {
    const data = await request.formData();
    const game_type_id = data.get('game_type_id') as string;
    const pack_id = data.get('pack_id') as string;
    const is_solo = data.get('is_solo') === 'true';
    const round_count = Number(data.get('round_count') ?? 5);
    const round_duration_seconds = Number(
      data.get('round_duration_seconds') ?? 60
    );
    const voting_duration_seconds = Number(
      data.get('voting_duration_seconds') ?? 30
    );

    const res = await fetch('/api/rooms', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        game_type_id,
        pack_id,
        is_solo,
        config: { round_count, round_duration_seconds, voting_duration_seconds }
      })
    });

    if (!res.ok) {
      let code = 'unknown';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      const messages: Record<string, string> = {
        pack_no_supported_items:
          'This pack has no items compatible with the selected game type.',
        pack_insufficient_items:
          'This pack does not have enough items for the selected round count.',
        invalid_game_type: 'Invalid game type selected.'
      };
      return fail(400, {
        error: messages[code] ?? 'Could not create room. Try again.'
      });
    }

    const body = await res.json();
    throw redirect(303, `/rooms/${body.code}`);
  },

  joinRoom: async ({ request }) => {
    const data = await request.formData();
    const code = ((data.get('code') as string) ?? '').trim().toUpperCase();
    if (code.length !== 4)
      return fail(400, { joinError: 'Enter a 4-character room code.' });
    throw redirect(303, `/rooms/${code}`);
  }
};
```

- [ ] **Step 2: Write the lobby page component**

```svelte
<!-- frontend/src/routes/(app)/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';
  import type { Pack } from '$lib/api/types';

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

  async function loadPacks() {
    if (!selectedGameTypeId) return;
    loadingPacks = true;
    try {
      const res = await fetch(`/api/packs?game_type_id=${selectedGameTypeId}`);
      packs = res.ok ? await res.json() : [];
      selectedPackId = packs[0]?.id ?? '';
    } finally {
      loadingPacks = false;
    }
  }

  $effect(() => {
    void loadPacks();
  });
</script>

<svelte:head>
  <title>Lobby — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex items-start justify-center p-6 pt-12">
  <div class="w-full max-w-3xl grid grid-cols-1 md:grid-cols-2 gap-6">

    <!-- Create Room Card -->
    <div class="rounded-xl border border-border bg-card p-6 flex flex-col gap-4">
      <h2 class="text-lg font-semibold">Create a Room</h2>

      {#if form?.error}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.error}
        </div>
      {/if}

      <form method="POST" action="?/createRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="game_type" class="text-sm font-medium">Game Type</label>
          <select
            id="game_type"
            name="game_type_id"
            bind:value={selectedGameTypeId}
            class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          >
            {#each data.gameTypes as gt}
              <option value={gt.id}>{gt.name}</option>
            {/each}
          </select>
        </div>

        <div class="flex flex-col gap-1">
          <label for="pack" class="text-sm font-medium">Pack</label>
          {#if loadingPacks}
            <p class="text-sm text-muted-foreground">Loading packs…</p>
          {:else if packs.length === 0}
            <p class="text-sm text-muted-foreground">No compatible packs found for this game type.</p>
          {:else}
            <select
              id="pack"
              name="pack_id"
              bind:value={selectedPackId}
              class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            >
              {#each packs as p}
                <option value={p.id}>{p.name}</option>
              {/each}
            </select>
          {/if}
        </div>

        {#if selectedGameType?.supports_solo}
          <label class="flex items-center gap-2 cursor-pointer">
            <input type="checkbox" bind:checked={isSolo} class="h-4 w-4 rounded border-input" />
            <span class="text-sm">Solo mode</span>
          </label>
          <input type="hidden" name="is_solo" value={String(isSolo)} />
        {:else}
          <input type="hidden" name="is_solo" value="false" />
        {/if}

        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium">Rounds: {roundCount}</label>
          <input type="range" name="round_count" min={1} max={50} bind:value={roundCount}
            class="accent-primary" />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium">Submission time: {roundDuration}s</label>
          <input type="range" name="round_duration_seconds" min={15} max={300} step={5}
            bind:value={roundDuration} class="accent-primary" />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-sm font-medium">Voting time: {votingDuration}s</label>
          <input type="range" name="voting_duration_seconds" min={10} max={120} step={5}
            bind:value={votingDuration} class="accent-primary" />
        </div>

        <button
          type="submit"
          disabled={!selectedPackId}
          class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
        >
          Create Room
        </button>
      </form>
    </div>

    <!-- Join Room Card -->
    <div class="rounded-xl border border-border bg-card p-6 flex flex-col gap-4">
      <h2 class="text-lg font-semibold">Join a Room</h2>

      {#if form?.joinError}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.joinError}
        </div>
      {/if}

      <form method="POST" action="?/joinRoom" use:enhance class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label for="room-code" class="text-sm font-medium">Room Code</label>
          <input
            id="room-code"
            name="code"
            type="text"
            inputmode="text"
            autocapitalize="characters"
            maxlength={4}
            placeholder="WXYZ"
            autofocus
            class="h-14 rounded-lg border border-input bg-background px-4 text-center text-2xl font-mono tracking-widest uppercase focus:outline-none focus:ring-2 focus:ring-ring"
          />
        </div>

        <button
          type="submit"
          class="h-11 rounded-lg bg-secondary text-secondary-foreground font-semibold hover:bg-secondary/80 transition-colors"
        >
          Join Game
        </button>
      </form>
    </div>
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(app\)/+page.server.ts frontend/src/routes/\(app\)/+page.svelte
git commit -m "feat(frontend): add app lobby with create room and join room cards"
```

---

## Task 4: Room Layout

**Files:**

- Create: `frontend/src/routes/(app)/rooms/[code]/+layout.server.ts`
- Create: `frontend/src/routes/(app)/rooms/[code]/+layout.svelte`

- [ ] **Step 1: Write the room layout server load**

```ts
// frontend/src/routes/(app)/rooms/[code]/+layout.server.ts
import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ params, fetch, url }) => {
  const res = await fetch(`/api/rooms/${params.code}`);
  if (!res.ok) throw error(404, `Room ${params.code} not found`);
  const roomData = await res.json();

  // Build WS URL — replace http(s) with ws(s)
  const wsBase = url.origin.replace(/^http/, 'ws');

  return {
    room: roomData,
    wsUrl: `${wsBase}/api/ws/rooms/${params.code}`
  };
};
```

- [ ] **Step 2: Write the room layout component**

```svelte
<!-- frontend/src/routes/(app)/rooms/[code]/+layout.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  onMount(() => {
    room.init(data.room);
    ws.connect(data.wsUrl);
    ws.onMessage((msg) => room.handleMessage(msg));
  });

  onDestroy(() => {
    ws.disconnect();
  });
</script>

<div class="flex-1 flex flex-col">
  <!-- Room header bar -->
  <div class="border-b border-border px-4 py-2 flex items-center gap-4">
    <div class="flex items-center gap-2">
      <span class="text-xs text-muted-foreground uppercase tracking-wider">Room</span>
      <span class="font-mono font-bold text-lg">{data.room.code}</span>
      <button
        type="button"
        onclick={() => navigator.clipboard.writeText(data.room.code)}
        class="text-xs text-muted-foreground hover:text-foreground transition-colors"
        title="Copy room code"
      >
        Copy
      </button>
    </div>

    {#if room.gameType}
      <span class="text-sm text-muted-foreground">{room.gameType.name}</span>
    {/if}

    <div class="flex-1" />

    <!-- Reconnecting banner -->
    {#if ws.status === 'reconnecting'}
      <div class="text-xs text-amber-600 animate-pulse">
        Reconnecting… (attempt {ws.retryCount} / 10)
      </div>
    {/if}
  </div>

  <div class="flex-1 flex overflow-hidden">
    <!-- Main game area -->
    <div class="flex-1 overflow-y-auto">
      {@render children()}
    </div>

    <!-- Player panel -->
    <aside class="w-48 shrink-0 border-l border-border overflow-y-auto px-3 py-4">
      <h3 class="text-xs font-semibold uppercase text-muted-foreground tracking-wider mb-3">
        Players ({room.players.length})
      </h3>
      <ul class="flex flex-col gap-1.5">
        {#each room.players as player}
          <li class="flex items-center gap-2 text-sm">
            <span class="h-2 w-2 rounded-full {player.connected ? 'bg-green-500' : 'bg-gray-300'}"></span>
            <span class="truncate">{player.username}</span>
            {#if player.is_host}
              <span class="text-xs text-muted-foreground ml-auto">(host)</span>
            {/if}
          </li>
        {/each}
      </ul>
    </aside>
  </div>
</div>
```

- [ ] **Step 3: Add `init` and player connection tracking to RoomState**

Edit `frontend/src/lib/state/room.svelte.ts` to add an `init()` method that populates from the HTTP room response:

```ts
// Add to RoomState class in room.svelte.ts
init(data: { code: string; game_type: GameType; state: string; players: Player[] }): void {
  this.code = data.code;
  this.gameType = data.game_type;
  this.state = data.state as typeof this.state;
  this.players = data.players;
}
```

- [ ] **Step 4: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/\(app\)/rooms/
git commit -m "feat(frontend): add room layout with WS connection and player panel"
```

---

## Task 5: Room Game Page (Game Phase Dispatcher)

**Files:**

- Create: `frontend/src/routes/(app)/rooms/[code]/+page.svelte`

- [ ] **Step 1: Write the game page dispatcher**

```svelte
<!-- frontend/src/routes/(app)/rooms/[code]/+page.svelte -->
<script lang="ts">
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import MemeCaptionSubmitForm from '$lib/games/meme-caption/SubmitForm.svelte';
  import MemeCaptionVoteForm from '$lib/games/meme-caption/VoteForm.svelte';
  import MemeCaptionResultsView from '$lib/games/meme-caption/ResultsView.svelte';
  import MemeCaptionGameRules from '$lib/games/meme-caption/GameRules.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  const isHost = $derived(
    room.players.find((p) => p.user_id === user.id)?.is_host ?? false
  );

  let countdown = $state<number | null>(null);
  let countdownInterval: ReturnType<typeof setInterval> | null = null;

  function startCountdown() {
    countdown = 3;
    countdownInterval = setInterval(() => {
      if (countdown === null) return;
      if (countdown <= 0) {
        clearInterval(countdownInterval!);
        countdown = null;
        room.phase = 'submitting';
      } else {
        countdown--;
      }
    }, 1000);
  }

  $effect(() => {
    // Listen for game_started to trigger countdown
    if (room.phase === 'countdown') {
      startCountdown();
    }
  });

  async function kickPlayer(userId: string) {
    await fetch(`/api/rooms/${data.room.code}/kick`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    });
  }

  async function startGame() {
    ws.send({ type: 'start' });
  }

  async function nextRound() {
    ws.send({ type: 'next_round' });
  }
</script>

<div class="p-6 flex flex-col gap-6">

  <!-- Countdown overlay -->
  {#if countdown !== null}
    <div class="fixed inset-0 bg-background/90 z-40 flex items-center justify-center">
      <div class="text-9xl font-black animate-bounce tabular-nums" aria-live="assertive">
        {countdown > 0 ? countdown : 'GO!'}
      </div>
    </div>
  {/if}

  <!-- Lobby phase -->
  {#if room.phase === 'idle' && room.state === 'lobby'}
    <div class="flex flex-col items-center gap-8 text-center">
      <div class="text-muted-foreground">
        {#if isHost}
          Waiting for players to join. Start when ready.
        {:else}
          Waiting for {room.players.find((p) => p.is_host)?.username ?? 'host'} to start…
        {/if}
      </div>

      {#if room.gameType}
        <MemeCaptionGameRules gameType={room.gameType} />
      {/if}

      {#if isHost}
        <div class="flex flex-col gap-3 w-full max-w-xs">
          <button
            type="button"
            onclick={startGame}
            disabled={room.players.length < 2}
            class="h-12 rounded-lg bg-primary text-primary-foreground font-semibold text-lg disabled:opacity-50 hover:bg-primary/90 transition-colors"
          >
            Start Game ▶
          </button>
          {#if room.players.length < 2}
            <p class="text-xs text-muted-foreground">Need at least 2 players to start.</p>
          {/if}

          <!-- Kick buttons in player panel are shown in layout.svelte -->
        </div>
      {/if}
    </div>

  <!-- Submission phase -->
  {:else if room.phase === 'submitting' && room.currentRound}
    <MemeCaptionSubmitForm round={room.currentRound} />

  <!-- Voting phase -->
  {:else if room.phase === 'voting'}
    <MemeCaptionVoteForm submissions={room.submissions} />

  <!-- Results phase -->
  {:else if room.phase === 'results'}
    <MemeCaptionResultsView
      submissions={room.submissions}
      leaderboard={room.leaderboard}
      {isHost}
      onNextRound={nextRound}
    />

  <!-- Game ended -->
  {:else if room.state === 'finished'}
    <div class="flex flex-col items-center gap-6 text-center">
      <h2 class="text-3xl font-bold">🏆 Game Over</h2>
      <ol class="flex flex-col gap-2 w-full max-w-xs">
        {#each room.leaderboard as entry, i}
          <li class="flex items-center gap-3 text-lg">
            <span class="w-6 text-muted-foreground text-right">
              {i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : `${i + 1}.`}
            </span>
            <span class="flex-1 text-left font-medium">{entry.username}</span>
            <span class="text-muted-foreground">{entry.score} pts</span>
          </li>
        {/each}
      </ol>
      <div class="flex gap-3">
        <a
          href="/"
          class="h-10 px-6 rounded-lg border border-border text-sm font-medium flex items-center hover:bg-muted transition-colors"
        >
          Back to Lobby
        </a>
      </div>
    </div>
  {/if}
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors (some may come from missing game components — create stubs in Task 6).

---

## Task 6: Meme-Caption Game Components

**Files:**

- Create: `frontend/src/lib/games/meme-caption/SubmitForm.svelte`
- Create: `frontend/src/lib/games/meme-caption/VoteForm.svelte`
- Create: `frontend/src/lib/games/meme-caption/ResultsView.svelte`
- Create: `frontend/src/lib/games/meme-caption/GameRules.svelte`

- [ ] **Step 1: SubmitForm**

```svelte
<!-- frontend/src/lib/games/meme-caption/SubmitForm.svelte -->
<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { Round } from '$lib/api/types';

  let { round }: { round: Round } = $props();

  let caption = $state('');
  let submitted = $state(false);

  const MAX_CHARS = 300;
  const deadline = $derived(Date.parse(round.ends_at));
  let timerMs = $state(Math.max(0, deadline - Date.now()));

  $effect(() => {
    const tick = () => {
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const progressPct = $derived(
    round.duration_seconds > 0
      ? (timerMs / (round.duration_seconds * 1000)) * 100
      : 0
  );
  const secondsLeft = $derived(Math.ceil(timerMs / 1000));
  const isExpired = $derived(timerMs <= 0);

  function submit() {
    if (submitted || isExpired || caption.trim().length === 0) return;
    ws.send({ type: 'meme_caption:submit', payload: { caption: caption.trim() } });
    submitted = true;
  }

  const submittedPlayers = $derived(
    room.players.filter((p) => room.submissions.some((s) => s.user_id === p.user_id))
  );
</script>

<div class="flex flex-col gap-6">
  <!-- Timer -->
  <div class="flex items-center gap-3">
    <div class="flex-1 h-2 rounded-full bg-muted overflow-hidden">
      <div
        class="h-full bg-primary transition-none rounded-full"
        style="width: {progressPct}%"
      ></div>
    </div>
    <span class="text-sm tabular-nums font-medium w-10 text-right">{secondsLeft}s</span>
    <span class="text-sm text-muted-foreground">Round {round.round_number}</span>
  </div>

  <!-- Media prompt (if present) -->
  {#if round.media_url}
    <img
      src={round.media_url}
      alt="Round prompt"
      class="w-full aspect-video object-cover rounded-lg border border-border"
    />
  {/if}

  {#if round.text_prompt}
    <p class="text-center text-muted-foreground italic">"{round.text_prompt}"</p>
  {/if}

  <!-- Caption input -->
  {#if submitted}
    <div class="rounded-lg border border-border bg-muted p-4 text-center text-sm text-muted-foreground">
      Submitted ✓ — waiting for others…
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      <textarea
        bind:value={caption}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={3}
        placeholder="Write your caption…"
        class="w-full rounded-lg border border-input bg-background p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
      ></textarea>
      <div class="flex items-center justify-between">
        <span class="text-xs text-muted-foreground">{caption.length}/{MAX_CHARS}</span>
        <button
          type="button"
          onclick={submit}
          disabled={submitted || isExpired || caption.trim().length === 0}
          class="h-10 px-6 rounded-lg bg-primary text-primary-foreground text-sm font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
        >
          Submit
        </button>
      </div>
    </div>
  {/if}

  <!-- Player submission status -->
  <div class="flex flex-wrap gap-2">
    {#each room.players as player}
      {@const hasSub = room.submissions.some((s) => s.user_id === player.user_id)}
      <span class="flex items-center gap-1 text-xs px-2 py-1 rounded-full border {hasSub ? 'border-green-300 bg-green-50 text-green-800' : 'border-border text-muted-foreground'}">
        {hasSub ? '✓' : '⏳'} {player.username}
      </span>
    {/each}
  </div>
</div>
```

- [ ] **Step 2: VoteForm**

```svelte
<!-- frontend/src/lib/games/meme-caption/VoteForm.svelte -->
<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { Submission } from '$lib/api/types';

  let { submissions }: { submissions: Submission[] } = $props();

  let selectedId = $state<string | null>(null);
  let voted = $state(false);

  const deadline = $derived(room.currentRound ? Date.parse(room.currentRound.ends_at) : 0);
  let timerMs = $state(Math.max(0, deadline - Date.now()));

  $effect(() => {
    if (!deadline) return;
    const tick = () => {
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const secondsLeft = $derived(Math.ceil(timerMs / 1000));

  function vote() {
    if (!selectedId || voted) return;
    ws.send({ type: 'meme_caption:vote', payload: { submission_id: selectedId } });
    voted = true;
  }
</script>

<div class="flex flex-col gap-6">
  <div class="flex items-center justify-between">
    <h3 class="font-semibold">Vote for the best caption</h3>
    <span class="text-sm tabular-nums text-muted-foreground">{secondsLeft}s</span>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
    {#each submissions as sub}
      {@const isOwn = sub.user_id === user.id}
      <button
        type="button"
        onclick={() => { if (!voted) selectedId = sub.id; }}
        disabled={voted || isOwn}
        class="relative rounded-xl border-2 p-4 text-left transition-colors
          {selectedId === sub.id ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/50'}
          {isOwn ? 'cursor-default' : 'cursor-pointer'}
          disabled:opacity-70"
      >
        {#if isOwn}
          <span class="absolute top-2 right-2 text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
            You
          </span>
        {/if}
        {#if selectedId === sub.id}
          <span class="absolute top-2 left-2 text-primary">✓</span>
        {/if}
        <p class="text-sm leading-relaxed pr-8">{sub.caption}</p>
      </button>
    {/each}
  </div>

  {#if !voted}
    <button
      type="button"
      onclick={vote}
      disabled={!selectedId}
      class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
    >
      Vote
    </button>
  {:else}
    <p class="text-center text-sm text-muted-foreground">Voted ✓ — waiting for results…</p>
  {/if}
</div>
```

- [ ] **Step 3: ResultsView**

```svelte
<!-- frontend/src/lib/games/meme-caption/ResultsView.svelte -->
<script lang="ts">
  import type { Submission, LeaderboardEntry } from '$lib/api/types';

  let {
    submissions,
    leaderboard,
    isHost,
    onNextRound,
  }: {
    submissions: Submission[];
    leaderboard: LeaderboardEntry[];
    isHost: boolean;
    onNextRound: () => void;
  } = $props();
</script>

<div class="flex flex-col gap-6">
  <h3 class="font-semibold text-lg">Round Results</h3>

  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
    {#each submissions as sub}
      <div class="rounded-xl border border-border p-4 flex flex-col gap-2">
        <p class="text-sm leading-relaxed">{sub.caption}</p>
        <div class="flex items-center gap-2 text-xs text-muted-foreground mt-auto">
          <span class="font-medium text-foreground">{sub.username}</span>
          <span>·</span>
          <span>{sub.vote_count ?? 0} vote{(sub.vote_count ?? 0) !== 1 ? 's' : ''}</span>
          {#if (sub.score ?? 0) > 0}
            <span class="ml-auto text-green-600 font-medium">+{sub.score} pts</span>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div>
    <h4 class="text-sm font-semibold text-muted-foreground uppercase tracking-wider mb-2">Leaderboard</h4>
    <ol class="flex flex-col gap-1">
      {#each leaderboard as entry, i}
        <li class="flex items-center gap-3 text-sm py-1">
          <span class="w-5 text-right text-muted-foreground">{i + 1}.</span>
          <span class="flex-1">{entry.username}</span>
          <span class="font-medium tabular-nums">{entry.score} pts</span>
        </li>
      {/each}
    </ol>
  </div>

  {#if isHost}
    <button
      type="button"
      onclick={onNextRound}
      class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
    >
      Next Round →
    </button>
  {:else}
    <p class="text-center text-sm text-muted-foreground">Waiting for host to continue…</p>
  {/if}
</div>
```

- [ ] **Step 4: GameRules**

```svelte
<!-- frontend/src/lib/games/meme-caption/GameRules.svelte -->
<script lang="ts">
  import type { GameType } from '$lib/api/types';
  let { gameType }: { gameType: GameType } = $props();
</script>

<div class="rounded-lg border border-border bg-muted/40 p-4 text-sm text-muted-foreground text-left max-w-sm">
  <p class="font-medium text-foreground mb-1">{gameType.name}</p>
  <p>{gameType.description}</p>
</div>
```

- [ ] **Step 5: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/\(app\)/rooms/ frontend/src/lib/games/
git commit -m "feat(frontend): add room game page and meme-caption game components"
```

---

## Task 7: Profile Page

**Files:**

- Create: `frontend/src/routes/(app)/profile/+page.server.ts`
- Create: `frontend/src/routes/(app)/profile/+page.svelte`

- [ ] **Step 1: Write the server load and actions**

```ts
// frontend/src/routes/(app)/profile/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals }) => {
  return { user: locals.user! };
};

export const actions: Actions = {
  updateUsername: async ({ request, fetch }) => {
    const data = await request.formData();
    const username = (data.get('username') as string | null) ?? '';

    const res = await fetch('/api/users/me', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username })
    });

    if (!res.ok) {
      let code = 'error';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      if (code === 'username_taken') {
        return fail(409, { usernameError: 'That username is already taken.' });
      }
      return fail(400, { usernameError: 'Failed to update username.' });
    }
    return { usernameSuccess: true };
  },

  requestEmailChange: async ({ request, fetch }) => {
    const data = await request.formData();
    const email = (data.get('email') as string | null) ?? '';

    const res = await fetch('/api/users/me', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    });

    if (!res.ok) {
      return fail(400, { emailError: 'Failed to send verification email.' });
    }
    return { emailSent: true };
  }
};
```

- [ ] **Step 2: Write the profile page component**

```svelte
<!-- frontend/src/routes/(app)/profile/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let editingUsername = $state(false);
  let editingEmail = $state(false);

  $effect(() => {
    if (form?.usernameSuccess) {
      editingUsername = false;
      toast.show('Username updated.', 'success');
    }
    if (form?.emailSent) {
      editingEmail = false;
      toast.show('Check your new email address for a verification link.', 'success');
    }
  });

  async function downloadExport() {
    const res = await fetch('/api/users/me/export');
    if (!res.ok) { toast.show('Failed to export data. Try again.', 'error'); return; }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'fabyoumeme-export.json';
    a.click();
    URL.revokeObjectURL(url);
    toast.show('Your data export is ready.', 'success');
  }
</script>

<svelte:head>
  <title>Profile — FabDoYouMeme</title>
</svelte:head>

<div class="max-w-lg mx-auto p-6 flex flex-col gap-8">
  <h1 class="text-2xl font-bold">Profile</h1>

  <!-- Username section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Username</h2>
    {#if editingUsername}
      <form method="POST" action="?/updateUsername" use:enhance class="flex flex-col gap-2">
        {#if form?.usernameError}
          <p class="text-sm text-red-600">{form.usernameError}</p>
        {/if}
        <div class="flex gap-2">
          <input
            name="username"
            type="text"
            value={data.user.username}
            minlength={3}
            maxlength={30}
            autofocus
            class="flex-1 h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          />
          <button type="submit" class="h-10 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
            Save
          </button>
          <button type="button" onclick={() => editingUsername = false}
            class="h-10 px-4 rounded-md border border-border text-sm hover:bg-muted">
            Cancel
          </button>
        </div>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm">{data.user.username}</span>
        <button type="button" onclick={() => editingUsername = true}
          class="text-xs text-muted-foreground underline hover:text-foreground">
          Edit
        </button>
      </div>
    {/if}
  </section>

  <!-- Email section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Email</h2>
    {#if editingEmail}
      <form method="POST" action="?/requestEmailChange" use:enhance class="flex flex-col gap-2">
        {#if form?.emailError}
          <p class="text-sm text-red-600">{form.emailError}</p>
        {/if}
        <div class="flex gap-2">
          <input
            name="email"
            type="email"
            autofocus
            class="flex-1 h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="new@example.com"
          />
          <button type="submit" class="h-10 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
            Send Verification
          </button>
          <button type="button" onclick={() => editingEmail = false}
            class="h-10 px-4 rounded-md border border-border text-sm hover:bg-muted">
            Cancel
          </button>
        </div>
        <p class="text-xs text-muted-foreground">Your current email stays active until you click the verification link.</p>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm">{data.user.email}</span>
        <button type="button" onclick={() => editingEmail = true}
          class="text-xs text-muted-foreground underline hover:text-foreground">
          Change Email
        </button>
      </div>
    {/if}
  </section>

  <!-- Data & Privacy section -->
  <section class="flex flex-col gap-4">
    <h2 class="text-base font-semibold">Data & Privacy</h2>

    <div class="flex flex-col gap-2">
      <p class="text-sm text-muted-foreground">
        Download a copy of all your personal data stored in this service.
      </p>
      <button
        type="button"
        onclick={downloadExport}
        class="self-start h-10 px-5 rounded-lg border border-border text-sm font-medium hover:bg-muted transition-colors"
      >
        Download My Data
      </button>
    </div>

    <div class="rounded-lg border border-border bg-muted/40 p-4 flex flex-col gap-1">
      <p class="text-sm font-medium">Delete My Account</p>
      <p class="text-sm text-muted-foreground">
        To request deletion of your account and all associated data, contact your admin.
        See the <a href="/privacy" class="underline hover:text-foreground">Privacy Policy</a> for details.
      </p>
    </div>
  </section>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(app\)/profile/
git commit -m "feat(frontend): add profile page with username/email edit and data export"
```

---

## Task 8: Integration Smoke Test

- [ ] **Step 1: Start the dev stack**

```bash
docker compose up --build
```

- [ ] **Step 2: Register + log in**

1. Navigate to `http://localhost:5173`
2. Go to `/auth/register?invite=<admin-created-token>`
3. Fill form, check both boxes, submit
4. Click magic link in Mailpit (`http://localhost:8025`)
5. Verify redirect to app lobby `/`

- [ ] **Step 3: Create a room and join via WS**

1. Select game type + pack, set round config
2. Click "Create Room" → should redirect to `/rooms/WXYZ`
3. Verify room header shows code, player panel shows your username
4. In a second browser window, log in as a different user and join the same room
5. Verify both players appear in the player panel

- [ ] **Step 4: Play a round**

1. Host clicks "Start Game" — countdown overlay should appear
2. Both players see submission form with timer bar
3. Both submit captions — verify submission status pills update
4. Voting phase: verify your own submission has "You" badge
5. Results: verify vote counts and leaderboard appear

- [ ] **Step 5: Profile page**

1. Navigate to `/profile`
2. Edit username, save — verify success toast
3. Click "Download My Data" — verify JSON file downloads

- [ ] **Step 6: Commit if fixes needed**

```bash
git commit -m "fix(frontend): resolve app route smoke test issues"
```
