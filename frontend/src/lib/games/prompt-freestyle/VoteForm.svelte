<script lang="ts">
  import { page } from '$app/stores';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Heart } from '$lib/icons';
  import type { Round, Submission } from '$lib/api/types';
  import SentenceWithBlank from '../_shared/SentenceWithBlank.svelte';
  import * as m from '$lib/paraglide/messages';

  let { submissions, round = null }: { submissions: Submission[]; round?: Round | null } = $props();

  let selectedId = $state<string | null>(null);
  let voted = $state(false);

  const allowSkipVote = $derived(
    ($page.data as any)?.room?.config?.allow_skip_vote ?? true
  );

  const hasVoteable = $derived(
    submissions.some((s) => s.id !== room.ownSubmissionId)
  );

  function abstain() {
    if (voted || room.ownSkippedVote) return;
    ws.send('skip_vote');
  }

  const prefix = $derived(
    round ? (round.item?.payload as { prefix?: string } | undefined)?.prefix ?? '' : ''
  );
  const suffix = $derived(
    round ? (round.item?.payload as { suffix?: string } | undefined)?.suffix ?? '' : ''
  );

  const letters = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'];

  function vote() {
    if (!selectedId || voted) return;
    ws.send('prompt-freestyle:vote', { submission_id: selectedId });
    voted = true;
  }
</script>

<style>
  .prompt-tilt-right { transform: rotate(0.8deg); }
  @media (max-width: 767.98px) { .prompt-tilt-right { transform: none; } }
</style>

