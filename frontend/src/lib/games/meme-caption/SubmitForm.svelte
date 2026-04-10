<script lang="ts">
  import { untrack } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { Round } from '$lib/api/types';

  let { round }: { round: Round } = $props();

  let caption = $state('');
  let submitted = $state(false);

  const MAX_CHARS = 300;
  const deadline = $derived(Date.parse(round.ends_at));
  const totalMs = $derived(round.duration_seconds * 1000);
  const mountedExpired = $derived(deadline <= Date.now());
  // Seed the timer from the current deadline; subsequent updates are
  // driven by the requestAnimationFrame loop in the $effect below.
  let timerMs = $state(untrack(() => Math.max(0, deadline - Date.now())));

  $effect(() => {
    if (mountedExpired) return;
    const tick = () => {
      timerMs = Math.max(0, deadline - Date.now());
      if (timerMs > 0) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  const progressPct = $derived(totalMs > 0 ? (timerMs / totalMs) * 100 : 0);
  const secondsLeft = $derived(Math.ceil(timerMs / 1000));
  const isExpired = $derived(timerMs <= 0 || mountedExpired);

  function submit() {
    if (submitted || isExpired || caption.trim().length === 0) return;
    ws.send('meme_caption:submit', { caption: caption.trim() });
    submitted = true;
  }
</script>

<div class="flex flex-col gap-6">
  <!-- Timer -->
  {#if mountedExpired}
    <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
      Submission window has closed.
    </div>
  {:else}
    <div class="flex items-center gap-3">
      <div
        class="flex-1 h-2 rounded-full bg-muted overflow-hidden"
        role="progressbar"
        aria-valuenow={secondsLeft}
        aria-valuemin={0}
        aria-valuemax={round.duration_seconds}
        aria-label="Time remaining"
      >
        <div
          class="h-full bg-primary transition-none rounded-full"
          style="width: {progressPct}%"
        ></div>
      </div>
      <span class="text-sm tabular-nums font-medium w-10 text-right">{secondsLeft}s</span>
      <span class="text-sm text-muted-foreground">Round {round.round_number}</span>
    </div>
  {/if}

  <!-- Media prompt (if present) -->
  {#if round.item?.media_url ?? round.media_url}
    <img
      src={round.item?.media_url ?? round.media_url}
      alt="Round prompt"
      class="w-full aspect-video object-cover rounded-lg border border-border"
    />
  {/if}

  {#if round.text_prompt}
    <p class="text-center text-muted-foreground italic">"{round.text_prompt}"</p>
  {/if}

  <!-- Caption input -->
  {#if submitted}
    <div class="rounded-lg border border-border bg-muted p-4 text-center text-sm text-muted-foreground">
      Submitted ✓ — waiting for others…
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      <textarea
        bind:value={caption}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={3}
        placeholder="Write your caption…"
        class="w-full rounded-lg border border-input bg-background p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
      ></textarea>
      <div class="flex items-center justify-between">
        <span class="text-xs text-muted-foreground">{caption.length}/{MAX_CHARS}</span>
        <button
          type="button"
          onclick={submit}
          disabled={submitted || isExpired || caption.trim().length === 0}
          class="h-10 px-6 rounded-lg bg-primary text-primary-foreground text-sm font-semibold disabled:opacity-50 hover:bg-primary/90 transition-colors"
        >
          Submit
        </button>
      </div>
    </div>
  {/if}

  <!-- Player submission status -->
  <div class="flex flex-wrap gap-2">
    {#each room.players as player}
      {@const hasSub = room.submissions.some((s) => s.user_id === player.user_id)}
      <span class="flex items-center gap-1 text-xs px-2 py-1 rounded-full border {hasSub ? 'border-green-300 bg-green-50 text-green-800' : 'border-border text-muted-foreground'}">
        {hasSub ? '✓' : '⏳'} {player.username}
      </span>
    {/each}
  </div>
</div>
