<script lang="ts">
  import { enhance } from '$app/forms';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import RoomCodeInput from '$lib/components/RoomCodeInput.svelte';
  import {
    Play,
    Sparkles,
    Trophy,
    Gamepad2,
    Clock,
    Users,
    PartyPopper,
  } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { HistoryRoom } from './+page.server';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let code = $state('');
  let joinForm = $state<HTMLFormElement | null>(null);

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

    <!-- ─── Greeting ─────────────────────────────────────────── -->
    <section use:reveal class="flex flex-col gap-1">
      <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
        Welcome back
      </p>
      <h1 class="text-4xl font-bold">Hey {data.user?.username ?? 'there'}.</h1>
      <p class="text-sm font-semibold text-brand-text-muted mt-1">
        Jump back into a room, or spin up a new one.
      </p>
    </section>

    <!-- ─── Quick actions: code + host shortcut ─────────────── -->
    <section use:reveal={{ delay: 1 }} class="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-4 items-stretch">
      <!-- Join by code -->
      <div
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-3"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-bold">Got a code?</h2>
          <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Jump in
          </span>
        </div>

        {#if form?.joinError}
          <div
            class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-2 text-xs font-bold text-center"
            style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
          >
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
            class="h-16 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-40 cursor-pointer inline-flex items-center justify-center gap-2"
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
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-5 flex flex-col justify-between gap-2 min-w-[200px] no-underline"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.2);"
      >
        <div class="text-[0.65rem] font-bold uppercase tracking-[0.2em] opacity-70">
          Host
        </div>
        <div class="text-xl font-bold leading-tight">Start a<br />new game</div>
        <div class="text-[0.65rem] font-bold uppercase tracking-[0.2em] opacity-70">
          Pick a game →
        </div>
      </a>
    </section>

    <!-- ─── Stats row ────────────────────────────────────────── -->
    <section class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div
        use:reveal={{ delay: 2 }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center gap-1.5 text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
          <Gamepad2 size={12} strokeWidth={2.5} />
          Played
        </div>
        <div class="text-3xl font-bold leading-none mt-1">{gamesPlayed}</div>
      </div>

      <div
        use:reveal={{ delay: 3 }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center gap-1.5 text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
          <Trophy size={12} strokeWidth={2.5} />
          Wins
        </div>
        <div class="text-3xl font-bold leading-none mt-1">{wins}</div>
      </div>

      <div
        use:reveal={{ delay: 4 }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center gap-1.5 text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
          <PartyPopper size={12} strokeWidth={2.5} />
          Win rate
        </div>
        <div class="text-3xl font-bold leading-none mt-1">{winRate}%</div>
      </div>

      <div
        use:reveal={{ delay: 5 }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-1"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <div class="flex items-center gap-1.5 text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
          <Sparkles size={12} strokeWidth={2.5} />
          Favourite
        </div>
        <div class="text-lg font-bold leading-tight mt-1 line-clamp-1">
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
            class="text-xs font-bold text-brand-text-muted hover:text-brand-text transition-colors"
          >
            See all →
          </a>
        </div>

        {#if history.length === 0}
          <div class="flex flex-col items-center text-center gap-2 py-6">
            <p class="text-sm font-bold">No games yet.</p>
            <p class="text-xs font-semibold text-brand-text-muted max-w-xs">
              Host a room or drop in with a code — your history lands here after the first round.
            </p>
          </div>
        {:else}
          <ul class="flex flex-col gap-2">
            {#each history as room}
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
                    <span class="font-bold truncate">{prettyGameSlug(room.game_type_slug)}</span>
                    <span class="text-[0.65rem] font-semibold text-brand-text-muted truncate">
                      {room.pack_name} · {formatRelative(room.started_at)}
                    </span>
                  </div>
                </div>
                <div class="flex items-center gap-4 shrink-0">
                  <div class="text-right hidden sm:block">
                    <div class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
                      Rank
                    </div>
                    <div class="font-bold text-sm">
                      {room.rank}<span class="text-brand-text-muted">/{room.player_count}</span>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
                      Score
                    </div>
                    <div class="font-bold text-sm">{room.score}</div>
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
        <div class="absolute top-3 right-3 text-[0.55rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-2 py-0.5 rounded-full border-[2px] border-brand-border-heavy bg-brand-white">
          Coming soon
        </div>
        <div class="flex items-center gap-2">
          <Users size={16} strokeWidth={2.5} />
          <h2 class="text-lg font-bold">Your circles</h2>
        </div>
        <p class="text-xs font-semibold text-brand-text-muted">
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
              <span class="ml-auto text-[0.6rem] font-semibold text-brand-text-muted">—</span>
            </div>
          {/each}
        </div>
      </div>
    </section>

    <!-- ─── Host a new game (game tile grid) ────────────────── -->
    <section class="flex flex-col gap-5">
      <div use:reveal={{ delay: 2 }} class="flex items-center justify-between">
        <div>
          <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
            Host a new game
          </p>
          <h2 class="text-2xl font-bold">Pick a game type</h2>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
        {#each data.gameTypes as gt, i}
          <a
            href={`/host?game_type=${gt.slug}`}
            use:reveal={{ delay: i + 3 }}
            use:physCard
            use:hoverEffect={'gradient'}
            class="group rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6 flex flex-col gap-3 cursor-pointer"
            style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
          >
            <div class="inline-flex items-center gap-2 text-lg font-bold">
              <Sparkles size={18} strokeWidth={2.5} />
              {gt.name}
            </div>
            {#if gt.description}
              <p class="text-sm font-semibold text-brand-text-muted line-clamp-3">{gt.description}</p>
            {/if}
            <div class="mt-auto text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
              Host this →
            </div>
          </a>
        {/each}
      </div>
    </section>
  </div>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">Privacy Policy</a>
</footer>
