<script lang="ts">
  import { untrack } from 'svelte';
  import { page } from '$app/stores';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Send, ChevronLeft, ChevronRight } from '$lib/icons';
  import type { Round } from '$lib/api/types';
  // Reuse the meme-showdown handStore — both showdown game types share the
  // same hand-of-cards mechanic and the room state already routes the WS
  // hand payload through this singleton for any game type.
  import { handStore } from '$lib/games/meme-showdown/handStore.svelte';
  import SentenceWithBlank from '../_shared/SentenceWithBlank.svelte';
  import { fly } from 'svelte/transition';
  import { backOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';
  import { localizeGameType } from '$lib/i18n/gameType';

  let { round }: { round: Round } = $props();

  let selectedCardId = $state<string | null>(null);
  let submitted = $state(false);
  // Snapshot of the played card's text taken at submit time. We need it
  // separate from `selectedCard` because handStore.onSubmit() removes the
  // played card from the hand — without the snapshot the dark cover would
  // lose the filler the moment the player submits.
  let playedCardText = $state<string | null>(null);

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
  const prefix = $derived(
    (round.item?.payload as { prefix?: string } | undefined)?.prefix ?? ''
  );
  const suffix = $derived(
    (round.item?.payload as { suffix?: string } | undefined)?.suffix ?? ''
  );

  const cards = $derived(handStore.cards);
  const selectedCard = $derived(cards.find((c) => c.card_id === selectedCardId) ?? null);

  function submit() {
    if (submitted || isExpired || !selectedCardId) return;
    const cardId = selectedCardId;
    const card = cards.find((c) => c.card_id === cardId);
    playedCardText = card?.text ?? null;
    ws.send('prompt-showdown:submit', { card_id: cardId });
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

  // Mobile pager — same behaviour as meme-showdown.
  let currentCard = $state(0);
  let cardDirection = $state(0);
  function gotoCard(i: number) {
    if (i < 0 || i >= cards.length || i === currentCard) return;
    cardDirection = i > currentCard ? 1 : -1;
    currentCard = i;
  }
  $effect(() => {
    if (currentCard >= cards.length && cards.length > 0) currentCard = cards.length - 1;
  });
</script>

<style>
  .prompt-tilt-right { transform: rotate(0.8deg); }
  @media (max-width: 767.98px) { .prompt-tilt-right { transform: none; } }

  .hand-fan {
    position: relative;
    flex-wrap: nowrap;
    align-items: stretch;
    justify-content: center;
    gap: 0;
    padding: 1.25rem 1rem 2.5rem;
    min-height: 340px;
    perspective: 1200px;
  }
  .hand-card-mobile {
    width: 200px;
    max-width: 100%;
    height: 280px;
    transition:
      transform 280ms cubic-bezier(0.34, 1.56, 0.64, 1),
      box-shadow 220ms ease,
      border-color 220ms ease,
      background 220ms ease;
    will-change: transform;
  }
  .hand-card-mobile.is-selected {
    transform: translateY(-6px) scale(1.02);
    animation: hand-card-pick 320ms cubic-bezier(0.34, 1.56, 0.64, 1);
  }
  @keyframes hand-card-pick {
    0%   { transform: translateY(0) scale(1); }
    45%  { transform: translateY(-10px) scale(1.06); }
    100% { transform: translateY(-6px) scale(1.02); }
  }
  @media (prefers-reduced-motion: reduce) {
    .hand-card-mobile { transition: box-shadow 200ms ease, border-color 200ms ease; }
    .hand-card-mobile.is-selected { animation: none; transform: none; }
  }

  .hand-card {
    position: relative;
    height: 280px;
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

  .hand-pager-dot {
    height: 0.5rem;
    width: 0.5rem;
    border-radius: 9999px;
    background: var(--brand-border);
    border: none;
    padding: 0;
    cursor: pointer;
    transition: background-color 200ms ease, transform 200ms ease;
  }
  .hand-pager-dot.is-active {
    background: var(--brand-accent);
    transform: scale(1.35);
  }

  @media (min-width: 720px) {
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
    white-space: nowrap;
    cursor: pointer;
    transition: background 220ms ease, border-color 220ms ease, color 220ms ease;
  }
  .joker-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .joker-btn-pip { font-size: 0.9rem; line-height: 1; transform: translateY(-1px); }
  .joker-btn.is-armed {
    background: linear-gradient(135deg, var(--brand-grad-1), var(--brand-grad-2));
    border-color: var(--brand-accent);
    color: var(--brand-text);
    animation: jokerArm 900ms ease-in-out infinite;
  }
  .joker-btn.is-armed .joker-btn-pip { animation: jokerPipSpin 1.6s linear infinite; }
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

<div class="flex flex-col gap-3 md:gap-6 pb-28 md:pb-0 min-w-0">
  {#if mountedExpired}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-mid w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
    >
      {m.game_prompt_showdown_submit_closed()}
    </div>
  {:else if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.game_paused_everyone_dropped()}
    </div>
  {/if}

  <!-- Sentence card with live preview of the selected filler -->
  <div
    use:dealCard={{ delay: 80, rotate: 0.8 }}
    class="prompt-tilt-right relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-4 sm:p-6 flex flex-col gap-2.5 sm:gap-3 overflow-hidden"
    style="box-shadow: 0 6px 0 rgba(0,0,0,0.25);"
  >
    <span
      class="absolute -top-2 -right-3 text-[90px] font-bold opacity-[0.08] pointer-events-none select-none leading-none"
      aria-hidden="true"
    >♣</span>
    <div class="relative flex items-center justify-between gap-2">
      <span class="text-[10px] font-bold uppercase tracking-[0.25em] opacity-70">
        {m.game_prompt_showdown_round_play({ number: round.round_number })}
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
        style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
      >
        <span class="h-1.5 w-1.5 rounded-full" style="background: var(--brand-accent); animation: pulse-dot 1.2s ease-in-out infinite;"></span>
        {m.game_prompt_showdown_live()}
      </span>
    </div>
    <p class="relative m-0 leading-tight tracking-tight">
      <SentenceWithBlank
        {prefix}
        {suffix}
        filler={playedCardText ?? selectedCard?.text ?? null}
        placeholder={m.game_prompt_showdown_blank_placeholder()}
        size="lg"
      />
    </p>
    <div class="relative flex items-center justify-between gap-2 mt-auto">
      <span class="text-[11px] font-bold uppercase tracking-[0.2em] opacity-70">
        {m.game_prompt_showdown_plays_anonymous()}
      </span>
      {#if room.gameType}
        <span class="hidden md:inline text-[10px] font-bold uppercase tracking-[0.2em] opacity-70 truncate max-w-[40%]">
          {localizeGameType(room.gameType).name}
        </span>
      {/if}
    </div>
  </div>

  <!-- Hand of filler cards — same shell + pager as meme-showdown -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full min-w-0"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.game_prompt_showdown_hand_label({ count: cards.length })}
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span
          class="h-2 w-2 rounded-full"
          style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"
        ></span>
        {submitted ? m.game_prompt_showdown_submitted_chip() : m.game_prompt_showdown_choose_one()}
      </span>
    </div>

    {#if room.ownSkippedSubmit}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {m.game_prompt_showdown_joker_used()}
      </div>
    {:else if submitted}
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        {m.game_prompt_showdown_submitted_waiting()}
      </p>
    {:else if cards.length === 0}
      <p class="text-xs font-bold text-brand-text-muted text-center m-0">
        {m.game_prompt_showdown_dealing()}
      </p>
    {:else}
      {@const active = cards[currentCard] ?? cards[0]}
      <div
        class="md:hidden flex flex-col items-center gap-3"
        role="radiogroup"
        aria-label={m.game_prompt_showdown_hand_aria()}
      >
        <div class="relative w-[200px] max-w-full h-[280px]">
          {#if active}
            {@const isSelected = selectedCardId === active.card_id}
            {#key active.card_id}
              <button
                in:fly={{ x: cardDirection * 60, duration: 240, easing: backOut, opacity: 0 }}
                type="button"
                role="radio"
                aria-checked={isSelected}
                onclick={() => { if (!isExpired) selectedCardId = active.card_id; }}
                disabled={isExpired}
                class="hand-card hand-card-mobile absolute inset-0"
                class:is-selected={isSelected}
                class:is-disabled={isExpired}
              >
                <span class="hand-card-pip top" aria-hidden="true">♣</span>
                <span class="hand-card-text">{active.text}</span>
                <span class="hand-card-pip bottom" aria-hidden="true">♣</span>
              </button>
            {/key}
          {/if}
        </div>

        {#if cards.length > 1}
          <div class="flex items-center justify-center gap-4">
            <button
              use:pressPhysics={'ghost'}
              type="button"
              onclick={() => gotoCard(currentCard - 1)}
              disabled={currentCard === 0}
              aria-label={m.game_prompt_showdown_pager_prev_aria()}
              class="h-12 w-12 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid disabled:opacity-30 disabled:cursor-not-allowed inline-flex items-center justify-center cursor-pointer"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
            >
              <ChevronLeft size={22} strokeWidth={2.5} />
            </button>
            <div class="flex items-center gap-2">
              {#each cards as _, i (i)}
                <button
                  type="button"
                  onclick={() => gotoCard(i)}
                  aria-label={m.game_prompt_showdown_pager_jump_aria({ index: i + 1, total: cards.length })}
                  aria-current={currentCard === i ? 'true' : 'false'}
                  class="hand-pager-dot"
                  class:is-active={currentCard === i}
                ></button>
              {/each}
            </div>
            <button
              use:pressPhysics={'ghost'}
              type="button"
              onclick={() => gotoCard(currentCard + 1)}
              disabled={currentCard === cards.length - 1}
              aria-label={m.game_prompt_showdown_pager_next_aria()}
              class="h-12 w-12 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid disabled:opacity-30 disabled:cursor-not-allowed inline-flex items-center justify-center cursor-pointer"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
            >
              <ChevronRight size={22} strokeWidth={2.5} />
            </button>
          </div>
        {/if}
      </div>

      <div
        role="radiogroup"
        aria-label={m.game_prompt_showdown_hand_aria()}
        class="hand-fan hidden md:flex"
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
            <span class="hand-card-pip top" aria-hidden="true">♣</span>
            <span class="hand-card-text">{card.text}</span>
            <span class="hand-card-pip bottom" aria-hidden="true">♣</span>
          </button>
        {/each}
      </div>

      <div class="hidden md:flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {selectedCardId ? m.game_prompt_showdown_card_selected() : m.game_prompt_showdown_pick_card()}
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
                {m.game_prompt_showdown_no_jokers_left()}
              {:else if jokerArmed}
                {m.game_prompt_showdown_joker_armed()}
              {:else}
                {m.game_prompt_showdown_joker_use({ n: jokerRemaining })}
              {/if}
            </button>
          {/if}
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            type="button"
            onclick={submit}
            disabled={submitted || isExpired || !selectedCardId}
            class="h-12 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2 whitespace-nowrap focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.28);"
          >
            <Send size={16} strokeWidth={2.5} />
            {m.game_prompt_showdown_play_filler()}
          </button>
        </div>
      </div>

      <span class="md:hidden font-mono text-xs font-bold text-brand-text-muted tabular-nums self-end">
        {selectedCardId ? m.game_prompt_showdown_card_selected() : m.game_prompt_showdown_pick_card()}
      </span>
    {/if}
  </div>

  {#if !submitted && !room.ownSkippedSubmit && cards.length > 0}
    <div
      class="md:hidden fixed inset-x-0 bottom-0 z-40 px-3 pt-0 pointer-events-none"
      style="padding-bottom: max(0.75rem, env(safe-area-inset-bottom));"
    >
      <div
        class="pointer-events-auto rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2 py-2 flex items-center gap-2"
        style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
      >
        {#if jokerEnabled && jokerRemaining > 0}
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={onJokerClick}
            disabled={submitted || isExpired}
            class="joker-btn shrink-0"
            class:is-armed={jokerArmed}
            aria-pressed={jokerArmed}
            aria-label={jokerArmed
              ? m.game_prompt_showdown_joker_armed()
              : m.game_prompt_showdown_joker_use({ n: jokerRemaining })}
          >
            <span class="joker-btn-pip" aria-hidden="true">♠</span>
            <span class="font-mono tabular-nums text-xs">
              {jokerArmed ? '?' : jokerRemaining}
            </span>
          </button>
        {/if}
        <button
          use:pressPhysics={'dark'}
          type="button"
          onclick={submit}
          disabled={submitted || isExpired || !selectedCardId}
          class="flex-1 h-11 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2 whitespace-nowrap"
        >
          <Send size={16} strokeWidth={2.5} />
          {m.game_prompt_showdown_play_filler()}
        </button>
      </div>
    </div>
  {/if}
</div>
