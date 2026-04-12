<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
  import { physCard } from '$lib/actions/physCard';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import type { Submission } from '$lib/api/types';

  let { submissions }: { submissions: Submission[] } = $props();

  let selectedId = $state<string | null>(null);
  let voted = $state(false);

  const deadline = $derived(room.currentRound ? Date.parse(room.currentRound.ends_at) : 0);
  let timerMs = $state(0);

  $effect(() => {
    if (!deadline) return;
    const tick = () => {
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const secondsLeft = $derived(Math.ceil(timerMs / 1000));

  function vote() {
    if (!selectedId || voted) return;
    ws.send('meme_caption:vote', { submission_id: selectedId });
    voted = true;
  }
</script>

<div class="flex flex-col gap-6">
  <div class="flex items-center justify-between">
    <h3 class="text-xl font-bold">Pick the best one</h3>
    <div
      class="flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-1.5"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.1);"
    >
      <span
        class="h-2.5 w-2.5 rounded-full"
        style="background: var(--brand-accent); animation: pulse-dot 1.5s ease-in-out infinite;"
      ></span>
      <span class="text-sm font-bold tabular-nums">{secondsLeft}s</span>
    </div>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
    {#each submissions as sub}
      {@const isOwn = sub.user_id === user.id}
      <button
        use:physCard
        type="button"
        onclick={() => { if (!voted) selectedId = sub.id; }}
        disabled={voted || isOwn}
        class="relative rounded-[22px] border-[2.5px] p-5 text-left transition-colors cursor-pointer
          {selectedId === sub.id
            ? 'border-brand-text bg-brand-white'
            : 'border-brand-border-heavy bg-brand-surface hover:bg-brand-white'}
          {isOwn ? 'cursor-default' : ''}
          disabled:opacity-70"
        style="box-shadow: {selectedId === sub.id ? '0 5px 0 var(--brand-text)' : '0 5px 0 rgba(0,0,0,0.08)'};"
      >
        {#if isOwn}
          <span class="absolute top-3 right-3 text-[0.7rem] font-bold px-3 py-1 rounded-full bg-brand-surface text-brand-text-muted">
            You
          </span>
        {/if}
        {#if selectedId === sub.id}
          <span class="absolute top-3 left-3 font-bold text-brand-text">{'\u2713'}</span>
        {/if}
        <p class="text-sm font-bold leading-relaxed pr-8">{sub.caption}</p>
      </button>
    {/each}
  </div>

  {#if !voted}
    <button
      use:pressPhysics={'dark'}
      type="button"
      onclick={vote}
      disabled={!selectedId}
      class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold disabled:opacity-50 transition-colors cursor-pointer"
    >
      Vote
    </button>
  {:else}
    <p class="text-center text-sm font-bold text-brand-text-muted">Voted — waiting for results…</p>
  {/if}
</div>
