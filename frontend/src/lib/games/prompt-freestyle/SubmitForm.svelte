<script lang="ts">
  import { untrack } from 'svelte';
  import { page } from '$app/stores';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Send } from '$lib/icons';
  import type { Round } from '$lib/api/types';
  import SentenceWithBlank from '../_shared/SentenceWithBlank.svelte';
  import * as m from '$lib/paraglide/messages';
  import { localizeGameType } from '$lib/i18n/gameType';

  let { round }: { round: Round } = $props();

  let filler = $state('');
  let submitted = $state(false);

  let jokerArmed = $state(false);
  let jokerArmedTimer: ReturnType<typeof setTimeout> | null = null;

  const jokerBudget = $derived(($page.data as any)?.room?.config?.joker_count ?? 0);
  const jokerRemaining = $derived(room.jokersRemaining ?? jokerBudget);
  const jokerEnabled = $derived(jokerBudget > 0);

  const MAX_CHARS = 200;
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

  function submit() {
    if (submitted || isExpired || filler.trim().length === 0) return;
    const trimmed = filler.trim();
    ws.send('prompt-freestyle:submit', { filler: trimmed });
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
</script>

<style>
  .prompt-tilt-right { transform: rotate(0.8deg); }
  @media (max-width: 767.98px) {
    .prompt-tilt-right { transform: none; }
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
      {m.game_prompt_freestyle_submit_closed()}
    </div>
  {:else if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.game_paused_everyone_dropped()}
    </div>
  {/if}

  <!-- Sentence card — full-width dark hero showing the prompt with the live
       blank rendered inline. Replaces the meme image+prompt split since
       prompt-freestyle has no image. -->
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
        {m.game_prompt_freestyle_round_prompt({ number: round.round_number })}
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
        style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
      >
        <span class="h-1.5 w-1.5 rounded-full" style="background: var(--brand-accent); animation: pulse-dot 1.2s ease-in-out infinite;"></span>
        {m.game_prompt_freestyle_live()}
      </span>
    </div>
    <div class="relative m-0 leading-tight tracking-tight">
      <SentenceWithBlank
        {prefix}
        {suffix}
        filler={submitted ? filler.trim() : (filler.length > 0 ? filler : null)}
        placeholder={m.game_prompt_freestyle_blank_placeholder()}
        size="lg"
      />
    </div>
    <div class="relative flex items-center justify-between gap-2 mt-auto">
      <span class="text-[11px] font-bold uppercase tracking-[0.2em] opacity-70">
        {m.game_prompt_freestyle_fillers_anonymous()}
      </span>
      {#if room.gameType}
        <span class="hidden md:inline text-[10px] font-bold uppercase tracking-[0.2em] opacity-70 truncate max-w-[40%]">
          {localizeGameType(room.gameType).name}
        </span>
      {/if}
    </div>
  </div>

  <!-- Composer card -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <label for="prompt-freestyle-input" class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.game_prompt_freestyle_input_label()}
      </label>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span class="h-2 w-2 rounded-full" style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"></span>
        {submitted ? m.game_prompt_freestyle_submitted_chip() : m.game_prompt_freestyle_drafting_chip()}
      </span>
    </div>

    {#if room.ownSkippedSubmit}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {m.game_prompt_freestyle_joker_used()}
      </div>
    {:else if submitted}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {filler.trim()}
      </div>
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        {m.game_prompt_freestyle_submitted_waiting()}
      </p>
    {:else}
      <textarea
        id="prompt-freestyle-input"
        bind:value={filler}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={2}
        placeholder={m.game_prompt_freestyle_placeholder()}
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            submit();
          }
        }}
        onfocus={(e) => {
          const el = e.currentTarget;
          setTimeout(() => {
            el.scrollIntoView({ block: 'center', behavior: 'smooth' });
          }, 320);
        }}
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-lg font-bold text-brand-text resize-none focus:outline-none focus:border-brand-accent focus:bg-brand-white disabled:opacity-50 transition-colors"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04); line-height: 1.3; scroll-margin-top: 7rem; scroll-margin-bottom: 6rem;"
      ></textarea>

      <div class="hidden md:flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {m.game_prompt_freestyle_char_count({ count: filler.length, max: MAX_CHARS })}
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
                {m.game_prompt_freestyle_no_jokers_left()}
              {:else if jokerArmed}
                {m.game_prompt_freestyle_joker_armed()}
              {:else}
                {m.game_prompt_freestyle_joker_use({ n: jokerRemaining })}
              {/if}
            </button>
          {/if}
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'gradient'}
            type="button"
            onclick={submit}
            disabled={submitted || isExpired || filler.trim().length === 0}
            class="h-12 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.28);"
          >
            <Send size={16} strokeWidth={2.5} />
            {m.game_prompt_freestyle_submit_filler()}
          </button>
        </div>
      </div>

      <span class="md:hidden font-mono text-xs font-bold text-brand-text-muted tabular-nums self-end">
        {m.game_prompt_freestyle_char_count({ count: filler.length, max: MAX_CHARS })}
      </span>
    {/if}
  </div>

  {#if !submitted && !room.ownSkippedSubmit}
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
              ? m.game_prompt_freestyle_joker_armed()
              : m.game_prompt_freestyle_joker_use({ n: jokerRemaining })}
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
          disabled={submitted || isExpired || filler.trim().length === 0}
          class="flex-1 h-11 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2 whitespace-nowrap"
        >
          <Send size={16} strokeWidth={2.5} />
          {m.game_prompt_freestyle_submit_filler()}
        </button>
      </div>
    </div>
  {/if}
</div>
