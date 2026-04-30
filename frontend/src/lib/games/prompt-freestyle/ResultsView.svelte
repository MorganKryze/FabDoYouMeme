<script lang="ts">
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowRight } from '$lib/icons';
  import type { Submission, LeaderboardEntry, Round } from '$lib/api/types';
  import RoundReveal from './RoundReveal.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    submissions,
    leaderboard,
    isHost,
    hostPaced,
    onNextRound,
    round = null,
  }: {
    submissions: Submission[];
    leaderboard: LeaderboardEntry[];
    isHost: boolean;
    hostPaced: boolean;
    onNextRound: () => void;
    round?: Round | null;
  } = $props();
</script>

<div class="flex flex-col gap-6">
  <RoundReveal {submissions} {leaderboard} {round} />

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
        {m.game_next_round()}
      </button>
    {:else}
      <p class="text-center text-sm font-bold text-brand-text-muted m-0">
        {m.game_waiting_for_host_continue()}
      </p>
    {/if}
  {/if}
</div>
