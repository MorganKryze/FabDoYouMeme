<script lang="ts">
  import { untrack } from 'svelte';
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import EndRoomButton from '$lib/components/room/EndRoomButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { totalRounds = null }: { totalRounds?: number | null } = $props();

  const showEndRoom = $derived(
    room.state === 'playing' && user.id !== null && room.hostUserId === user.id
  );

  // Phase-aware deadline. Falls back to 0 when no timer applies — during
  // results the deadline is only present in server-paced mode; host-paced
  // mode has no deadline (the host advances manually).
  const deadline = $derived(
    room.phase === 'submitting'
      ? (room.currentRound?.ends_at ? Date.parse(room.currentRound.ends_at) : 0)
      : room.phase === 'voting'
        ? (room.votingEndsAt ? Date.parse(room.votingEndsAt) : 0)
        : room.phase === 'results'
          ? (room.resultsEndsAt ? Date.parse(room.resultsEndsAt) : 0)
          : 0
  );

  let timerMs = $state(untrack(() => Math.max(0, deadline - Date.now())));

  $effect(() => {
    if (!deadline) {
      timerMs = 0;
      return;
    }
    const tick = () => {
      if (room.roundPaused) return; // freeze while paused
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const secondsLeft = $derived(Math.ceil(timerMs / 1000));
  const mm = $derived(Math.floor(secondsLeft / 60).toString().padStart(2, '0'));
  const ss = $derived((secondsLeft % 60).toString().padStart(2, '0'));
  const showCountdown = $derived(
    (room.phase === 'submitting' || room.phase === 'voting' || room.phase === 'results') &&
      deadline > 0
  );
  const isWarn = $derived(showCountdown && !room.roundPaused && secondsLeft <= 10);
  const isLarge = $derived(room.phase === 'submitting');

  const roundNumber = $derived(room.currentRound?.round_number ?? null);
  const showRoundPills = $derived(roundNumber !== null && totalRounds !== null && totalRounds > 0);
  const codeLetters = $derived((room.code ?? '').split(''));
  const phaseLabel = $derived(
    room.phase === 'submitting'
      ? m.room_phase_submitting()
      : room.phase === 'voting'
        ? m.room_phase_voting()
        : room.phase === 'results'
          ? m.room_phase_results()
          : ''
  );
  // During results with a live deadline, prefix the countdown so players see
  // "Next round 00:08" instead of a bare 00:08 that could be mistaken for the
  // current phase's remaining time.
  const countdownPrefix = $derived(
    room.phase === 'results' && deadline > 0 ? m.room_countdown_next_round() : ''
  );
</script>

<section
  class="room-header sticky top-4 z-20 mx-auto w-full max-w-[1280px] flex flex-wrap items-center justify-between gap-3 px-4 py-2.5 md:py-3 rounded-[22px] md:rounded-full bg-brand-white border-[2.5px] border-brand-border-heavy"
  style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
  aria-label={m.room_header_aria()}
>
  <!-- LEFT: code chip + game + pack -->
  <div class="flex items-center gap-3 min-w-0">
    {#if codeLetters.length > 0}
      <div class="inline-flex gap-1 shrink-0" aria-label={m.room_code_aria({ code: room.code ?? '' })}>
        {#each codeLetters as letter}
          <span
            class="inline-grid place-items-center w-8 h-10 rounded-[8px] border-[2.5px] border-brand-border-heavy bg-brand-white font-mono font-bold text-[15px]"
            style="box-shadow: 0 2px 0 rgba(0,0,0,0.18), inset 0 1.5px 0 rgba(255,255,255,0.8);"
          >
            {letter}
          </span>
        {/each}
      </div>
    {/if}
    {#if room.gameType}
      <div class="flex flex-col min-w-0">
        <span class="text-sm font-bold leading-tight truncate">
          {room.gameType.name}
        </span>
        <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid leading-tight truncate">
          {phaseLabel}
        </span>
      </div>
    {/if}
  </div>

  <!-- CENTER: round progress -->
  {#if roundNumber !== null}
    <div class="flex items-center gap-3.5">
      <span class="text-[13px] font-bold uppercase tracking-[0.22em] text-brand-text">
        {#if totalRounds}
          {m.room_rounds_info_of({ number: roundNumber, total: totalRounds })}
        {:else}
          {m.room_rounds_info({ number: roundNumber })}
        {/if}
      </span>
      {#if showRoundPills}
        <div class="inline-flex gap-1.5" aria-hidden="true">
          {#each Array(totalRounds) as _, i}
            {@const n = i + 1}
            <span
              class="round-pill"
              class:done={n < (roundNumber ?? 0)}
              class:active={n === roundNumber}
            ></span>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <!-- RIGHT: timer + optional end-room control (host only, during play) -->
  <div class="flex items-center gap-2">
  {#if showEndRoom}
    <EndRoomButton compact />
  {/if}
  <div
    class="timer inline-flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy px-5 py-2.5 transition-colors"
    class:large={isLarge && showCountdown}
    class:warn={isWarn}
    style="background: {isWarn ? 'var(--brand-accent)' : 'var(--brand-text)'}; color: {isWarn ? 'var(--brand-text)' : 'var(--brand-white)'}; box-shadow: 0 5px 0 rgba(0,0,0,{isWarn ? '0.22' : '0.35'});"
    role="timer"
    aria-label={room.roundPaused ? m.room_timer_paused_aria() : (showCountdown ? m.room_timer_remaining_aria() : phaseLabel)}
  >
    <span
      class="h-3 w-3 rounded-full shrink-0"
      style="background: {isWarn ? 'var(--brand-text)' : 'var(--brand-accent)'}; animation: {room.roundPaused || !showCountdown ? 'none' : 'pulse-dot 1.2s ease-in-out infinite'};"
    ></span>
    {#if room.roundPaused}
      <span class="text-lg font-bold uppercase tracking-[0.2em] tabular-nums">{m.room_timer_paused()}</span>
    {:else if showCountdown}
      {#if countdownPrefix}
        <span class="text-[11px] font-bold uppercase tracking-[0.2em] opacity-80">{countdownPrefix}</span>
      {/if}
      <span class="font-bold tabular-nums leading-none {isLarge ? 'text-4xl' : 'text-2xl'}">
        {mm}:{ss}
      </span>
    {:else}
      <span class="text-base font-bold uppercase tracking-[0.2em]">{phaseLabel}</span>
    {/if}
  </div>
  </div>
</section>

<style>
  /* Scoped — the pill needs to read clearly against the gradient behind it. */
  .room-header { backdrop-filter: saturate(1.1); }
</style>
