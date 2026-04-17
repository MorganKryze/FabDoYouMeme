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

  const deadline = $derived(room.votingEndsAt ? Date.parse(room.votingEndsAt) : 0);
  let timerMs = $state(0);

  $effect(() => {
    if (!deadline) return;
    const tick = () => {
      if (room.roundPaused) return; // timer frozen while all players are reconnecting
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const secondsLeft = $derived(Math.ceil(timerMs / 1000));

  function vote() {
    if (!selectedId || voted) return;
    ws.send('meme-caption:vote', { submission_id: selectedId });
    voted = true;
  }
</script>

<div class="flex flex-col gap-6">
  {#if room.roundPaused}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2 text-xs font-bold text-brand-text-muted text-center"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      Everyone dropped — timer paused
    </div>
  {/if}

  <div class="flex items-center justify-between">
    <h3 class="stage-title">Pick the best one</h3>
    <div
      class="flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.1);"
    >
      <span
        class="h-2.5 w-2.5 rounded-full"
        style="background: var(--brand-accent); animation: {room.roundPaused ? 'none' : 'pulse-dot 1.5s ease-in-out infinite'};"
      ></span>
      <span class="text-sm font-bold tabular-nums">{secondsLeft}s</span>
    </div>
  </div>

  {#if (round?.item?.payload as { prompt?: string } | undefined)?.prompt}
    <p class="text-center text-brand-text-mid font-semibold italic text-sm">
      "{(round!.item!.payload as { prompt?: string }).prompt}"
    </p>
  {/if}

  <div class="w-full max-w-3xl mx-auto grid grid-cols-1 sm:grid-cols-2 gap-5">
    {#each submissions as sub, i}
      {@const isOwn = sub.id === room.ownSubmissionId}
      {@const isSelected = selectedId === sub.id}
      <button
        use:dealCard={{ delay: 80 + i * 90, rotate: i % 2 === 0 ? -2 : 2, smooth: submissions.length > 3 }}
        use:physCard
        type="button"
        onclick={() => { if (!voted && !isOwn) selectedId = sub.id; }}
        disabled={voted || isOwn}
        class="relative rounded-[22px] border-[2.5px] p-4 flex flex-col gap-3 text-left transition-colors
          {isSelected
            ? 'border-brand-text bg-brand-white'
            : 'border-brand-border-heavy bg-brand-surface'}
          {isOwn ? 'cursor-default opacity-70' : 'cursor-pointer'}"
        style="box-shadow: {isSelected ? '0 6px 0 var(--brand-text)' : '0 6px 0 rgba(0,0,0,0.12)'};"
      >
        {#if isOwn}
          <span class="absolute top-2 right-2 text-[0.65rem] font-bold uppercase tracking-[0.15em] px-2.5 py-1 rounded-full bg-brand-white border-[2px] border-brand-border-heavy text-brand-text-muted z-10">
            You
          </span>
        {/if}
        {#if isSelected}
          <span
            class="absolute top-2 left-2 h-7 w-7 rounded-full bg-brand-text text-brand-white text-sm font-bold inline-flex items-center justify-center border-[2px] border-brand-border-heavy z-10"
            aria-hidden="true"
          >
            {'\u2713'}
          </span>
        {/if}

        {#if round?.item?.media_url}
          <div
            class="w-full rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-white flex items-center justify-center"
            style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
          >
            <img
              src={mediaSrc(round.item.media_url, room.code)}
              alt="Round prompt"
              class="block w-full max-h-48 object-contain"
            />
          </div>
        {/if}

        <div
          class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2 text-sm font-bold text-brand-text text-center leading-snug"
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
      class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 cursor-pointer inline-flex items-center justify-center gap-2"
    >
      <Heart size={18} strokeWidth={2.5} />
      Vote
    </button>
  {:else}
    <p class="text-center text-sm font-bold text-brand-text-muted">Voted — waiting for results…</p>
  {/if}
</div>
