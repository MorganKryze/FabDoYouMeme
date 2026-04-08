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