<div class="flex flex-col gap-3 md:gap-6 pb-28 md:pb-0">
  {#if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      {m.game_paused_everyone_dropped()}
    </div>
  {/if}

  <!-- Voting hero card — sentence with blank emphasised so voters
       remember the prompt while comparing fillers. -->
  <div
    use:dealCard={{ delay: 160, rotate: 0.8 }}
    class="prompt-tilt-right relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white p-6 flex flex-col gap-3 overflow-hidden"
    style="box-shadow: 0 6px 0 rgba(0,0,0,0.25);"
  >
    <span
      class="absolute -top-2 -right-3 text-[90px] font-bold opacity-[0.08] pointer-events-none select-none leading-none"
      aria-hidden="true"
    >♥</span>
    <div class="relative flex items-center justify-between gap-2">
      <span class="text-[10px] font-bold uppercase tracking-[0.25em] opacity-70">
        {m.game_prompt_freestyle_vote_title()}
      </span>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
        style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
      >
        {m.game_prompt_freestyle_submissions_in({ count: submissions.length })}
      </span>
    </div>
    <p class="relative m-0 leading-tight tracking-tight">
      <SentenceWithBlank {prefix} {suffix} placeholder={m.game_prompt_freestyle_blank_placeholder()} size="lg" />
    </p>
    <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
      {m.game_prompt_freestyle_one_vote()}
    </span>
  </div>

  <!-- Submissions grid -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    {#each submissions as sub, i (sub.id)}
      {@const isOwn = sub.id === room.ownSubmissionId}
      {@const isSelected = selectedId === sub.id}
      {@const letter = letters[i] ?? ''}
      {@const fillerVal = (sub as Submission & { filler?: string }).filler ?? ''}
      <button
        use:dealCard={{ delay: 120 + i * 70, rotate: i % 2 === 0 ? -1.5 : 1.5, smooth: submissions.length > 3 }}
        use:physCard
        type="button"
        onclick={() => { if (!voted && !isOwn) selectedId = sub.id; }}
        disabled={voted || isOwn}
        aria-label={m.game_prompt_freestyle_filler_aria({ letter, filler: fillerVal })}
        aria-pressed={isSelected}
        class="relative rounded-[20px] p-3 flex flex-col gap-2.5 text-left transition-all duration-150"
        class:cursor-pointer={!isOwn && !voted}
        class:cursor-default={isOwn || voted}
        style="
          background: {isSelected ? 'var(--brand-white)' : 'var(--brand-surface)'};
          border: {isSelected ? '3.5px' : '2.5px'} solid {isSelected ? 'var(--brand-accent)' : 'var(--brand-border-heavy)'};
          box-shadow: {isSelected
            ? '0 7px 0 var(--brand-accent), 0 0 0 4px rgba(232,147,127,0.22)'
            : '0 5px 0 rgba(0,0,0,0.10)'};
          transform: {isSelected ? 'translateY(-2px)' : 'none'};
          opacity: {isOwn ? 0.7 : 1};
        "
      >
        <div class="flex items-center justify-between gap-2">
          <span
            class="font-mono text-[11px] font-bold tracking-[0.2em] transition-colors"
            style="color: {isSelected ? 'var(--brand-accent)' : 'var(--brand-text-muted)'};"
          >
            {letter} ·
          </span>
          {#if isSelected && !voted}
            <span
              class="chip-picked inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
            >
              {m.game_prompt_freestyle_picked()}
            </span>
          {/if}
        </div>

        {#if voted && isSelected}
          <span
            class="chip-voted absolute -top-2 right-2 z-20 inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-bold tracking-[0.18em] uppercase"
            style="transform: rotate(6deg);"
          >
            {m.game_prompt_freestyle_voted_chip()}
          </span>
        {/if}

        {#if isOwn}
          <span class="absolute top-2 right-2 z-10 text-[10px] font-bold uppercase tracking-[0.18em] px-2 py-0.5 rounded-full bg-brand-white border-[2px] border-brand-border-heavy text-brand-text-muted">
            {m.game_prompt_freestyle_yours()}
          </span>
        {/if}

        <div
          class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2.5 text-brand-text flex-1 leading-snug text-balance"
          style="box-shadow: 0 2px 0 rgba(0,0,0,0.04);"
        >
          <SentenceWithBlank {prefix} {suffix} filler={fillerVal} size="sm" />
        </div>
      </button>
    {/each}
  </div>

  {#if room.ownSkippedVote}
    <p class="text-center text-sm font-bold text-brand-text-mid m-0">
      {m.game_prompt_freestyle_skipped_waiting()}
    </p>
  {:else if !voted}
    <div class="hidden md:flex flex-row items-center justify-center gap-3">
      <button
        use:pressPhysics={'dark'}
        use:hoverEffect={'bounce'}
        type="button"
        onclick={vote}
        disabled={!selectedId}
        class="h-12 px-8 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <Heart size={18} strokeWidth={2.5} />
        {m.game_prompt_freestyle_lock_vote()}
      </button>
      {#if allowSkipVote && hasVoteable}
        <button
          use:pressPhysics={'ghost'}
          use:hoverEffect={'bounce'}
          type="button"
          onclick={abstain}
          class="h-12 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-surface text-brand-text-mid text-xs font-bold cursor-pointer inline-flex items-center justify-center"
        >
          {m.game_prompt_freestyle_skip()}
        </button>
      {/if}
    </div>

    <div
      class="md:hidden fixed inset-x-0 bottom-0 z-40 px-3 pt-0 pointer-events-none"
      style="padding-bottom: max(0.75rem, env(safe-area-inset-bottom));"
    >
      <div
        class="pointer-events-auto rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2 flex items-center gap-2"
        style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
      >
        <button
          use:pressPhysics={'dark'}
          type="button"
          onclick={vote}
          disabled={!selectedId}
          class="flex-1 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
        >
          <Heart size={16} strokeWidth={2.5} />
          {m.game_prompt_freestyle_lock_vote()}
        </button>
        {#if allowSkipVote && hasVoteable}
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={abstain}
            class="h-11 px-4 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-surface text-brand-text-mid text-xs font-bold cursor-pointer inline-flex items-center justify-center"
          >
            {m.game_prompt_freestyle_skip()}
          </button>
        {/if}
      </div>
    </div>
  {:else}
    <p class="text-center text-sm font-bold text-brand-text-mid m-0">
      {m.game_prompt_freestyle_voted_waiting()}
    </p>
  {/if}
</div>
