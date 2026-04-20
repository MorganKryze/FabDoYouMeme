<script lang="ts">
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowRight } from '$lib/icons';
  import type { Submission, LeaderboardEntry } from '$lib/api/types';
  import RoundReveal from './RoundReveal.svelte';

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
  <RoundReveal {submissions} {leaderboard} />

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
