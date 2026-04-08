<script lang="ts">
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import { room } from '$lib/state/room.svelte';
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
    <h3 class="font-semibold">Vote for the best caption</h3>
    <span class="text-sm tabular-nums text-muted-foreground">{secondsLeft}s</span>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
    {#each submissions as sub}
      {@const isOwn = sub.user_id === user.id}
      <button
        type="button"
        onclick={() => { if (!voted) selectedId = sub.id; }}
        disabled={voted || isOwn}
        class="relative rounded-xl border-2 p-4 text-left transition-colors
          {selectedId === sub.id ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/50'}
          {isOwn ? 'cursor-default' : 'cursor-pointer'}
          disabled:opacity-70"
      >
        {#if isOwn}
          <span class="absolute top-2 right-2 text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
            You
          </span>
        {/if}
        {#if selectedId === sub.id}
          <span class="absolute top-2 left-2 text-primary">✓</span>
        {/if}
        <p class="text-sm leading-relaxed pr-8">{sub.caption}</p>
      </button>
    {/each}
  </div>

  {#if !voted}
    <button
      type="button"
      onclick={vote}
      disabled={!selectedId}
      class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
    >
      Vote
    </button>
  {:else}
    <p class="text-center text-sm text-muted-foreground">Voted ✓ — waiting for results…</p>
  {/if}
</div>
