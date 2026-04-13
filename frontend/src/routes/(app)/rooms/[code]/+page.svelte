<script lang="ts">
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { guest } from '$lib/state/guest.svelte';
  import { stage } from '$lib/motion/stage.svelte';
  import MemeCaptionSubmitForm from '$lib/games/meme-caption/SubmitForm.svelte';
  import MemeCaptionVoteForm from '$lib/games/meme-caption/VoteForm.svelte';
  import MemeCaptionResultsView from '$lib/games/meme-caption/ResultsView.svelte';
  import WaitingStage from '$lib/components/room/WaitingStage.svelte';
  import EndStage from '$lib/components/room/EndStage.svelte';

  // Player identity — registered users match via user.id; guest identity
  // is recovered from sessionStorage (set by the /join flow in F4).
  const playerId = $derived.by(() => {
    if (user.id) return user.id;
    return room.code ? guest.playerId(room.code) : null;
  });

  const isHost = $derived(
    playerId !== null && room.hostUserId === playerId
  );

  let countdown = $state<number | null>(null);
  let countdownInterval: ReturnType<typeof setInterval> | null = null;
  let prefersReducedMotion = $state(false);

  $effect(() => {
    if (typeof window !== 'undefined') {
      prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    }
  });

  function startCountdown() {
    countdown = 3;
    countdownInterval = setInterval(() => {
      if (countdown === null) return;
      if (countdown <= 0) {
        clearInterval(countdownInterval!);
        countdown = null;
      } else {
        countdown--;
      }
    }, 1000);
  }

  $effect(() => {
    if (room.phase === 'countdown') {
      startCountdown();
    }
  });

  $effect(() => {
    stage.sync(room.phase);
  });

  function nextRound() {
    ws.send('next_round');
  }
</script>

<div class="p-6 flex flex-col gap-6">

  <!-- Countdown overlay -->
  {#if countdown !== null}
    <div class="fixed inset-0 z-40 flex items-center justify-center" style="background: rgba(0,0,0,0.6); backdrop-filter: blur(8px);">
      <div
        class="font-bold tabular-nums text-brand-white {prefersReducedMotion ? '' : 'animate-bounce'}"
        style="font-size: clamp(6rem, 20vw, 12rem); line-height: 1;"
        aria-live="assertive"
      >
        {countdown > 0 ? countdown : 'GO!'}
      </div>
    </div>
  {/if}

  <!-- Stage-wrapped phase branches (L1c) -->
  <div class="stage-wrap" class:hidden={!stage.visible}>
    <!-- Waiting stage: lobby -->
    {#if stage.displayPhase === 'idle' && room.state === 'lobby'}
      <WaitingStage {isHost} />

    <!-- Submission phase -->
    {:else if stage.displayPhase === 'submitting' && room.currentRound}
      <MemeCaptionSubmitForm round={room.currentRound} />

    <!-- Voting phase -->
    {:else if stage.displayPhase === 'voting'}
      <MemeCaptionVoteForm submissions={room.submissions} />

    <!-- Results phase -->
    {:else if stage.displayPhase === 'results'}
      <MemeCaptionResultsView
        submissions={room.submissions}
        leaderboard={room.leaderboard}
        {isHost}
        onNextRound={nextRound}
      />

    <!-- End stage: game finished, rematch available -->
    {:else if room.state === 'finished'}
      <EndStage {isHost} />
    {/if}
  </div>
</div>
