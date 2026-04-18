<script lang="ts">
  import { untrack } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { dealCard } from '$lib/actions/dealCard';
  import { Send } from '$lib/icons';
  import { mediaSrc } from '$lib/api/media';
  import type { Round } from '$lib/api/types';

  let { round }: { round: Round } = $props();

  let caption = $state('');
  let submitted = $state(false);

  const MAX_CHARS = 200;
  const deadline = $derived(Date.parse(round.ends_at));
  const mountedExpired = $derived(deadline <= Date.now());
  // Timer is primarily rendered by RoomHeader now; we still track locally
  // to gate the submit button without reading from the DOM.
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
    if (submitted || isExpired || caption.trim().length === 0) return;
    const trimmed = caption.trim();
    ws.send('meme-caption:submit', { caption: trimmed });
    submitted = true;
  }
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

  <!-- Prompt split: tilted image card + dark prompt card -->
  <div class="grid gap-4 md:grid-cols-[1fr_1.2fr] items-stretch">
    <!-- Left: image card (tilted) -->
    <div
      use:dealCard={{ delay: 80, rotate: -1.2 }}
      class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-3 flex flex-col gap-2"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12); transform: rotate(-1.2deg);"
    >
      {#if round.item?.media_url}
        <div
          class="relative w-full rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-surface flex items-center justify-center"
          style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
        >
          <!-- Tape accent — decorative, pinned top-center -->
          <span
            class="absolute top-[-6px] left-1/2 w-[90px] h-[18px] rounded-[4px] border-[2px] z-10"
            style="transform: translateX(-50%) rotate(-2deg); background: rgba(255,255,255,0.6); border-color: var(--brand-border);"
            aria-hidden="true"
          ></span>
          <img
            src={mediaSrc(round.item.media_url, room.code)}
            alt="Round prompt"
            class="block w-full max-h-60 object-contain"
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

    <!-- Right: dark prompt card (tilted other way) -->
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
          Round {round.round_number} prompt
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
        {promptText ?? 'Write the funniest caption.'}
      </p>
      <span class="relative text-[11px] font-bold uppercase tracking-[0.2em] opacity-70 mt-auto">
        Captions are anonymous · voting comes next
      </span>
    </div>
  </div>

  <!-- Composer -->
  <div
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-white p-5 flex flex-col gap-3.5 w-full"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <div class="flex items-center justify-between gap-2">
      <label for="meme-caption-input" class="text-[10px] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        Your caption
      </label>
      <span
        class="inline-flex items-center gap-1.5 rounded-full border-[2.5px] px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.15em]"
        style="border-color: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-border)'}; background: {submitted ? 'rgba(124,181,161,0.15)' : 'var(--brand-surface)'}; color: var(--brand-text);"
      >
        <span
          class="h-2 w-2 rounded-full"
          style="background: {submitted ? 'var(--brand-accent-2)' : 'var(--brand-accent)'};"
        ></span>
        {submitted ? 'Submitted' : 'Drafting'}
      </span>
    </div>

    {#if submitted}
      <div
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-base font-bold text-brand-text text-left leading-snug"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
      >
        {caption.trim()}
      </div>
      <p class="text-xs font-bold text-brand-text-mid text-center m-0">
        Waiting for the others…
      </p>
    {:else}
      <textarea
        id="meme-caption-input"
        bind:value={caption}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={3}
        placeholder="Type the funniest thing you can think of…"
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            submit();
          }
        }}
        class="w-full rounded-[14px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-4 py-3 text-lg font-bold text-brand-text resize-none focus:outline-none focus:border-brand-accent focus:bg-brand-white disabled:opacity-50 transition-colors"
        style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04); line-height: 1.3;"
      ></textarea>

      <div class="flex items-center justify-between gap-2 flex-wrap">
        <span class="font-mono text-xs font-bold text-brand-text-muted tabular-nums">
          {caption.length} / {MAX_CHARS}
        </span>
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="button"
          onclick={submit}
          disabled={submitted || isExpired || caption.trim().length === 0}
          class="h-12 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2 focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.28);"
        >
          <Send size={16} strokeWidth={2.5} />
          Submit caption
        </button>
      </div>
    {/if}
  </div>
</div>
