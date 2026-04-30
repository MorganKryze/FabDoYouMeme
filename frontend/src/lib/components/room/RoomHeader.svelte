<script lang="ts">
  import { untrack } from 'svelte';
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import EndRoomButton from '$lib/components/room/EndRoomButton.svelte';
  import RoomRail from '$lib/components/room/RoomRail.svelte';
  import { ChevronDown, Volume2, VolumeX } from '$lib/icons';
  import { music, MUSIC_LEVELS } from '$lib/state/music.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import * as m from '$lib/paraglide/messages';
  import { localizeGameType } from '$lib/i18n/gameType';

  let { totalRounds = null }: { totalRounds?: number | null } = $props();

  // Mobile-only collapsed/expanded state. Desktop ignores this — the full
  // header is always rendered there (the chevron and panel are `lg:hidden`).
  // Default collapsed: phone players see only round + timer; tapping the
  // chevron reveals the code chip, game type, and the roster sheet.
  let expanded = $state(false);

  // Music slider visibility — click-only (hover-based reveal closed too
  // eagerly on desktop the moment the pointer crossed the gap to the
  // slider). Two separate slots: one in the bottom strip on phone, one
  // in the right cluster on desktop, so each gets its own open flag and
  // anchor ref for click-outside dismissal.
  let musicOpenMobile = $state(false);
  let musicOpenDesktop = $state(false);
  let musicWrapMobile: HTMLDivElement | undefined = $state();
  let musicWrapDesktop: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (!musicOpenMobile) return;
    function onDocClick(e: MouseEvent) {
      if (musicWrapMobile && !musicWrapMobile.contains(e.target as Node)) {
        musicOpenMobile = false;
      }
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });

  $effect(() => {
    if (!musicOpenDesktop) return;
    function onDocClick(e: MouseEvent) {
      if (musicWrapDesktop && !musicWrapDesktop.contains(e.target as Node)) {
        musicOpenDesktop = false;
      }
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });

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
</script>

<section
  class="room-header sticky top-4 z-20 mx-auto w-full max-w-[1280px] flex flex-col gap-3 px-4 py-2.5 md:py-3 rounded-[22px] md:rounded-full bg-brand-white border-[2.5px] border-brand-border-heavy"
  style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
  aria-label={m.room_header_aria()}
>
  <div class="flex flex-wrap items-center justify-between gap-3">
    <!-- LEFT: code chip + game type. Hidden on phone when collapsed so the
         primary header is just round + timer; revealed alongside the roster
         when the player taps the expand chevron. -->
    <div class="{expanded ? 'flex' : 'hidden'} lg:flex items-center gap-3 min-w-0">
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
            {localizeGameType(room.gameType).name}
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
          <!-- Pills are decorative (textual count is alongside). At many-round
               configs (up to 30) the strip would punch off the right edge of
               a phone, so hide it below md and rely on the round-of-N text. -->
          <div class="hidden md:inline-flex flex-wrap gap-1.5 max-w-full" aria-hidden="true">
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
      {#if music.available}
        <!-- Desktop slot: music control in the right cluster. Hidden on
             phone (the timer + round-of-N already fills that row); the
             phone slot lives in the bottom strip below. Slider is
             right-anchored because the button sits near the right edge. -->
        <div bind:this={musicWrapDesktop} class="hidden lg:block relative">
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={() => (musicOpenDesktop = !musicOpenDesktop)}
            class="h-10 w-10 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid hover:text-brand-accent inline-flex items-center justify-center cursor-pointer transition-colors"
            title={music.playing ? m.room_music_mute_title() : m.room_music_play_title()}
            aria-label={m.room_music_volume_aria()}
            aria-expanded={musicOpenDesktop}
          >
            {#if music.playing && !music.muted}
              <Volume2 size={16} strokeWidth={2.5} />
            {:else}
              <VolumeX size={16} strokeWidth={2.5} />
            {/if}
          </button>
          {#if musicOpenDesktop}
            <div
              class="absolute right-0 top-full mt-2 z-30 bg-brand-white border-[2.5px] border-brand-border-heavy rounded-2xl p-2 flex items-center gap-1"
              style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
              role="group"
              aria-label={m.room_music_volume_aria()}
            >
              <button
                type="button"
                onclick={() => music.toggle()}
                class="h-8 w-8 mr-1 rounded-full border-[2px] border-brand-border-heavy {music.playing ? 'bg-brand-surface text-brand-text-mid' : 'bg-brand-text text-brand-white'} inline-flex items-center justify-center cursor-pointer"
                aria-label={music.playing ? m.room_music_mute_aria() : m.room_music_play_aria()}
                aria-pressed={!music.playing}
              >
                {#if music.playing}
                  <VolumeX size={14} strokeWidth={2.5} />
                {:else}
                  <Volume2 size={14} strokeWidth={2.5} />
                {/if}
              </button>
              {#each Array(MUSIC_LEVELS) as _, i (i)}
                {@const n = i + 1}
                {@const active = music.playing && music.level >= n}
                <button
                  type="button"
                  onclick={() => music.setLevel(n)}
                  disabled={!music.playing}
                  class="w-7 h-8 flex items-end justify-center cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                  aria-label={m.room_music_volume_level_aria({ level: n })}
                  aria-pressed={music.level === n}
                >
                  <span
                    class="block w-full rounded-sm transition-colors {active ? 'bg-brand-accent' : 'bg-brand-border'}"
                    style="height: {6 + i * 3}px;"
                  ></span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
      <div
        class="timer inline-flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy px-5 py-2.5 transition-colors"
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
          <span class="text-2xl font-bold tabular-nums leading-none">
            {mm}:{ss}
          </span>
        {:else}
          <span class="text-base font-bold uppercase tracking-[0.2em]">{phaseLabel}</span>
        {/if}
      </div>
    </div>
  </div>

  <!-- Phone-only expansion: roster sheet appears below the always-visible
       row when the chevron is tapped. Desktop has its own right rail and
       skips the toggle entirely. -->
  {#if expanded}
    <div class="lg:hidden border-t-[2px] border-brand-border pt-3">
      <RoomRail variant="sheet" />
    </div>
  {/if}

  <!-- Phone-only bottom strip — music control on the left, room-detail
       collapse/expand toggle centered. Splitting the music out of the
       round/timer row prevents that row from wrapping when the timer is
       in its large submission-phase form. -->
  <div class="lg:hidden -mx-1 -mb-0.5 flex items-center py-1">
    <!-- LEFT: music control -->
    <div class="w-16 flex items-center">
      {#if music.available}
        <div bind:this={musicWrapMobile} class="relative">
          <button
            type="button"
            onclick={() => (musicOpenMobile = !musicOpenMobile)}
            class="h-9 w-9 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid inline-flex items-center justify-center cursor-pointer"
            title={music.playing ? m.room_music_mute_title() : m.room_music_play_title()}
            aria-label={m.room_music_volume_aria()}
            aria-expanded={musicOpenMobile}
          >
            {#if music.playing && !music.muted}
              <Volume2 size={14} strokeWidth={2.5} />
            {:else}
              <VolumeX size={14} strokeWidth={2.5} />
            {/if}
          </button>
          {#if musicOpenMobile}
            <!-- Slider drops down + extends RIGHTWARD (left-anchored) so it
                 never overflows the left viewport edge. -->
            <div
              class="absolute left-0 top-full mt-2 z-30 bg-brand-white border-[2.5px] border-brand-border-heavy rounded-2xl p-2 flex items-center gap-1 whitespace-nowrap"
              style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
              role="group"
              aria-label={m.room_music_volume_aria()}
            >
              <button
                type="button"
                onclick={() => music.toggle()}
                class="h-8 w-8 mr-1 shrink-0 rounded-full border-[2px] border-brand-border-heavy {music.playing ? 'bg-brand-surface text-brand-text-mid' : 'bg-brand-text text-brand-white'} inline-flex items-center justify-center cursor-pointer"
                aria-label={music.playing ? m.room_music_mute_aria() : m.room_music_play_aria()}
                aria-pressed={!music.playing}
              >
                {#if music.playing}
                  <VolumeX size={14} strokeWidth={2.5} />
                {:else}
                  <Volume2 size={14} strokeWidth={2.5} />
                {/if}
              </button>
              {#each Array(MUSIC_LEVELS) as _, i (i)}
                {@const n = i + 1}
                {@const active = music.playing && music.level >= n}
                <button
                  type="button"
                  onclick={() => music.setLevel(n)}
                  disabled={!music.playing}
                  class="w-7 h-8 shrink-0 flex items-end justify-center cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                  aria-label={m.room_music_volume_level_aria({ level: n })}
                  aria-pressed={music.level === n}
                >
                  <span
                    class="block w-full rounded-sm transition-colors {active ? 'bg-brand-accent' : 'bg-brand-border'}"
                    style="height: {6 + i * 3}px;"
                  ></span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- CENTER: collapse/expand toggle -->
    <button
      type="button"
      onclick={() => (expanded = !expanded)}
      aria-expanded={expanded}
      aria-label={expanded ? m.room_header_collapse_aria() : m.room_header_expand_aria()}
      class="flex-1 inline-flex items-center justify-center gap-1.5 py-1 rounded-full text-brand-text-mid cursor-pointer"
    >
      <span class="text-[10px] font-bold uppercase tracking-[0.2em]">
        {expanded ? m.room_header_less() : m.room_header_more()}
      </span>
      <ChevronDown
        size={14}
        strokeWidth={2.5}
        class="transition-transform duration-200 {expanded ? 'rotate-180' : ''}"
      />
    </button>

    <!-- RIGHT: balance spacer matching the music slot width so More stays
         visually centered. -->
    <div class="w-16" aria-hidden="true"></div>
  </div>
</section>

<style>
  /* Scoped — the pill needs to read clearly against the gradient behind it. */
  .room-header { backdrop-filter: saturate(1.1); }
</style>
