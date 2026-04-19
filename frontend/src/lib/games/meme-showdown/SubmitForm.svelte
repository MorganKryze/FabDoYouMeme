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
  import { handStore } from './handStore.svelte';

  let { round }: { round: Round } = $props();

  let selectedCardId = $state<string | null>(null);
  let submitted = $state(false);

  // Joker plumbing mirrors meme-freestyle.SubmitForm. Two-step confirm: arm on
  // first tap, fire on second tap within 3s. Prevents a stray touch from
  // costing a joker silently.
  let jokerArmed = $state(false);
  let jokerArmedTimer: ReturnType<typeof setTimeout> | null = null;

  const jokerBudget = $derived(($page.data as any)?.room?.config?.joker_count ?? 0);
  const jokerRemaining = $derived(room.jokersRemaining ?? jokerBudget);
  const jokerEnabled = $derived(jokerBudget > 0);

  const deadline = $derived(Date.parse(round.ends_at));
  const mountedExpired = $derived(deadline <= Date.now());
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
    if (submitted || isExpired || !selectedCardId) return;
    const cardId = selectedCardId;
    ws.send('meme-showdown:submit', { card_id: cardId });
    handStore.onSubmit(cardId);
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
    if (!jokerArmed) armJoker();
    else fireJoker();
  }

  const cards = $derived(handStore.cards);
</script>

