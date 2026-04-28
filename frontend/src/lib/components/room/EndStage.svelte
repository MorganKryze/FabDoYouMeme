<script lang="ts">
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Play, LogOut, Trophy, PartyPopper } from '$lib/icons';
  import type { LeaderboardEntry } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';

  const END_REASONS = $derived<Record<string, { headline: string; subtext: string | null }>>({
    game_complete:             { headline: m.room_end_reason_game_complete_headline(),     subtext: m.room_end_reason_game_complete_subtext() },
    pack_exhausted:            { headline: m.room_end_reason_pack_exhausted_headline(),    subtext: m.room_end_reason_pack_exhausted_subtext() },
    all_players_disconnected:  { headline: m.room_end_reason_all_disconnected_headline(),  subtext: m.room_end_reason_all_disconnected_subtext() },
    host_disconnected:         { headline: m.room_end_reason_host_disconnected_headline(), subtext: m.room_end_reason_host_disconnected_subtext() },
  });

  const endInfo = $derived(
    room.endReason && END_REASONS[room.endReason]
      ? END_REASONS[room.endReason]
      : { headline: m.room_end_reason_default(), subtext: null }
  );

  // Match the lobby's per-player color palette so avatars stay visually
  // consistent from the waiting room through to the final scoreboard.
  const AVATAR_COLORS = ['#E8937F', '#8BC9B1', '#D4A5C9', '#A8D8EA', '#FDDCB5', '#B5E2D0'];

  function colorFor(userId: string): string {
    const idx = room.players.findIndex((p) => p.user_id === userId);
    return AVATAR_COLORS[(idx >= 0 ? idx : 0) % AVATAR_COLORS.length];
  }

  function initials(name: string): string {
    return name.slice(0, 2).toUpperCase();
  }

  // Confetti lifecycle: full burst for 30s, thinned for the next 30s, then
  // completely removed so the stage calms down.
  let confettiPhase = $state<'full' | 'reduced' | 'none'>('full');

  // "Show all" leaderboard expansion: collapsed by default on phones (the
  // podium alone fills a phone viewport); expanded by default at md+ so
  // desktop gets the full standings without an extra click.
  let restExpanded = $state(false);
  $effect(() => {
    if (typeof window === 'undefined') return;
    const mql = window.matchMedia('(min-width: 768px)');
    restExpanded = mql.matches;
  });

  $effect(() => {
    const reduceAt = setTimeout(() => { confettiPhase = 'reduced'; }, 30_000);
    const stopAt = setTimeout(() => { confettiPhase = 'none'; }, 60_000);
    return () => {
      clearTimeout(reduceAt);
      clearTimeout(stopAt);
    };
  });

  // Stable particle set — spread across the whole viewport with varied
  // colors, shapes, drift, and timing. Generated once at mount.
  // A deterministic pseudo-random hash per index avoids the visible
  // "pairing" and wave patterns that integer modulo produces.
  const rnd = (i: number, salt: number) => {
    const x = Math.sin(i * 12.9898 + salt * 78.233) * 43758.5453;
    return x - Math.floor(x);
  };

  const CONFETTI_PARTICLES = Array.from({ length: 180 }, (_, i) => ({
    left: `${rnd(i, 1) * 100}%`,
    delay: `${rnd(i, 2) * 7}s`,
    duration: `${3.8 + rnd(i, 3) * 3.2}s`,
    color: AVATAR_COLORS[Math.floor(rnd(i, 4) * AVATAR_COLORS.length)],
    shape: Math.floor(rnd(i, 5) * 3),
    drift: `${Math.round((rnd(i, 6) - 0.5) * 260)}px`,
  }));

  const winner = $derived<LeaderboardEntry | null>(room.leaderboard[0] ?? null);
  const rest = $derived(room.leaderboard.slice(3));

  // New Game → game-choice for registered users; sign-in for guests.
  const newGameHref = $derived(user.id !== null ? '/host' : '/auth/magic-link');

  // Portal: move the confetti layer to <body> so its `position: fixed`
  // attaches to the viewport. An ancestor with any non-`none` transform
  // (e.g. `.stage-wrap` on the page wrapper) would otherwise re-scope
  // the fixed positioning to that element's box.
  function portalToBody(node: HTMLElement) {
    document.body.appendChild(node);
    return {
      destroy() {
        node.remove();
      },
    };
  }

  // Podium order: 2nd, 1st, 3rd — classic stepped silhouette with the
  // champion in the middle at the tallest height.
  type PodiumSlot = { entry: LeaderboardEntry; rank: 1 | 2 | 3; heightClass: string; medal: string };
  const podiumArranged = $derived.by<(PodiumSlot | null)[]>(() => {
    const order: (1 | 2 | 3)[] = [2, 1, 3];
    return order.map((r) => {
      const entry = room.leaderboard[r - 1];
      if (!entry) return null;
      const heightClass = r === 1 ? 'h-44' : r === 2 ? 'h-32' : 'h-24';
      const medal = r === 1 ? '\u{1F947}' : r === 2 ? '\u{1F948}' : '\u{1F949}';
      return { entry, rank: r, heightClass, medal };
    });
  });


