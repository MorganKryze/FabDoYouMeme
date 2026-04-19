<script lang="ts">
  import { onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { cubicIn, cubicOut } from 'svelte/easing';
  import { enhance } from '$app/forms';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import MakerCard from '$lib/components/MakerCard.svelte';
  import { computeMedals } from '$lib/medals';
  import { tone } from '$lib/state/tone.svelte';
  import { pickForSlot } from '$lib/content/toneSelect';
  import type { TonePair } from '$lib/content/tonePools';
  import {
    Play,
    Sparkles,
    Clock,
    Users,
    IdCard,
    XCircle,
  } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { HistoryRoom } from './+page.server';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let code = $state('');
  let joinForm = $state<HTMLFormElement | null>(null);
  let showMakerCard = $state(false);

  // Greeting rotates per visit — we deliberately capture `tone.level` once
  // on mount rather than binding reactively. Changes on /profile take effect
  // on the next /home navigation.
  let greetingPair = $state<TonePair | null>(null);

  onMount(() => {
    const lastRaw = sessionStorage.getItem('fdym:home_greeting_last');
    let last: TonePair | null = null;
    if (lastRaw) {
      try {
        last = JSON.parse(lastRaw) as TonePair;
      } catch {
        last = null;
      }
    }
    const picked = pickForSlot('home_greeting', tone.level, last);
    greetingPair = picked;
    try {
      sessionStorage.setItem('fdym:home_greeting_last', JSON.stringify(picked));
    } catch {
      // quota / private mode — anti-repeat is best-effort
    }
  });

  const usernameStr = $derived(data.user?.username ?? 'there');
  const greetingH1Parts = $derived.by(() => {
    if (!greetingPair) return null;
    const idx = greetingPair.h1.indexOf('{username}');
    if (idx === -1) return { before: greetingPair.h1, after: '' };
    return {
      before: greetingPair.h1.slice(0, idx),
      after: greetingPair.h1.slice(idx + '{username}'.length)
    };
  });
  const greetingSub = $derived(greetingPair?.subline.replaceAll('{username}', usernameStr) ?? null);

  const medals = $derived(
    data.user ? computeMedals(data.user, data.history) : []
  );

  function submitJoin(next: string) {
    code = next;
    if (next.length === 4 && joinForm) joinForm.requestSubmit();
  }

  // ─── Derived stats from history ────────────────────────────
  // The dashboard derives its numbers client-side from the paginated
  // history payload. This is good enough until we need all-time
  // aggregates beyond the window — at which point, build a
  // /api/users/me/stats endpoint server-side.
  const history = $derived(data.history);
  const gamesPlayed = $derived(history.length);
  const wins = $derived(history.filter((r: HistoryRoom) => r.rank === 1).length);
  const winRate = $derived(
    gamesPlayed === 0 ? 0 : Math.round((wins / gamesPlayed) * 100)
  );
  const bestRank = $derived.by<number | null>(() => {
    if (history.length === 0) return null;
    let best = Infinity;
    for (const r of history) if (r.rank && r.rank < best) best = r.rank;
    return best === Infinity ? null : best;
  });
  const earnedMedalCount = $derived(medals.filter((m) => m.earned).length);

  const favoriteGameType = $derived.by(() => {
    if (history.length === 0) return null;
    const counts = new Map<string, number>();
    for (const r of history) {
      counts.set(r.game_type_slug, (counts.get(r.game_type_slug) ?? 0) + 1);
    }
    let top = '';
    let topN = 0;
    for (const [slug, n] of counts) {
      if (n > topN) {
        top = slug;
        topN = n;
      }
    }
    return top;
  });

  function formatRelative(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime();
    const minutes = Math.floor(diff / 60_000);
    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    if (days < 7) return `${days}d ago`;
    const weeks = Math.floor(days / 7);
    if (weeks < 5) return `${weeks}w ago`;
    return new Date(iso).toLocaleDateString();
  }

  function prettyGameSlug(slug: string): string {
    return slug
      .split('-')
      .map((s) => s[0]?.toUpperCase() + s.slice(1))
      .join(' ');
  }
</script>

<svelte:head>
  <title>Dashboard — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex justify-center p-6 pt-4 pb-10">
  <div class="w-full max-w-5xl flex flex-col gap-10">

    <!-- ─── Hero row: greeting (full-width) ──────────────────── -->
    <section use:reveal class="flex flex-col justify-center gap-3" aria-live="polite">
      <p class="text-sm font-bold uppercase tracking-[0.3em] text-brand-text-mid">
        Welcome back
      </p>
      {#if greetingH1Parts && greetingSub}
        <h1
          class="greeting-h1 font-extrabold leading-[0.98] tracking-tight m-0"
          in:fade={{ duration: 150 }}
        >{greetingH1Parts.before}<span
            class="username-gradient">{usernameStr}</span>{greetingH1Parts.after}</h1>
        <p
          class="font-semibold text-brand-text-mid m-0"
          style="font-size: clamp(1rem, 1.6vw, 1.25rem);"
          in:fade={{ duration: 150 }}
        >
          {greetingSub}
        </p>
        {#if bestRank !== null || earnedMedalCount > 0}
          <div class="flex flex-wrap gap-2 mt-1">
            {#if bestRank !== null}
              <span class="meta-chip">
                <span class="glyph">★</span>
                Best rank · {bestRank}
              </span>
            {/if}
            {#if earnedMedalCount > 0}
              <span class="meta-chip">
                <span class="glyph">♠</span>
                {earnedMedalCount} medal{earnedMedalCount !== 1 ? 's' : ''}
              </span>
            {/if}
          </div>
        {/if}
      {:else}
        <!-- SSR / pre-mount skeleton: blurred placeholder that reserves layout.
             aria-hidden so screen readers don't read the placeholder copy
             inside the aria-live region before the rotated greeting arrives. -->
        <!-- svelte-ignore a11y_hidden -->
        <h1 aria-hidden="true" class="greeting-h1 font-extrabold blur-sm opacity-40 select-none m-0">Hey there.</h1>
        <p aria-hidden="true" class="font-semibold text-brand-text-mid m-0 blur-sm opacity-40 select-none" style="font-size: clamp(1rem, 1.6vw, 1.25rem);">
          Jump back into a room, or spin up a new one.
        </p>
      {/if}
    </section>

    <!-- ─── Quick actions: either "return to room" OR create/join ───
         A user can be in at most one lobby/playing room at a time. When
         data.activeRoom is set, the create + join affordances are hidden
         and replaced with a single "return to your room" CTA — backed by
         the backend's single-room enforcement in RoomHandler.Create and
         WSHandler.ServeHTTP. ───────────────────────────────────────── -->
    {#if data.activeRoom}
      <section use:reveal={{ delay: 1 }}>
        <a
          href={`/rooms/${data.activeRoom.code}`}
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          use:physCard
          class="active-room group relative block rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-6 overflow-hidden no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.2);"
        >
          <div class="relative flex items-center justify-between gap-4">
            <div class="flex flex-col gap-1 min-w-0">
              <span class="text-xs font-bold uppercase tracking-[0.2em] opacity-70">
                {data.activeRoom.is_host ? 'Your room' : "You're in"} · {data.activeRoom.state === 'playing' ? 'In progress' : 'Lobby'}
              </span>
              <div class="text-xl font-bold leading-tight">
                Return to <span class="wavy font-mono tracking-widest">{data.activeRoom.code}</span>
              </div>
              <div class="text-xs font-bold uppercase tracking-[0.2em] opacity-70 truncate">
                {prettyGameSlug(data.activeRoom.game_type_slug)}
              </div>
            </div>
            <div class="flex items-center gap-3 shrink-0">
              <span class="inline-flex items-center justify-center h-12 w-12 rounded-full bg-brand-white text-brand-text border-[2.5px] border-brand-border-heavy transition-transform group-hover:translate-x-0.5">
                <Play size={18} strokeWidth={2.5} />
              </span>
            </div>
          </div>
          <p class="relative text-xs font-semibold opacity-70 mt-3">
            You can only be in one room at a time. Leave or finish this one to create or join another.
          </p>
        </a>
      </section>
    {:else}
      <section use:reveal={{ delay: 1 }} class="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-4 items-stretch">
        <!-- Join by code -->
        <div
          class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-3"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
        >
          <div class="flex items-center justify-between">
            <h2 class="text-lg font-bold">Got a code?</h2>
            <span class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
              Jump in
            </span>
          </div>

          {#if form?.joinError}
            <div
              role="alert"
              class="inline-flex items-center justify-center gap-2 rounded-full border-[2.5px] border-brand-accent bg-brand-accent/15 px-5 py-2 text-xs font-bold text-center text-brand-text"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              <XCircle size={14} strokeWidth={2.5} />
              {form.joinError}
            </div>
          {/if}

          <form
            bind:this={joinForm}
            method="POST"
            action="?/joinRoom"
            use:enhance
            class="grid grid-cols-[1fr_auto] gap-3 items-end"
          >
            <RoomCodeInput bind:value={code} onenter={submitJoin} />
            <button
              use:pressPhysics={'dark'}
              use:hoverEffect={'gradient'}
              type="submit"
              disabled={code.length !== 4}
              class="h-16 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-40 cursor-pointer inline-flex items-center justify-center gap-2 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            >
              <Play size={18} strokeWidth={2.5} />
              Play
            </button>
          </form>
        </div>

        <!-- Host shortcut -->
        <a
          href="/host"
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          use:physCard
          class="group rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-5 flex flex-col justify-between gap-2 min-w-[200px] no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.2);"
        >
          <div class="text-xs font-bold uppercase tracking-[0.2em] opacity-70">
            Host
          </div>
          <div class="text-xl font-bold leading-tight">Start a<br />new game</div>
          <div class="text-xs font-bold uppercase tracking-[0.2em] opacity-70 inline-flex items-center gap-1 transition-transform group-hover:translate-x-0.5">
            Pick a game →
          </div>
        </a>
      </section>
    {/if}

    <!-- ─── Stats row ────────────────────────────────────────── -->
    <section class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div
        use:reveal={{ delay: 2 }}
        class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1 overflow-hidden"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <span class="bg-suit" aria-hidden="true">♠</span>
        <div class="relative flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          <span class="font-mono" style="color: var(--brand-text);">♠</span>
          Played
        </div>
        <div class="relative text-4xl font-bold leading-none mt-1 tabular-nums">{gamesPlayed}</div>
      </div>

      <div
        use:reveal={{ delay: 3 }}
        class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1 overflow-hidden"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <span class="bg-suit" aria-hidden="true">♥</span>
        <div class="relative flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          <span class="font-mono" style="color: var(--brand-accent);">♥</span>
          Wins
        </div>
        <div class="relative text-4xl font-bold leading-none mt-1 tabular-nums">{wins}</div>
      </div>

      <div
        use:reveal={{ delay: 4 }}
        class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1 overflow-hidden"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <span class="bg-suit" aria-hidden="true">♦</span>
        <div class="relative flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          <span class="font-mono" style="color: var(--brand-accent);">♦</span>
          Win rate
        </div>
        <div class="relative text-4xl font-bold leading-none mt-1 tabular-nums">{winRate}%</div>
      </div>

      <div
        use:reveal={{ delay: 5 }}
        class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1 overflow-hidden"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <span class="bg-suit" aria-hidden="true">♣</span>
        <div class="relative flex items-center gap-1.5 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          <span class="font-mono" style="color: var(--brand-text);">♣</span>
          Favourite
        </div>
        <div class="relative text-lg font-bold leading-tight mt-1 line-clamp-1">
          {favoriteGameType ? prettyGameSlug(favoriteGameType) : '—'}
        </div>
      </div>
    </section>

    <!-- ─── Recent activity + circles placeholder ────────────── -->
    <section class="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-5">
      <!-- Recent activity -->
      <div
        use:reveal={{ delay: 2 }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-4"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Clock size={16} strokeWidth={2.5} />
            <h2 class="text-lg font-bold">Recent activity</h2>
          </div>
          <a
            href="/profile"
            class="text-xs font-bold text-brand-text hover:underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          >
            See all →
          </a>
        </div>

        {#if history.length === 0}
          <div class="flex flex-col items-center text-center gap-3 py-6">
            <p class="text-sm font-bold">No games yet.</p>
            <p class="text-xs font-semibold text-brand-text-mid max-w-xs">
              Your history lands here after the first round.
            </p>
            <a
              href="/host"
              use:pressPhysics={'ghost'}
              use:hoverEffect={'swap'}
              class="inline-flex items-center gap-2 px-5 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold no-underline text-xs focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              <Play size={14} strokeWidth={2.5} />
              Host your first room
            </a>
          </div>
        {:else}
          <ul class="flex flex-col gap-2">
            {#each history.slice(0, 5) as room}
              <li
                class="flex items-center justify-between gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2.5 text-sm"
                style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
              >
                <div class="flex items-center gap-3 min-w-0 flex-1">
                  <span
                    class="font-mono font-bold tracking-widest text-xs inline-flex items-center justify-center rounded-full bg-brand-surface border-[2.5px] border-brand-border-heavy px-2.5 h-8"
                  >
                    {room.code}
                  </span>
                  <div class="flex flex-col min-w-0">
                    <span class="inline-flex items-center gap-2 font-bold truncate">
                      {#if room.rank === 1}
                        <span class="medal gold" aria-label="First place">1</span>
                      {:else if room.rank === 2}
                        <span class="medal silver" aria-label="Second place">2</span>
                      {:else if room.rank === 3}
                        <span class="medal bronze" aria-label="Third place">3</span>
                      {/if}
                      <span class="truncate">{prettyGameSlug(room.game_type_slug)}</span>
                    </span>
                    <span class="text-xs font-semibold text-brand-text-mid truncate">
                      {room.pack_name} · {formatRelative(room.started_at)}
                    </span>
                  </div>
                </div>
                <div class="flex items-center gap-4 shrink-0">
                  <div class="text-right hidden sm:block">
                    <div class="text-xs font-bold uppercase tracking-[0.15em] text-brand-text-mid">
                      Rank
                    </div>
                    <div class="font-bold text-sm tabular-nums">
                      {room.rank}<span class="text-brand-text-mid">/{room.player_count}</span>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="text-xs font-bold uppercase tracking-[0.15em] text-brand-text-mid">
                      Score
                    </div>
                    <div class="font-bold text-sm tabular-nums">{room.score}</div>
                  </div>
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      </div>

      <!-- Circles (mocked placeholder) -->
      <div
        use:reveal={{ delay: 3 }}
        class="rounded-[22px] border-[2.5px] border-dashed border-brand-border-heavy bg-brand-white/40 p-5 flex flex-col gap-3 relative overflow-hidden"
      >
        <div class="absolute top-3 right-3 text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid px-2 py-0.5 rounded-full border-[2px] border-brand-border-heavy bg-brand-white">
          Coming soon
        </div>
        <div class="flex items-center gap-2">
          <Users size={16} strokeWidth={2.5} />
          <h2 class="text-lg font-bold">Your circles</h2>
        </div>
        <p class="text-xs font-semibold text-brand-text-mid">
          Track the people you play with most. See who you keep beating (or losing to), revisit old rooms, tag your favourite opponents.
        </p>

        <!-- Placeholder circle chips -->
        <div class="flex flex-col gap-2 mt-2 opacity-60 pointer-events-none select-none">
          {#each ['Sunday crew', 'The office', 'D&D group'] as name}
            <div
              class="flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-1.5 text-xs"
            >
              <div class="h-6 w-6 rounded-full bg-brand-surface border-[2px] border-brand-border-heavy"></div>
              <span class="font-bold">{name}</span>
              <span class="ml-auto text-xs font-semibold text-brand-text-mid">—</span>
            </div>
          {/each}
        </div>
      </div>
    </section>

  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-mid">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:underline rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60">Privacy Policy</a>
</footer>

<!-- ─── Floating Maker Card (home only) ─────────────────────────────
     Bottom-left floating button that expands into the full Maker Card
     on click. Both elements are independently `fixed` so each slide
     transition anchors to the same bottom-left corner without layout
     reflow between states. The card carries its own close button
     (passed via `onClose`) so the wrapper stays non-interactive and
     card content remains selectable / focusable.
     ───────────────────────────────────────────────────────────── -->
<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && showMakerCard) showMakerCard = false;
  }}
/>

{#if data.user}
  {#if showMakerCard}
    <div
      class="fixed bottom-20 left-6 z-40 w-75"
      in:fly={{ x: -380, duration: 420, delay: 100, easing: cubicOut }}
      out:fly={{ x: -380, duration: 260, easing: cubicIn }}
    >
      <MakerCard user={data.user} {medals} onClose={() => (showMakerCard = false)} />
    </div>
  {:else}
    <button
      type="button"
      onclick={() => (showMakerCard = true)}
      use:hoverEffect={'swap'}
      aria-label="Show maker card"
      class="fixed bottom-20 left-6 z-40 inline-flex items-center justify-center h-14 w-14 rounded-full bg-brand-white border-[2.5px] border-brand-border-heavy cursor-pointer focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.12);"
      in:fly={{ x: -300, duration: 360, delay: 140, easing: cubicOut }}
      out:fly={{ x: -300, duration: 260, easing: cubicIn }}
    >
      <IdCard size={22} strokeWidth={2.5} />
    </button>
  {/if}
{/if}

<style>
  /* Dashboard greeting — bold, hero-scale, but not landing-page hero.
     clamp() keeps it proportionate: 3rem on tiny screens, up to 6rem on
     wide desktops. Line-height tuned tight so the two-line fallback
     ("Oh good, admin is here.") still reads as a single beat. */
  .greeting-h1 {
    font-size: clamp(2.5rem, 5.5vw, 4.25rem);
    letter-spacing: -0.02em;
  }

  .username-gradient {
    background: linear-gradient(
      95deg,
      var(--brand-accent),
      var(--brand-accent-3) 60%,
      var(--brand-accent-2)
    );
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }

  .wavy {
    text-decoration: underline;
    text-decoration-style: wavy;
    text-decoration-color: var(--brand-accent);
    text-underline-offset: 6px;
    text-decoration-thickness: 2px;
  }

  /* Decorative suit watermark on the active-room banner. */
  .active-room::after {
    content: "♠ ♥ ♦ ♣";
    position: absolute;
    right: -10px;
    bottom: -26px;
    font-size: 120px;
    font-weight: 700;
    letter-spacing: 0.1em;
    color: var(--brand-white);
    opacity: 0.06;
    pointer-events: none;
    transform: rotate(-6deg);
    white-space: nowrap;
  }
</style>
