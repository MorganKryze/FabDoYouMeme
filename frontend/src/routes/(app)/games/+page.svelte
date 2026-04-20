<script lang="ts">
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Play, Clock, ArrowLeft } from '$lib/icons';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  function prettyGameSlug(slug: string): string {
    return slug.split('-').map((s) => s[0]?.toUpperCase() + s.slice(1)).join(' ');
  }

  function formatRelative(iso: string): string {
    const diff = Date.now() - new Date(iso).getTime();
    const minutes = Math.floor(diff / 60_000);
    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    if (days < 7) return `${days}d ago`;
    return new Date(iso).toLocaleDateString();
  }
</script>

<svelte:head><title>All games — FabDoYouMeme</title></svelte:head>

<div class="flex-1 flex justify-center p-6 pt-4 pb-10">
  <div class="w-full max-w-3xl flex flex-col gap-6">
    <a
      href="/home"
      class="inline-flex items-center gap-2 text-xs font-bold text-brand-text-mid hover:text-brand-text self-start"
    >
      <ArrowLeft size={14} strokeWidth={2.5} /> Home
    </a>

    <section use:reveal class="flex items-center gap-2">
      <Clock size={18} strokeWidth={2.5} />
      <h1 class="text-2xl font-bold m-0">All games</h1>
    </section>

    {#if data.rooms.length === 0}
      <div class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 text-center flex flex-col gap-3 items-center">
        <p class="text-sm font-bold m-0">No games yet.</p>
        <p class="text-xs font-semibold text-brand-text-mid max-w-xs m-0">
          Your history lands here after the first round.
        </p>
        <a
          href="/host"
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          class="inline-flex items-center gap-2 px-5 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold no-underline text-xs"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          <Play size={14} strokeWidth={2.5} />
          Host your first room
        </a>
      </div>
    {:else}
      <ul class="flex flex-col gap-2 list-none p-0 m-0">
        {#each data.rooms as room (room.code)}
          <li>
            <a
              href={`/games/${room.code}`}
              use:pressPhysics={'ghost'}
              class="flex items-center justify-between gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2.5 text-sm no-underline text-brand-text focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
            >
              <div class="flex items-center gap-3 min-w-0 flex-1">
                <span class="font-mono font-bold tracking-widest text-xs inline-flex items-center justify-center rounded-full bg-brand-surface border-[2.5px] border-brand-border-heavy px-2.5 h-8">
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
                  <div class="text-xs font-bold uppercase tracking-[0.15em] text-brand-text-mid">Rank</div>
                  <div class="font-bold text-sm tabular-nums">
                    {room.rank}<span class="text-brand-text-mid">/{room.player_count}</span>
                  </div>
                </div>
                <div class="text-right">
                  <div class="text-xs font-bold uppercase tracking-[0.15em] text-brand-text-mid">Score</div>
                  <div class="font-bold text-sm tabular-nums">{room.score}</div>
                </div>
              </div>
            </a>
          </li>
        {/each}
      </ul>

      {#if data.nextCursor}
        <a
          href={`/games?after=${data.nextCursor}`}
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          class="self-center inline-flex items-center gap-2 px-6 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold no-underline text-xs"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          Load more →
        </a>
      {/if}
    {/if}
  </div>
</div>
