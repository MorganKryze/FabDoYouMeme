<script lang="ts">
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
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

  <!-- Lobby phase -->
  {#if room.phase === 'idle' && room.state === 'lobby'}
    <div class="flex flex-col items-center gap-8 text-center" use:reveal>
      <p class="text-sm font-semibold text-brand-text-mid">
        {#if isHost}
          Waiting for players to join. Start when ready.
        {:else}
          Waiting for {room.players.find((p) => p.is_host)?.username ?? 'host'} to start…
        {/if}
      </p>

      {#if room.gameType}
        <MemeCaptionGameRules gameType={room.gameType} />
      {/if}

      {#if isHost}
        <div class="flex flex-col gap-3 w-full max-w-xs">
          <button
            use:pressPhysics={'dark'}
            type="button"
            onclick={startGame}
            disabled={room.players.length < 2}
            class="h-13 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold text-lg disabled:opacity-50 transition-colors cursor-pointer"
          >
            Start Game
          </button>
          {#if room.players.length < 2}
            <p class="text-xs font-semibold text-brand-text-muted">Need at least 2 players to start.</p>
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
    <div class="flex flex-col items-center gap-6 text-center" use:reveal>
      <h2 style="font-size: clamp(2.5rem, 6vw, 4rem); font-weight: 700; line-height: 1; letter-spacing: -0.03em;">
        Game Over
      </h2>

      <div
        class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 w-full max-w-sm"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
      >
        <ol class="flex flex-col gap-2">
          {#each room.leaderboard as entry, i}
            <li
              class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border bg-brand-white px-4 py-2.5 text-sm"
            >
              <span class="w-6 font-bold text-brand-text-muted text-right">
                {i === 0 ? '\u{1F947}' : i === 1 ? '\u{1F948}' : i === 2 ? '\u{1F949}' : `${i + 1}.`}
              </span>
              <span class="flex-1 text-left font-bold">{entry.username}</span>
              <span class="font-bold tabular-nums text-brand-text-mid">{entry.total_score} pts</span>
            </li>
          {/each}
        </ol>
      </div>

      <a
        href="/"
        class="inline-flex items-center h-11 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-sm font-bold transition-colors hover:bg-brand-surface"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
      >
        Back to Lobby
      </a>
    </div>
  {/if}
</div>
