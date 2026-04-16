<script lang="ts">
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Trophy, ArrowRight } from '$lib/icons';
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
</script>

<div class="flex flex-col gap-6">
  <h3 class="stage-title">Round Results</h3>

  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
    {#each submissions as sub, i}
      <div
        use:physCard
        use:reveal={{ delay: i }}
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-2 cursor-default"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <p class="text-sm font-bold leading-relaxed">{sub.caption}</p>
        <div class="flex items-center gap-2 text-xs font-semibold text-brand-text-muted mt-auto">
          <span class="font-bold text-brand-text">{sub.username}</span>
          <span>&middot;</span>
          <span>{sub.votes_received ?? 0} vote{(sub.votes_received ?? 0) !== 1 ? 's' : ''}</span>
          {#if (sub.points_awarded ?? 0) > 0}
            <span
              class="ml-auto font-bold rounded-full bg-brand-text text-brand-white px-3 py-0.5 text-xs"
            >
              +{sub.points_awarded}
            </span>
          {/if}
        </div>
      </div>
    {/each}
  </div>

  <div use:reveal={{ delay: 2 }}>
    <h4 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-3 inline-flex items-center gap-2">
      <Trophy size={12} strokeWidth={2.5} />
      Leaderboard
    </h4>
    <div
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <ol class="flex flex-col gap-2">
        {#each leaderboard as entry, i}
          <li
            class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border bg-brand-white px-4 py-2.5 text-sm"
          >
            <span class="w-5 text-right font-bold text-brand-text-muted">{i + 1}.</span>
            <span class="flex-1 font-bold">{entry.display_name}</span>
            <span class="font-bold tabular-nums">{entry.score} pts</span>
          </li>
        {/each}
      </ol>
    </div>
  </div>

  {#if hostPaced}
    {#if isHost}
      <button
        use:pressPhysics={'primary'}
        use:hoverEffect={'rainbow'}
        type="button"
        onclick={onNextRound}
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <ArrowRight size={18} strokeWidth={2.5} />
        Next Round
      </button>
    {:else}
      <p class="text-center text-sm font-bold text-brand-text-muted">Waiting for host to continue…</p>
    {/if}
  {/if}
</div>