<div class="flex flex-col gap-6">
  {#if mountedExpired}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-mid w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
    >
      Submission window has closed.
    </div>
  {:else if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      Everyone dropped — timer paused
    </div>
  {/if}

  <!-- Image + prompt split (matches meme-freestyle visual grammar) -->
  <div class="grid gap-4 md:grid-cols-[1fr_1.2fr] items-stretch">
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
            alt="Round meme"
            class="block w-full h-auto max-h-[60vh] object-cover"
          />
        </div>
      {/if}
      <div class="flex justify-between text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1">
        <span>Round {round.round_number}</span>
        {#if room.gameType}
          <span class="truncate max-w-[60%]">{room.gameType.name}</span>
        {/if}
      </div>
    </div>

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
          Round {round.round_number} · play a card
        </span>
        <span
          class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
          style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
        >
          <span class="h-1.5 w-1.5 rounded-full" style="background: var(--brand-accent); animation: pulse-dot 1.2s ease-in-out infinite;"></span>
          Live
        </span>
      </div>
      <p
        class="relative m-0 font-bold leading-tight tracking-tight"
        style="font-size: clamp(1.5rem, 2.4vw, 2rem);"
      >
        {promptText ?? 'Pick the funniest caption for this meme.'}
      </p>
      <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
        Plays are anonymous · voting comes next
      </span>
    </div>
  </div>

  <!-- Hand of text cards -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        Your hand ({cards.length})
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span
          class="h-2 w-2 rounded-full"
          style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"
        ></span>
        {submitted ? 'Submitted' : 'Choose one'}
      </span>
    </div>

    {#if room.ownSkippedSubmit}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        Joker used — waiting for the others…
      </div>
    {:else if submitted}
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        Waiting for the others…
      </p>
    {:else if cards.length === 0}
      <p class="text-xs font-bold text-brand-text-muted text-center m-0">
        Dealing hand…
      </p>
    {:else}
      <div
        role="radiogroup"
        aria-label="Your caption hand"
        class="hand-fan"
        style="--card-count: {cards.length};"
      >
        {#each cards as card, i (card.card_id)}
          {@const isSelected = selectedCardId === card.card_id}
          {@const offset = cards.length <= 1 ? 0 : i - (cards.length - 1) / 2}
          {@const rot = offset * 5}
          {@const lift = Math.abs(offset) * Math.abs(offset) * 2.5}
          <button
            use:dealCard={{ delay: 100 + i * 70, rotate: i % 2 === 0 ? -2 : 2, smooth: cards.length > 3 }}
            type="button"
            role="radio"
            aria-checked={isSelected}
            onclick={() => { if (!isExpired) selectedCardId = card.card_id; }}
            disabled={isExpired}
            class="hand-card"
            class:is-selected={isSelected}
            class:is-disabled={isExpired}
            style="--rot: {rot}deg; --lift: {lift}px;"
          >
            <span class="hand-card-pip top" aria-hidden="true">♠</span>
            <span class="hand-card-text">{card.text}</span>
            <span class="hand-card-pip bottom" aria-hidden="true">♠</span>
          </button>
        {/each}
      </div>

      <div class="flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {selectedCardId ? 'Card selected' : 'Pick a card'}
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
                No jokers left
              {:else if jokerArmed}
                Tap again · cancels in 3s
              {:else}
                Use joker · {jokerRemaining} left
              {/if}
            </button>
          {/if}
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            type="button"
            onclick={submit}
            disabled={submitted || isExpired || !selectedCardId}
            class="h-12 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.28);"
          >
            <Send size={16} strokeWidth={2.5} />
            Play caption
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  /* ------- Hand of cards ------- */
  .hand-fan {
    position: relative;
    display: flex;
    flex-wrap: nowrap;
    align-items: flex-end;
    justify-content: flex-start;
    gap: 0.6rem;
    width: 100%;
    padding: 0.75rem 0.25rem 1.5rem;
    overflow-x: auto;
    overflow-y: visible;
    scroll-snap-type: x mandatory;
    scrollbar-width: none;
  }
  .hand-fan::-webkit-scrollbar { display: none; }

  .hand-card {
    position: relative;
    flex: 0 0 158px;
    height: 228px;
    scroll-snap-align: center;
    border-radius: 16px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.1);
    padding: 0.85rem;
    display: flex;
    flex-direction: column;
    text-align: left;
    cursor: pointer;
    transition:
      transform 280ms cubic-bezier(0.2, 0.8, 0.25, 1.05),
      box-shadow 220ms ease,
      border-color 220ms ease,
      background 220ms ease;
  }
  .hand-card::before {
    content: "";
    position: absolute;
    inset: 6px;
    border-radius: 11px;
    background:
      radial-gradient(circle at top left, rgba(0, 0, 0, 0.03), transparent 55%),
      radial-gradient(circle at bottom right, rgba(0, 0, 0, 0.03), transparent 55%);
    pointer-events: none;
  }
  .hand-card-pip {
    position: absolute;
    font-size: 1rem;
    font-weight: 700;
    color: var(--brand-text-muted);
    line-height: 1;
    opacity: 0.75;
    pointer-events: none;
    transition: color 220ms ease, opacity 220ms ease;
  }
  .hand-card-pip.top { top: 0.55rem; left: 0.65rem; }
  .hand-card-pip.bottom { bottom: 0.55rem; right: 0.65rem; transform: rotate(180deg); }

  .hand-card-text {
    margin: auto 0;
    display: block;
    font-size: 0.95rem;
    font-weight: 700;
    line-height: 1.3;
    color: var(--brand-text);
    text-wrap: balance;
    text-align: center;
    padding: 0 0.2rem;
  }

  .hand-card.is-selected {
    border-color: var(--brand-accent);
    box-shadow: 0 8px 0 var(--brand-accent), 0 0 0 4px rgba(232, 147, 127, 0.22);
  }
  .hand-card.is-selected .hand-card-pip {
    color: var(--brand-accent);
    opacity: 1;
  }
  .hand-card.is-disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  @media (min-width: 720px) {
    .hand-fan {
      overflow: visible;
      justify-content: center;
      gap: 0;
      padding: 1.25rem 1rem 2.5rem;
      min-height: 340px;
      perspective: 1200px;
    }
    .hand-card {
      flex: 0 0 200px;
      height: 286px;
      margin-left: -92px;
      transform-origin: 50% calc(100% + 120px);
      transform: rotate(var(--rot, 0deg)) translateY(var(--lift, 0px));
    }
    .hand-card:first-child { margin-left: 0; }
    .hand-card.is-selected {
      z-index: 20;
      transform: rotate(0deg) translateY(-38px) scale(1.05);
    }
    .hand-card:hover:not(.is-disabled),
    .hand-card:focus-visible {
      z-index: 30;
      transform: rotate(0deg) translateY(-24px) scale(1.04);
      box-shadow: 0 10px 0 rgba(0, 0, 0, 0.14);
      outline: none;
    }
    .hand-card.is-selected:hover,
    .hand-card.is-selected:focus-visible {
      transform: rotate(0deg) translateY(-46px) scale(1.06);
    }
    .hand-card-text { font-size: 1rem; line-height: 1.32; }
    .hand-card-pip { font-size: 1.05rem; }
  }

  @media (prefers-reduced-motion: reduce) {
    .hand-card { transition: box-shadow 200ms ease, border-color 200ms ease; }
  }

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
    transition: background 220ms ease, border-color 220ms ease, color 220ms ease;
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
  @media (prefers-reduced-motion: reduce) {
    .joker-btn.is-armed { animation: none; }
    .joker-btn.is-armed .joker-btn-pip { animation: none; }
  }
</style>
