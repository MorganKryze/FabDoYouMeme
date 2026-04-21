<script lang="ts">
  import { fade } from 'svelte/transition';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { reveal } from '$lib/actions/reveal';
  import { ArrowLeft, ArrowRight } from '$lib/icons';
  import MemeFreestyleReplay from '$lib/games/meme-freestyle/ReplayRoundView.svelte';
  import MemeShowdownReplay from '$lib/games/meme-showdown/ReplayRoundView.svelte';
  import type { PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const replay = $derived(data.replay);
  const rounds = $derived(replay.rounds);
  const hasRounds = $derived(rounds.length > 0);

  let idx = $state(0);
  const current = $derived(hasRounds ? rounds[idx] : null);

  function prev() { if (idx > 0) idx -= 1; }
  function next() { if (idx < rounds.length - 1) idx += 1; }
  function jumpTo(i: number) { idx = Math.max(0, Math.min(rounds.length - 1, i)); }

  function onKey(e: KeyboardEvent) {
    if (e.target instanceof HTMLElement && ['INPUT', 'TEXTAREA'].includes(e.target.tagName)) return;
    if (e.key === 'ArrowLeft')  { prev(); e.preventDefault(); }
    if (e.key === 'ArrowRight') { next(); e.preventDefault(); }
    if (e.key === 'Home')       { idx = 0; e.preventDefault(); }
    if (e.key === 'End')        { idx = rounds.length - 1; e.preventDefault(); }
  }

  function prettyGameSlug(slug: string): string {
    return slug.split('-').map((s) => s[0]?.toUpperCase() + s.slice(1)).join(' ');
  }

  function formatRelative(iso?: string): string {
    if (!iso) return '';
    const diff = Date.now() - new Date(iso).getTime();
    const minutes = Math.floor(diff / 60_000);
    if (minutes < 1) return m.games_relative_just_now();
    if (minutes < 60) return m.games_relative_minutes({ minutes });
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return m.games_relative_hours({ hours });
    const days = Math.floor(hours / 24);
    if (days < 7) return m.games_relative_days({ days });
    return new Date(iso).toLocaleDateString();
  }

  function formatDuration(start?: string, end?: string): string {
    if (!start || !end) return '';
    const ms = new Date(end).getTime() - new Date(start).getTime();
    const mins = Math.round(ms / 60_000);
    if (mins < 1) return m.replay_duration_short();
    if (mins < 60) return m.replay_duration_minutes({ minutes: mins });
    return m.replay_duration_hours_minutes({ hours: Math.floor(mins / 60), minutes: mins % 60 });
  }
</script>

<svelte:head>
  <title>{m.replay_page_title({ code: replay.room.code })}</title>
</svelte:head>

<svelte:window onkeydown={onKey} />

<div class="flex-1 flex justify-center p-6 pt-4 pb-10">
  <div class="w-full max-w-3xl flex flex-col gap-6">
    <a
      href="/games"
      class="inline-flex items-center gap-2 text-xs font-bold text-brand-text-mid hover:text-brand-text self-start"
    >
      <ArrowLeft size={14} strokeWidth={2.5} /> {m.replay_back_all_games()}
    </a>

    <section use:reveal class="flex flex-col gap-2">
      <p class="text-xs font-bold uppercase tracking-[0.3em] text-brand-text-mid m-0">
        {m.replay_eyebrow({ code: replay.room.code })}
      </p>
      <h1 class="text-3xl font-bold m-0">{prettyGameSlug(replay.room.game_type_slug)}</h1>
      <p class="text-sm font-semibold text-brand-text-mid m-0">
        {replay.room.pack_name}
        {#if replay.room.text_pack_name}· {replay.room.text_pack_name}{/if}
        · {m.replay_subtitle_ended_prefix()} {formatRelative(replay.room.finished_at ?? replay.room.started_at)}
        {#if formatDuration(replay.room.started_at, replay.room.finished_at)}
          · {formatDuration(replay.room.started_at, replay.room.finished_at)}
        {/if}
        · {replay.room.player_count === 1
            ? m.replay_subtitle_players_one({ count: replay.room.player_count })
            : m.replay_subtitle_players_other({ count: replay.room.player_count })}
      </p>
    </section>

    {#if !hasRounds}
      <section
        use:reveal
        class="rounded-[22px] border-[2.5px] border-dashed border-brand-border-heavy bg-brand-white/40 p-8 text-center"
      >
        <p class="text-sm font-bold m-0">{m.replay_empty_title()}</p>
        <p class="text-xs font-semibold text-brand-text-mid mt-2 m-0">
          {m.replay_empty_body()}
        </p>
      </section>
    {:else}
      <section class="flex items-center justify-between gap-4">
        <span class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid">
          {m.replay_round_of({ current: idx + 1, total: rounds.length })}
        </span>
        <div class="flex items-center gap-1.5">
          {#each rounds as _, i}
            <button
              type="button"
              onclick={() => jumpTo(i)}
              aria-label={m.replay_jump_aria({ round: i + 1 })}
              class="h-2.5 w-2.5 rounded-full border-[2px] border-brand-border-heavy transition-transform cursor-pointer"
              style="background: {i === idx ? 'var(--brand-accent)' : i < idx ? 'var(--brand-text)' : 'var(--brand-white)'}; transform: {i === idx ? 'scale(1.3)' : 'none'};"
            ></button>
          {/each}
        </div>
      </section>

      {#key idx}
        <div in:fade={{ duration: 180 }}>
          {#if replay.room.game_type_slug === 'meme-freestyle' && current}
            <MemeFreestyleReplay round={current} leaderboard={replay.leaderboard} />
          {:else if replay.room.game_type_slug === 'meme-showdown' && current}
            <MemeShowdownReplay round={current} leaderboard={replay.leaderboard} />
          {/if}
        </div>
      {/key}

      <nav class="grid grid-cols-2 gap-3">
        <button
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          type="button"
          onclick={prev}
          disabled={idx === 0}
          class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold inline-flex items-center justify-center gap-2 disabled:opacity-40 cursor-pointer"
        >
          <ArrowLeft size={16} strokeWidth={2.5} />
          {idx === 0 ? m.replay_nav_start() : m.replay_nav_round({ round: idx })}
        </button>
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="button"
          onclick={next}
          disabled={idx === rounds.length - 1}
          class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold inline-flex items-center justify-center gap-2 disabled:opacity-40 cursor-pointer"
        >
          {#if idx === rounds.length - 1}
            {m.replay_nav_end()}
          {:else}
            {m.replay_nav_round({ round: idx + 2 })}
            <ArrowRight size={16} strokeWidth={2.5} />
          {/if}
        </button>
      </nav>
    {/if}
  </div>
</div>
