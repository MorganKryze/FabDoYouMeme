<script lang="ts">
  import { untrack } from 'svelte';
  import { page } from '$app/stores';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Send } from '$lib/icons';
  import { mediaSrc } from '$lib/api/media';
  import type { Round } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';
  import { localizeGameType } from '$lib/i18n/gameType';

  let { round }: { round: Round } = $props();

  let caption = $state('');
  let submitted = $state(false);

  // Two-step confirm for the joker: first click arms it (shows "Tap again to
  // confirm"), second click within 3s fires skip_submit. The arm state
  // self-resets if the user doesn't confirm, so a stray tap never costs a joker.
  let jokerArmed = $state(false);
  let jokerArmedTimer: ReturnType<typeof setTimeout> | null = null;

  const jokerBudget = $derived(($page.data as any)?.room?.config?.joker_count ?? 0);
  const jokerRemaining = $derived(room.jokersRemaining ?? jokerBudget);
  const jokerEnabled = $derived(jokerBudget > 0);

  const MAX_CHARS = 200;
  const deadline = $derived(Date.parse(round.ends_at));
  const mountedExpired = $derived(deadline <= Date.now());
  // Timer is primarily rendered by RoomHeader now; we still track locally
  // to gate the submit button without reading from the DOM.
  let timerMs = $state(untrack(() => Math.max(0, deadline - Date.now())));

  $effect(() => {
    if (mountedExpired) return;
    const tick = () => {
      if (room.roundPaused) return;
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const isExpired = $derived(timerMs <= 0 || mountedExpired);
  const promptText = $derived(
    (round.item?.payload as { prompt?: string } | undefined)?.prompt ?? null
  );

  function submit() {
    if (submitted || isExpired || caption.trim().length === 0) return;
    const trimmed = caption.trim();
    ws.send('meme-freestyle:submit', { caption: trimmed });
    submitted = true;
  }

  function armJoker() {
    jokerArmed = true;
    if (jokerArmedTimer) clearTimeout(jokerArmedTimer);
    jokerArmedTimer = setTimeout(() => {
      jokerArmed = false;
      jokerArmedTimer = null;
    }, 3000);
  }

  function fireJoker() {
    if (jokerArmedTimer) {
      clearTimeout(jokerArmedTimer);
      jokerArmedTimer = null;
    }
    jokerArmed = false;
    ws.send('skip_submit');
  }

  function onJokerClick() {
    if (submitted || isExpired || !jokerEnabled || jokerRemaining <= 0) return;
    if (!jokerArmed) {
      armJoker();
    } else {
      fireJoker();
    }
  }
</script>

<style>
  .joker-btn {
    height: 2.75rem;
    padding: 0 1rem;
    border-radius: 9999px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-surface);
    color: var(--brand-text);
    font-size: 0.75rem;
    font-weight: 700;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    cursor: pointer;
    transition:
      background 220ms ease,
      border-color 220ms ease,
      color 220ms ease;
  }
  .joker-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .joker-btn-pip {
    font-size: 0.9rem;
    line-height: 1;
    transform: translateY(-1px);
  }
  .joker-btn.is-armed {
    background: linear-gradient(135deg, var(--brand-grad-1), var(--brand-grad-2));
    border-color: var(--brand-accent);
    color: var(--brand-text);
    animation: jokerArm 900ms ease-in-out infinite;
  }
  .joker-btn.is-armed .joker-btn-pip {
    animation: jokerPipSpin 1.6s linear infinite;
  }
  @keyframes jokerArm {
    0%, 100% { box-shadow: 0 3px 0 rgba(0, 0, 0, 0.1), 0 0 0 0 rgba(232, 147, 127, 0.45); }
    50%      { box-shadow: 0 3px 0 rgba(0, 0, 0, 0.1), 0 0 0 8px rgba(232, 147, 127, 0); }
  }
  @keyframes jokerPipSpin {
    0%   { transform: translateY(-1px) rotate(0deg); }
    100% { transform: translateY(-1px) rotate(360deg); }
  }

  /* Joker-drop overlay — appears once skip_submit is confirmed. A playing
     card tumbles from above and lands tilted over the meme card. */
  .joker-drop {
    position: absolute;
    inset: 0;
    z-index: 20;
    pointer-events: none;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 8%;
  }
  .joker-card {
    position: relative;
    width: min(68%, 220px);
    aspect-ratio: 3 / 4.2;
    border-radius: 18px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    padding: 14px;
    box-shadow:
      0 14px 28px rgba(0, 0, 0, 0.28),
      0 6px 0 rgba(0, 0, 0, 0.2);
    transform-origin: 50% -40%;
    animation: jokerDrop 820ms cubic-bezier(0.22, 1.22, 0.36, 1) both;
  }
  .joker-card .corner {
    display: flex;
    flex-direction: column;
    align-items: center;
    line-height: 1;
    color: var(--brand-text);
    font-weight: 800;
  }
  .joker-card .corner .rank {
    font-size: 1.2rem;
    letter-spacing: -0.02em;
  }
  .joker-card .corner .pip {
    font-size: 1rem;
    margin-top: 2px;
  }
  .joker-card .corner.bottom {
    align-self: flex-end;
    transform: rotate(180deg);
  }
  .joker-card .center {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    pointer-events: none;
  }
  .joker-card .center .pip-big {
    font-size: clamp(3rem, 9vw, 4.5rem);
    line-height: 1;
    color: var(--brand-text);
    opacity: 0.92;
  }
  .joker-card .center .word {
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-weight: 800;
    letter-spacing: 0.22em;
    font-size: 0.85rem;
    color: var(--brand-text);
    padding: 4px 10px;
    border: 2px solid var(--brand-border-heavy);
    border-radius: 9999px;
    background: var(--brand-grad-1);
  }
  @keyframes jokerDrop {
    0% {
      transform: translate3d(-8%, -150%, 0) rotate(-32deg) scale(0.55);
      opacity: 0;
    }
    45% {
      transform: translate3d(2%, 6%, 0) rotate(12deg) scale(1.06);
      opacity: 1;
    }
    65% {
      transform: translate3d(-1%, -2%, 0) rotate(-6deg) scale(0.97);
    }
    85% {
      transform: translate3d(0, 1%, 0) rotate(-3.5deg) scale(1.01);
    }
    100% {
      transform: translate3d(0, 0, 0) rotate(-4deg) scale(1);
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .joker-card { animation: none; transform: rotate(-4deg); }
    .joker-btn.is-armed { animation: none; }
    .joker-btn.is-armed .joker-btn-pip { animation: none; }
  }
</style>

<div class="flex flex-col gap-6">
  {#if mountedExpired}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-mid w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
    >
      {m.game_meme_freestyle_submit_closed()}
    </div>
  {:else if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.game_paused_everyone_dropped()}
    </div>
  {/if}

  <!-- Prompt split: tilted image card + dark prompt card -->
  <div class="grid gap-4 md:grid-cols-[1fr_1.2fr] items-stretch">
    <!-- Left: image card (tilted) — wrapped so the joker-drop overlay can
         position over it without being subject to the dealCard rotation. -->
    <div class="relative">
      <div
        use:dealCard={{ delay: 80, rotate: -1.2 }}
        class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-3 flex flex-col gap-2"
        style="box-shadow: 0 6px 0 rgba(0,0,0,0.12); transform: rotate(-1.2deg);"
      >
        {#if round.item?.media_url}
          <div
            class="relative w-full rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-surface"
            style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
          >
            <img
              src={mediaSrc(round.item.media_url, room.code)}
              alt={m.game_meme_freestyle_prompt_alt()}
              class="block w-full h-auto max-h-[60vh] object-cover"
            />
          </div>
        {/if}
        <div class="flex justify-between text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1">
          <span>{m.game_meme_freestyle_round_number({ number: round.round_number })}</span>
          {#if room.gameType}
            <span class="truncate max-w-[60%]">{localizeGameType(room.gameType).name}</span>
          {/if}
        </div>
      </div>

      {#if room.ownSkippedSubmit}
        <div class="joker-drop" aria-hidden="true">
          <div class="joker-card">
            <div class="corner top">
              <span class="rank">J</span>
              <span class="pip">♠</span>
            </div>
            <div class="center">
              <span class="pip-big">♠</span>
              <span class="word">JOKER</span>
            </div>
            <div class="corner bottom">
              <span class="rank">J</span>
              <span class="pip">♠</span>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <!-- Right: dark prompt card (tilted other way) -->
    <div
      use:dealCard={{ delay: 160, rotate: 0.8 }}
      class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-6 flex flex-col gap-3 overflow-hidden"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.25); transform: rotate(0.8deg);"
    >
      <span
        class="absolute -top-2 -right-3 text-[90px] font-bold opacity-[0.08] pointer-events-none select-none leading-none"
        aria-hidden="true"
      >♠</span>
      <div class="relative flex items-center justify-between gap-2">
        <span class="text-[10px] font-bold uppercase tracking-[0.25em] opacity-70">
          {m.game_meme_freestyle_round_prompt({ number: round.round_number })}
        </span>
        <span
          class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
          style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
        >
          <span class="h-1.5 w-1.5 rounded-full" style="background: var(--brand-accent); animation: pulse-dot 1.2s ease-in-out infinite;"></span>
          {m.game_meme_freestyle_live()}
        </span>
      </div>
      <p
        class="relative m-0 font-bold leading-tight tracking-tight"
        style="font-size: clamp(1.5rem, 2.4vw, 2rem);"
      >
        {promptText ?? m.game_meme_freestyle_prompt_fallback()}
      </p>
      <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
        {m.game_meme_freestyle_captions_anonymous()}
      </span>
    </div>
  </div>

  <!-- Composer -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <label for="meme-freestyle-input" class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.game_meme_freestyle_input_label()}
      </label>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span
          class="h-2 w-2 rounded-full"
          style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"
        ></span>
        {submitted ? m.game_meme_freestyle_submitted_chip() : m.game_meme_freestyle_drafting_chip()}
      </span>
    </div>

    {#if room.ownSkippedSubmit}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {m.game_meme_freestyle_joker_used()}
      </div>
    {:else if submitted}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {caption.trim()}
      </div>
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        {m.game_meme_freestyle_submitted_waiting()}
      </p>
    {:else}
      <textarea
        id="meme-freestyle-input"
        bind:value={caption}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={3}
        placeholder={m.game_meme_freestyle_placeholder()}
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            submit();
          }
        }}
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-lg font-bold text-brand-text resize-none focus:outline-none focus:border-brand-accent focus:bg-brand-white disabled:opacity-50 transition-colors"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04); line-height: 1.3;"
      ></textarea>

      <div class="flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {m.game_meme_freestyle_char_count({ count: caption.length, max: MAX_CHARS })}
        </span>
        <div class="flex items-center gap-2">
          {#if jokerEnabled}
            <button
              use:pressPhysics={'ghost'}
              type="button"
              onclick={onJokerClick}
              disabled={submitted || isExpired || jokerRemaining <= 0}
              class="joker-btn"
              class:is-armed={jokerArmed}
              aria-pressed={jokerArmed}
            >
              <span class="joker-btn-pip" aria-hidden="true">♠</span>
              {#if jokerRemaining <= 0}
                {m.game_meme_freestyle_no_jokers_left()}
              {:else if jokerArmed}
                {m.game_meme_freestyle_joker_armed()}
              {:else}
                {m.game_meme_freestyle_joker_use({ n: jokerRemaining })}
              {/if}
            </button>
          {/if}
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            type="button"
            onclick={submit}
            disabled={submitted || isExpired || caption.trim().length === 0}
            class="h-12 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.28);"
          >
            <Send size={16} strokeWidth={2.5} />
            {m.game_meme_freestyle_submit_caption()}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
