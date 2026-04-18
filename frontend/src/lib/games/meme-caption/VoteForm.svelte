<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Heart } from '$lib/icons';
  import { mediaSrc } from '$lib/api/media';
  import type { Round, Submission } from '$lib/api/types';

  let { submissions, round = null }: { submissions: Submission[]; round?: Round | null } = $props();

  let selectedId = $state<string | null>(null);
  let voted = $state(false);

  const promptText = $derived(
    round ? (round.item?.payload as { prompt?: string } | undefined)?.prompt ?? null : null
  );

  // A/B/C/D letter labels — useful as a compact identifier when narrating
  // votes in chat or screenshots.
  const letters = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'];

  function vote() {
    if (!selectedId || voted) return;
    ws.send('meme-caption:vote', { submission_id: selectedId });
    voted = true;
  }
</script>

<div class="flex flex-col gap-6">
  {#if room.roundPaused}
    <div
      class="inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5 text-xs font-bold text-brand-text-muted w-fit mx-auto"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      Everyone dropped — timer paused
    </div>
  {/if}

  <!-- Prompt split -->
  <div class="grid gap-4 md:grid-cols-[1fr_1.2fr] items-stretch">
    <div
      use:dealCard={{ delay: 80, rotate: -1.2 }}
      class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-3 flex flex-col gap-2"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12); transform: rotate(-1.2deg);"
    >
      {#if round?.item?.media_url}
        <div
          class="relative w-full rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-surface flex items-center justify-center"
          style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
        >
          <span
            class="absolute top-[-6px] left-1/2 w-[90px] h-[18px] rounded-[4px] border-[2px] z-10"
            style="transform: translateX(-50%) rotate(-2deg); background: rgba(255,255,255,0.6); border-color: var(--brand-border);"
            aria-hidden="true"
          ></span>
          <img
            src={mediaSrc(round.item.media_url, room.code)}
            alt="Round prompt"
            class="block w-full max-h-52 object-contain"
          />
        </div>
      {/if}
      <div class="flex justify-between text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-muted px-1">
        {#if round}
          <span>Round {round.round_number}</span>
        {/if}
        <span>{submissions.length} captions</span>
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
      >♥</span>
      <div class="relative flex items-center justify-between gap-2">
        <span class="text-[10px] font-bold uppercase tracking-[0.25em] opacity-70">
          Pick the funniest
        </span>
        <span
          class="inline-flex items-center gap-1.5 rounded-full border-[2px] px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.18em]"
          style="border-color: rgba(255,255,255,0.25); background: rgba(255,255,255,0.08);"
        >
          {submissions.length} in
        </span>
      </div>
      <p
        class="relative m-0 font-bold leading-tight tracking-tight"
        style="font-size: clamp(1.5rem, 2.4vw, 2rem);"
      >
        {promptText ? `"${promptText}"` : 'Captions are in. Time to vote.'}
      </p>
      <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
        One vote each · your own caption is locked
      </span>
    </div>
  </div>

  <!-- Submissions grid -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    {#each submissions as sub, i}
      {@const isOwn = sub.id === room.ownSubmissionId}
      {@const isSelected = selectedId === sub.id}
      {@const letter = letters[i] ?? ''}
      <button
        use:dealCard={{ delay: 120 + i * 70, rotate: i % 2 === 0 ? -1.5 : 1.5, smooth: submissions.length > 3 }}
        use:physCard
        type="button"
        onclick={() => { if (!voted && !isOwn) selectedId = sub.id; }}
        disabled={voted || isOwn}
        aria-label="Caption {letter}: {sub.caption}"
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
        <!-- Letter label + selection chip -->
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
              ♥ Picked
            </span>
          {/if}
        </div>

        <!-- VOTED sticker (only after final vote, not on pre-select) -->
        {#if voted && isSelected}
          <span
            class="chip-voted absolute -top-2 right-2 z-20 inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-bold tracking-[0.18em] uppercase"
            style="transform: rotate(6deg);"
          >
            ♥ Voted
          </span>
        {/if}

        <!-- Own-caption marker -->
        {#if isOwn}
          <span class="absolute top-2 right-2 z-10 text-[10px] font-bold uppercase tracking-[0.18em] px-2 py-0.5 rounded-full bg-brand-white border-[2px] border-brand-border-heavy text-brand-text-muted">
            Yours
          </span>
        {/if}

        {#if round?.item?.media_url}
          <div
            class="w-full rounded-[10px] overflow-hidden border-[2px] border-brand-border bg-brand-white flex items-center justify-center"
          >
            <img
              src={mediaSrc(round.item.media_url, room.code)}
              alt=""
              class="block w-full max-h-28 object-cover opacity-90"
            />
          </div>
        {/if}

        <div
          class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2.5 text-sm font-bold text-brand-text flex-1 flex items-center leading-snug text-balance"
          style="box-shadow: 0 2px 0 rgba(0,0,0,0.04);"
        >
          {sub.caption}
        </div>
      </button>
    {/each}
  </div>

  {#if !voted}
    <button
      use:pressPhysics={'dark'}
      use:hoverEffect={'bounce'}
      type="button"
      onclick={vote}
      disabled={!selectedId}
      class="h-12 mx-auto px-8 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
    >
      <Heart size={18} strokeWidth={2.5} />
      Lock in my vote
    </button>
  {:else}
    <p class="text-center text-sm font-bold text-brand-text-mid m-0">
      Voted — waiting for the count…
    </p>
  {/if}
</div>