</script>

<!-- Viewport-wide confetti — rendered OUTSIDE the reveal-wrapped
     container so its `position: fixed` attaches to the viewport. A
     transformed ancestor would re-scope it to that element's box. -->
{#if confettiPhase !== 'none'}
  <div use:portalToBody class="confetti" class:reduced={confettiPhase === 'reduced'} aria-hidden="true">
    {#each CONFETTI_PARTICLES as p, i (i)}
      <span
        class="p{p.shape}"
        style="left: {p.left}; background: {p.color}; animation-delay: {p.delay}; animation-duration: {p.duration}; --drift: {p.drift};"
      ></span>
    {/each}
  </div>
{/if}

<div class="relative w-full max-w-5xl mx-auto px-4 sm:px-6 py-3 sm:py-4 flex flex-col gap-4 sm:gap-8" use:reveal>

  <!-- ═══════════════════════════════════════════════════════════════
       HEADER — reason chip + winner hero.
       ═══════════════════════════════════════════════════════════════ -->
  <div class="flex flex-col items-center gap-3 text-center relative z-[1]">
    <span
      class="inline-flex items-center gap-2 h-9 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-[0.65rem] font-bold uppercase tracking-[0.2em]"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <PartyPopper size={14} strokeWidth={2.5} />
      {endInfo.headline}
    </span>

    {#if endInfo.subtext}
      <p class="text-xs text-brand-text-muted max-w-xs">{endInfo.subtext}</p>
    {/if}

    {#if winner}
      <h1
        class="inline-flex flex-wrap items-baseline justify-center gap-x-3 text-brand-text text-3xl sm:text-7xl md:text-8xl font-extrabold leading-none tracking-tight"
      >
        <span>{winner.display_name}</span>
        <span class="wins-gradient">{m.room_wins_suffix()}</span>
      </h1>
      <p class="text-sm font-semibold text-brand-text-muted">
        {m.room_wins_score({ score: winner.score })}
      </p>
    {:else}
      <h1 class="text-3xl sm:text-7xl md:text-8xl font-extrabold leading-none tracking-tight">
        {m.room_final_scores()}
      </h1>
    {/if}
  </div>

  <!-- ═══════════════════════════════════════════════════════════════
       PODIUM — top 3 rendered as a stepped 2-1-3 silhouette.
       ═══════════════════════════════════════════════════════════════ -->
  {#if room.leaderboard.length > 0}
    <div class="grid grid-cols-3 gap-3 sm:gap-5 items-end max-w-2xl mx-auto w-full relative z-[1]">
      {#each podiumArranged as slot, i (i)}
        {#if slot}
          <div class="flex flex-col items-center gap-3 min-w-0" use:reveal={{ delay: i + 1 }}>
            <div class="relative">
              <span
                class="h-16 w-16 sm:h-20 sm:w-20 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy flex items-center justify-center text-sm sm:text-base font-bold text-white"
                style="background: {colorFor(slot.entry.player_id)}; box-shadow: 0 5px 0 rgba(0,0,0,0.14);"
              >
                {initials(slot.entry.display_name)}
              </span>
              <span class="absolute -top-2 -right-2 text-2xl leading-none select-none" aria-hidden="true">
                {slot.medal}
              </span>
            </div>
            <div
              class="w-full rounded-t-3xl border-[2.5px] border-brand-border-heavy bg-brand-surface flex flex-col items-center justify-start gap-1 px-3 pt-4 pb-5 min-w-0 {slot.heightClass}"
              style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
            >
              <span class="text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
                #{slot.rank}
              </span>
              <span class="font-bold text-brand-text text-center truncate max-w-full px-1">
                {slot.entry.display_name}
              </span>
              <span class="font-bold tabular-nums text-brand-text-mid text-sm">
                {slot.entry.score} pts
              </span>
            </div>
          </div>
        {:else}
          <div aria-hidden="true"></div>
        {/if}
      {/each}
    </div>
  {/if}

  <!-- ═══════════════════════════════════════════════════════════════
       REST OF LEADERBOARD — only shown when 4+ players.
       Collapsed by default on mobile so the podium fits one viewport;
       open by default at md+ where vertical room is plentiful.
       ═══════════════════════════════════════════════════════════════ -->
  {#if rest.length > 0}
    <div
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 w-full max-w-xl mx-auto relative z-[1]"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      use:reveal={{ delay: 3 }}
    >
      <button
        type="button"
        onclick={() => (restExpanded = !restExpanded)}
        aria-expanded={restExpanded}
        class="w-full flex items-center justify-between gap-2 text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted cursor-pointer"
      >
        <span class="inline-flex items-center gap-2">
          <Trophy size={12} strokeWidth={2.5} />
          {m.room_final_standings()}
        </span>
        <span class="font-mono tabular-nums">{rest.length}</span>
      </button>
      {#if restExpanded}
      <ol class="flex flex-col gap-2 mt-3">
        {#each rest as entry, i (entry.player_id)}
          <li class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border bg-brand-white px-3 py-2 text-sm">
            <span class="w-6 text-right font-bold text-brand-text-muted tabular-nums">
              {i + 4}.
            </span>
            <span
              class="h-8 w-8 shrink-0 rounded-full border-[2px] border-brand-border-heavy flex items-center justify-center text-[0.6rem] font-bold text-white"
              style="background: {colorFor(entry.player_id)};"
            >
              {initials(entry.display_name)}
            </span>
            <span class="flex-1 text-left font-bold truncate">{entry.display_name}</span>
            <span class="font-bold tabular-nums text-brand-text-mid">
              {entry.score} pts
            </span>
          </li>
        {/each}
      </ol>
      {/if}
    </div>
  {/if}

  <!-- ═══════════════════════════════════════════════════════════════
       ACTIONS — new game or leave.
       ═══════════════════════════════════════════════════════════════ -->
  <div class="flex flex-col items-center gap-3 w-full max-w-xs mx-auto relative z-[1]" use:reveal={{ delay: 4 }}>
    <a
      href={newGameHref}
      use:pressPhysics={'dark'}
      use:hoverEffect={'gradient'}
      class="w-full h-14 px-10 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold text-lg cursor-pointer inline-flex items-center justify-center gap-2"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.22);"
    >
      <Play size={20} strokeWidth={2.5} />
      {m.room_new_game()}
    </a>

    <a
      href="/"
      use:pressPhysics={'ghost'}
      class="w-full h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <LogOut size={16} strokeWidth={2.5} />
      {m.room_leave()}
    </a>
  </div>
</div>

<style>
  /* Viewport-wide confetti. Fixed positioning spans the whole screen so
     particles aren't bound to the EndStage container's own box. */
  .confetti {
    position: fixed;
    inset: 0;
    pointer-events: none;
    overflow: hidden;
    z-index: 0;
  }
  .confetti span {
    position: absolute;
    top: -40px;
    opacity: 0;
    animation-name: confettiFall;
    animation-timing-function: ease-in;
    animation-iteration-count: infinite;
    will-change: transform, opacity;
    transition: opacity 0.6s ease;
  }
  .confetti span.p0 { width: 10px; height: 14px; border-radius: 2px; }
  .confetti span.p1 { width: 8px;  height: 8px;  border-radius: 50%; }
  .confetti span.p2 { width: 6px;  height: 16px; border-radius: 2px; }

  /* Thinned phase: hide two out of every three particles. */
  .confetti.reduced span:nth-child(3n),
  .confetti.reduced span:nth-child(3n + 1) {
    display: none;
  }

  @keyframes confettiFall {
    0%   { transform: translate3d(0, 0, 0) rotate(0deg);              opacity: 0; }
    8%   { opacity: 1; }
    100% { transform: translate3d(var(--drift, 0), 110vh, 0) rotate(720deg); opacity: 0; }
  }

  /* "wins!" — gradient text tied to the live time-of-day palette.
     `paint-order: stroke fill` draws the dark outline first so the
     gradient fill sits on top; the stepped drop-shadow mirrors the
     `box-shadow: 0 5px 0` language used across the stage. */
  .wins-gradient {
    background: linear-gradient(
      135deg,
      var(--brand-grad-2),
      var(--brand-grad-3),
      var(--brand-grad-4),
      var(--brand-grad-2)
    );
    background-size: 200% 200%;
    background-clip: text;
    -webkit-background-clip: text;
    color: transparent;
    -webkit-text-stroke: 3px var(--brand-text);
    paint-order: stroke fill;
    filter: drop-shadow(0 4px 0 rgba(0, 0, 0, 0.22));
    animation: winsFlow 4s ease-in-out infinite;
  }
  @keyframes winsFlow {
    0%, 100% { background-position: 0% 50%; }
    50%      { background-position: 100% 50%; }
  }

  @media (prefers-reduced-motion: reduce) {
    .confetti { display: none; }
    .wins-gradient { animation: none; }
  }
</style>
