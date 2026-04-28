<script lang="ts">
  import { untrack } from 'svelte';
  import { page } from '$app/stores';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Send, ChevronLeft, ChevronRight } from '$lib/icons';
  import { mediaSrc } from '$lib/api/media';
  import type { Round } from '$lib/api/types';
  import { handStore } from './handStore.svelte';
  import { fly } from 'svelte/transition';
  import { backOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';
  import { localizeGameType } from '$lib/i18n/gameType';

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

  // ── Mobile hand pager ─────────────────────────────────────────────
  // Phone shows ONE card at a time (no scroll carousel, no flex with
  // inflexible children — those propagate intrinsic widths up the tree
  // and made the page render at 2-3x the viewport width). Prev / next
  // buttons + dots drive the index. Desktop keeps the fan layout.
  let currentCard = $state(0);
  // Direction of the last move (+1 forward, -1 back, 0 first render) —
  // drives the slide-in transition so the new card flies in from the
  // matching side.
  let cardDirection = $state(0);

  function gotoCard(i: number) {
    if (i < 0 || i >= cards.length || i === currentCard) return;
    cardDirection = i > currentCard ? 1 : -1;
    currentCard = i;
  }
  // Auto-clamp when the hand shrinks (e.g. server re-deals or removes a card).
  $effect(() => {
    if (currentCard >= cards.length && cards.length > 0) currentCard = cards.length - 1;
  });
</script>

<div class="flex flex-col gap-3 md:gap-6 pb-28 md:pb-0 min-w-0">
  {#if mountedExpired}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-mid w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
    >
      {m.game_meme_showdown_submit_closed()}
    </div>
  {:else if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.game_paused_everyone_dropped()}
    </div>
  {/if}

  <!-- Prompt split: dark instructions card first (so the player reads the
       brief before scrolling past the meme), tilted image card second.
       `grid-cols-1` + `minmax(0, …fr)` on desktop are load-bearing — without
       them the implicit grid track sizes to the image's *natural* pixel
       width and inflates the whole page. -->
  <div class="grid grid-cols-1 gap-3 md:gap-4 md:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)] items-stretch">
    <div
      use:dealCard={{ delay: 80, rotate: 0.8 }}
      class="prompt-tilt-right relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-4 sm:p-6 flex flex-col gap-2.5 sm:gap-3 overflow-hidden"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.25);"
    >
      <span
        class="absolute -top-2 -right-3 text-[90px] font-bold opacity-[0.08] pointer-events-none select-none leading-none"
        aria-hidden="true"
      >♠</span>
      <div class="relative flex items-center justify-between gap-2">
        <span class="text-[10px] font-bold uppercase tracking-[0.25em] opacity-70">
          {m.game_meme_showdown_round_play({ number: round.round_number })}
        </span>
        <span
          class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
          style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
        >
          <span class="h-1.5 w-1.5 rounded-full" style="background: var(--brand-accent); animation: pulse-dot 1.2s ease-in-out infinite;"></span>
          {m.game_meme_showdown_live()}
        </span>
      </div>
      <p
        class="relative m-0 font-bold leading-tight tracking-tight"
        style="font-size: clamp(1.125rem, 2.4vw, 2rem);"
      >
        {promptText ?? m.game_meme_showdown_prompt_fallback()}
      </p>
      <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
        {m.game_meme_showdown_plays_anonymous()}
      </span>
    </div>

    <!-- Wrapped so the joker-drop overlay can position over the image
         without being subject to the dealCard rotation. min-w-0 on the
         grid item + the inner card stops the image from forcing the
         whole row wider than its grid track. -->
    <div class="relative min-w-0">
      <div
        use:dealCard={{ delay: 160, rotate: -1.2 }}
        class="prompt-tilt-left relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-3 flex flex-col gap-2 min-w-0"
        style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
      >
        {#if round.item?.media_url}
          <div
            class="relative w-full min-w-0 rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-surface"
            style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
          >
            <!-- min-w-0 on the img stops its intrinsic natural pixel
                 width from propagating up as the min-content of every
                 ancestor (the actual root cause of "page is 2× the
                 viewport" — w-full only caps display, not min-content). -->
            <img
              src={mediaSrc(round.item.media_url, room.code)}
              alt={m.game_round_meme_alt()}
              class="block w-full h-auto max-h-[26dvh] md:max-h-[60dvh] object-cover min-w-0"
            />
          </div>
        {/if}
        <!-- Round/pack labels: hidden on phone (round + game name already
             live in the sticky RoomHeader). -->
        <div class="hidden md:flex justify-between text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1">
          <span>{m.game_meme_showdown_round_number({ number: round.round_number })}</span>
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
  </div>

  <!-- Hand of text cards — same shell as meme-freestyle's composer card,
       just with a swipe pager replacing the textarea. min-w-0 stops the
       internal scroll content from inflating the card's allocated width. -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full min-w-0"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.game_meme_showdown_hand_label({ count: cards.length })}
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span
          class="h-2 w-2 rounded-full"
          style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"
        ></span>
        {submitted ? m.game_meme_showdown_submitted_chip() : m.game_meme_showdown_choose_one()}
      </span>
    </div>

    {#if room.ownSkippedSubmit}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {m.game_meme_showdown_joker_used()}
      </div>
    {:else if submitted}
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        {m.game_meme_showdown_submitted_waiting()}
      </p>
    {:else if cards.length === 0}
      <p class="text-xs font-bold text-brand-text-muted text-center m-0">
        {m.game_meme_showdown_dealing()}
      </p>
    {:else}
      <!-- ─── Phone hand: one card at a time, prev/next + dots ────────
           Critical: no flex container with inflexible 200-px children
           on mobile — that pattern propagates intrinsic min-content up
           the tree and forces the entire page wider than viewport, no
           matter how many `min-width: 0` you stack. Rendering exactly
           one card means there's nothing wider than its parent. -->
      {@const active = cards[currentCard] ?? cards[0]}
      <div
        class="md:hidden flex flex-col items-center gap-3"
        role="radiogroup"
        aria-label={m.game_meme_showdown_hand_aria()}
      >
        <!-- Wrapper has fixed dimensions matching the card so the in/out
             transitions don't make sibling elements jump around. -->
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
                <span class="hand-card-pip top" aria-hidden="true">♠</span>
                <span class="hand-card-text">{active.text}</span>
                <span class="hand-card-pip bottom" aria-hidden="true">♠</span>
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
              aria-label={m.game_meme_showdown_pager_prev_aria()}
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
                  aria-label={m.game_meme_showdown_pager_jump_aria({ index: i + 1, total: cards.length })}
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
              aria-label={m.game_meme_showdown_pager_next_aria()}
              class="h-12 w-12 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid disabled:opacity-30 disabled:cursor-not-allowed inline-flex items-center justify-center cursor-pointer"
              style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
            >
              <ChevronRight size={22} strokeWidth={2.5} />
            </button>
          </div>
        {/if}
      </div>

      <!-- ─── Desktop hand: original fan ─────────────────────────────── -->
      <div
        role="radiogroup"
        aria-label={m.game_meme_showdown_hand_aria()}
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
            <span class="hand-card-pip top" aria-hidden="true">♠</span>
            <span class="hand-card-text">{card.text}</span>
            <span class="hand-card-pip bottom" aria-hidden="true">♠</span>
          </button>
        {/each}
      </div>

      <!-- Desktop action row — joker + submit live inside the hand card.
           Mirrored on phone by a sticky bottom pill outside this card so
           the buttons never wrap into the hand's horizontal scroll area. -->
      <div class="hidden md:flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {selectedCardId ? m.game_meme_showdown_card_selected() : m.game_meme_showdown_pick_card()}
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
                {m.game_meme_showdown_no_jokers_left()}
              {:else if jokerArmed}
                {m.game_meme_showdown_joker_armed()}
              {:else}
                {m.game_meme_showdown_joker_use({ n: jokerRemaining })}
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
            {m.game_meme_showdown_play_caption()}
          </button>
        </div>
      </div>

      <!-- Mobile pick-state caption — kept inline above the (sticky) action
           pill so the player still sees "pick a card" feedback. -->
      <span class="md:hidden font-mono text-xs font-bold text-brand-text-muted tabular-nums self-end">
        {selectedCardId ? m.game_meme_showdown_card_selected() : m.game_meme_showdown_pick_card()}
      </span>
    {/if}
  </div>

  <!-- Mobile sticky action pill — joker + play pinned to the bottom edge,
       same pattern as meme-freestyle. Compact joker (icon + count) so the
       primary submit button keeps its full width on phones. -->
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
              ? m.game_meme_showdown_joker_armed()
              : m.game_meme_showdown_joker_use({ n: jokerRemaining })}
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
          {m.game_meme_showdown_play_caption()}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Decorative card rotations only kick in on tablet+ — on phones the
     ~1.2° tilt makes corners poke past the viewport edge and forces a
     horizontal scrollbar (visible as "image sets the page width"). */
  .prompt-tilt-left { transform: rotate(-1.2deg); }
  .prompt-tilt-right { transform: rotate(0.8deg); }
  @media (max-width: 767.98px) {
    .prompt-tilt-left,
    .prompt-tilt-right { transform: none; }
  }

  /* ------- Hand of cards ------- */
  /* Mobile renders one card at a time (see the `md:hidden` block in the
     markup) — no flex container with inflexible children that could
     propagate intrinsic widths up the tree. The `.hand-fan` rules below
     only apply on tablet+ (gated by `hidden md:flex` in the markup). */
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

  /* Mobile single-card sizing: the same TCG portrait visual but as a
     standalone block, not a flex item. Spring-style transition on
     transform/box-shadow gives the selection a small "yes!" hop
     instead of a snap, and the active state lifts the card slightly. */
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

  /* Pager dots — small chip-like indicators below the hand on phone. */
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

  /* Desktop fan layout — `.hand-fan` is gated `hidden md:flex` in the
     markup so these rules only apply when it actually renders. */
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
  }
</style>
