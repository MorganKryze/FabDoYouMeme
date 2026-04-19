<script lang="ts">
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowRight } from '$lib/icons';
  import type { Submission, LeaderboardEntry } from '$lib/api/types';

  let {
    submissions,
    leaderboard,
    isHost,
    hostPaced,
    onNextRound,
  }: {
    submissions: Submission[];
    leaderboard: LeaderboardEntry[];
    isHost: boolean;
    /** When true the host manually advances to the next round; when false the server auto-advances. */
    hostPaced: boolean;
    onNextRound: () => void;
  } = $props();

  const ranked = $derived(
    [...submissions].sort((a, b) => (b.votes_received ?? 0) - (a.votes_received ?? 0))
  );

  const podiumShape = $derived.by<'solo' | 'duet' | 'trio' | 'none'>(() => {
    if (ranked.length === 0) return 'none';
    if (ranked.length === 1) return 'solo';
    const firstVotes = ranked[0].votes_received ?? 0;
    const secondVotes = ranked[1].votes_received ?? 0;
    if (ranked.length === 2) return 'duet';
    if (firstVotes === secondVotes) return 'duet';
    return 'trio';
  });

  const podium = $derived.by(() => {
    if (podiumShape === 'trio') {
      return [
        { sub: ranked[1], place: 2 as const },
        { sub: ranked[0], place: 1 as const },
        { sub: ranked[2], place: 3 as const },
      ];
    }
    if (podiumShape === 'duet') return [
      { sub: ranked[0], place: 1 as const },
      { sub: ranked[1], place: 1 as const },
    ];
    if (podiumShape === 'solo') return [{ sub: ranked[0], place: 1 as const }];
    return [];
  });

  const alsoRans = $derived(
    podiumShape === 'trio'
      ? ranked.slice(3)
      : podiumShape === 'duet'
        ? ranked.slice(2)
        : podiumShape === 'solo'
          ? ranked.slice(1)
          : []
  );

  function placeLabel(place: 1 | 2 | 3): string {
    return place === 1 ? '1st' : place === 2 ? '2nd' : '3rd';
  }

  function medalClass(place: 1 | 2 | 3): string {
    return place === 1 ? 'medal gold' : place === 2 ? 'medal silver' : 'medal bronze';
  }
</script>

<div class="flex flex-col gap-6">
  {#if podiumShape !== 'none'}
    <div
      use:reveal={{ delay: 0 }}
      class="flex flex-col gap-3"
    >
      <div class="text-center">
        <span class="text-[10px] font-bold uppercase tracking-[0.25em] text-brand-text-muted">
          Round results
        </span>
      </div>

      <div
        class="grid gap-4 items-end"
        class:grid-cols-1={podiumShape === 'solo'}
        class:sm:grid-cols-2={podiumShape === 'duet'}
        class:sm:grid-cols-3={podiumShape === 'trio'}
      >
        {#each podium as entry, i (entry.sub.id + '-' + entry.place + '-' + i)}
          {@const isFirst = entry.place === 1 && podiumShape === 'trio'}
          {@const votes = entry.sub.votes_received ?? 0}
          {@const points = entry.sub.points_awarded ?? 0}
          <div
            use:physCard
            use:reveal={{ delay: i + 1 }}
            class="relative rounded-[22px] border-[2.5px] bg-brand-white p-4 flex flex-col gap-2.5 cursor-default"
            style="
              border-color: {isFirst ? 'var(--brand-accent)' : 'var(--brand-border-heavy)'};
              box-shadow: {isFirst ? '0 8px 0 var(--brand-accent)' : '0 5px 0 rgba(0,0,0,0.10)'};
              transform: {isFirst ? 'scale(1.06)' : 'none'};
            "
          >
            {#if isFirst && points > 0}
              <span
                class="absolute -top-3 left-1/2 z-10 inline-flex items-center gap-1 rounded-full border-[2.5px] border-brand-border-heavy px-3 py-0.5 text-[10px] font-bold tracking-[0.18em] uppercase whitespace-nowrap"
                style="transform: translateX(-50%); background: var(--brand-accent); color: #1A1A1A; box-shadow: 0 2px 0 rgba(0,0,0,0.2);"
              >
                +{points} pts
              </span>
            {/if}

            <div class="flex items-center justify-between gap-2">
              <span class={medalClass(entry.place)} aria-label="{placeLabel(entry.place)} place">
                {entry.place}
              </span>
              <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
                {votes} vote{votes !== 1 ? 's' : ''}
              </span>
            </div>

            <div
              class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-3 py-2.5 text-sm font-bold text-brand-text leading-snug text-balance flex-1"
              style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
            >
              {entry.sub.text ?? ''}
            </div>

            <div class="flex items-center justify-between gap-2">
              <span class="text-sm font-bold text-brand-text truncate">
                {entry.sub.username}
              </span>
              {#if !isFirst && points > 0}
                <span
                  class="font-bold rounded-full px-2.5 py-0.5 text-[10px] tracking-[0.15em] uppercase border-[2px] border-brand-border-heavy"
                  style="background: var(--brand-accent-2); color: #1A1A1A;"
                >
                  +{points}
                </span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if alsoRans.length > 0}
    <div use:reveal={{ delay: 4 }} class="flex flex-col gap-2.5">
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1">
        Also in the hand
      </span>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {#each alsoRans as sub (sub.id)}
          {@const votes = sub.votes_received ?? 0}
          {@const points = sub.points_awarded ?? 0}
          <div
            use:physCard
            class="rounded-[20px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-3 flex flex-col gap-2 cursor-default"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
          >
            <div
              class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2 text-sm font-bold text-brand-text leading-snug"
            >
              {sub.text ?? ''}
            </div>
            <div class="flex items-center gap-2 text-[11px] font-bold text-brand-text-muted">
              <span class="text-brand-text">{sub.username}</span>
              <span>·</span>
              <span>{votes} vote{votes !== 1 ? 's' : ''}</span>
              {#if points > 0}
                <span class="ml-auto font-mono tabular-nums text-brand-text">+{points}</span>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if leaderboard.length > 0}
    <div use:reveal={{ delay: 5 }} class="flex flex-col gap-2.5">
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1 inline-flex items-center gap-2">
        <span style="color: var(--brand-accent);">♠</span>
        Leaderboard
      </span>
      <ol class="flex flex-col gap-1.5 m-0 p-0 list-none">
        {#each leaderboard as entry, i (entry.player_id)}
          {@const isFirst = i === 0}
          <li
            class="l-row flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy px-3 py-1.5 text-sm font-bold"
            style="background: {isFirst ? 'var(--brand-accent)' : 'var(--brand-white)'}; color: #1A1A1A;"
          >
            <span class="font-mono tabular-nums text-[11px] font-bold w-5 text-right opacity-70">
              {entry.rank}
            </span>
            <span class="flex-1 truncate">{entry.display_name}</span>
            <span class="font-mono tabular-nums text-xs font-bold">
              {entry.score}
            </span>
          </li>
        {/each}
      </ol>
    </div>
  {/if}

  {#if hostPaced}
    {#if isHost}
      <button
        use:pressPhysics={'primary'}
        use:hoverEffect={'rainbow'}
        type="button"
        onclick={onNextRound}
        class="h-12 mx-auto px-8 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <ArrowRight size={18} strokeWidth={2.5} />
        Next Round
      </button>
    {:else}
      <p class="text-center text-sm font-bold text-brand-text-muted m-0">
        Waiting for host to continue…
      </p>
    {/if}
  {/if}
</div>
