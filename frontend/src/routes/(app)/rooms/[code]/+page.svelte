<script lang="ts">
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import MemeCaptionSubmitForm from '$lib/games/meme-caption/SubmitForm.svelte';
  import MemeCaptionVoteForm from '$lib/games/meme-caption/VoteForm.svelte';
  import MemeCaptionResultsView from '$lib/games/meme-caption/ResultsView.svelte';
  import MemeCaptionGameRules from '$lib/games/meme-caption/GameRules.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  const isHost = $derived(
    room.players.find((p) => p.user_id === user.id)?.is_host ?? false
  );

  let countdown = $state<number | null>(null);
  let countdownInterval: ReturnType<typeof setInterval> | null = null;

  function startCountdown() {
    countdown = 3;
    countdownInterval = setInterval(() => {
      if (countdown === null) return;
      if (countdown <= 0) {
        clearInterval(countdownInterval!);
        countdown = null;
        room.phase = 'submitting';
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

  async function kickPlayer(userId: string) {
    await fetch(`/api/rooms/${data.room.code}/kick`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    });
  }

  function startGame() {
    ws.send('start');
  }

  function nextRound() {
    ws.send('next_round');
  }
</script>

<div class="p-6 flex flex-col gap-6">

  <!-- Countdown overlay -->
  {#if countdown !== null}
    <div class="fixed inset-0 bg-background/90 z-40 flex items-center justify-center">
      <div class="text-9xl font-black animate-bounce tabular-nums" aria-live="assertive">
        {countdown > 0 ? countdown : 'GO!'}
      </div>
    </div>
  {/if}

  <!-- Lobby phase -->
  {#if room.phase === 'idle' && room.state === 'lobby'}
    <div class="flex flex-col items-center gap-8 text-center">
      <div class="text-muted-foreground">
        {#if isHost}
          Waiting for players to join. Start when ready.
        {:else}
          Waiting for {room.players.find((p) => p.is_host)?.username ?? 'host'} to start…
        {/if}
      </div>

      {#if room.gameType}
        <MemeCaptionGameRules gameType={room.gameType} />
      {/if}

      {#if isHost}
        <div class="flex flex-col gap-3 w-full max-w-xs">
          <button
            type="button"
            onclick={startGame}
            disabled={room.players.length < 2}
            class="h-12 rounded-lg bg-primary text-primary-foreground font-semibold text-lg disabled:opacity-50 hover:bg-primary/90 transition-colors"
          >
            Start Game ▶
          </button>
          {#if room.players.length < 2}
            <p class="text-xs text-muted-foreground">Need at least 2 players to start.</p>
          {/if}
        </div>
      {/if}
    </div>

  <!-- Submission phase -->
  {:else if room.phase === 'submitting' && room.currentRound}
    <MemeCaptionSubmitForm round={room.currentRound} />

  <!-- Voting phase -->
  {:else if room.phase === 'voting'}
    <MemeCaptionVoteForm submissions={room.submissions} />

  <!-- Results phase -->
  {:else if room.phase === 'results'}
    <MemeCaptionResultsView
      submissions={room.submissions}
      leaderboard={room.leaderboard}
      {isHost}
      onNextRound={nextRound}
    />

  <!-- Game ended -->
  {:else if room.state === 'finished'}
    <div class="flex flex-col items-center gap-6 text-center">
      <h2 class="text-3xl font-bold">🏆 Game Over</h2>
      <ol class="flex flex-col gap-2 w-full max-w-xs">
        {#each room.leaderboard as entry, i}
          <li class="flex items-center gap-3 text-lg">
            <span class="w-6 text-muted-foreground text-right">
              {i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : `${i + 1}.`}
            </span>
            <span class="flex-1 text-left font-medium">{entry.username}</span>
            <span class="text-muted-foreground">{entry.score} pts</span>
          </li>
        {/each}
      </ol>
      <div class="flex gap-3">
        <a
          href="/"
          class="h-10 px-6 rounded-lg border border-border text-sm font-medium flex items-center hover:bg-muted transition-colors"
        >
          Back to Lobby
        </a>
      </div>
    </div>
  {/if}
</div>
