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
  const totalMs = $derived(round.duration_seconds * 1000);
  const mountedExpired = $derived(deadline <= Date.now());
  // Seed the timer from the current deadline; subsequent updates are
  // driven by the requestAnimationFrame loop in the $effect below.
  let timerMs = $state(untrack(() => Math.max(0, deadline - Date.now())));

  $effect(() => {
    if (mountedExpired) return;
    const tick = () => {
      if (room.roundPaused) return; // timer frozen while all players are reconnecting
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
    const trimmed = caption.trim();
    ws.send('meme-caption:submit', { caption: trimmed });
    submitted = true;
  }
</script>

<div class="flex flex-col gap-6">
  <!-- Timer -->
  {#if mountedExpired}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold w-fit mx-auto"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.14);"
    >
      Submission window has closed.
    </div>
  {:else}
    {#if room.roundPaused}
      <div
        class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2 text-xs font-bold text-brand-text-muted text-center w-fit mx-auto"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        Everyone dropped — timer paused
      </div>
    {/if}
    <div class="flex flex-col items-center gap-3">
      <!-- Brand timer pill -->
      <div
        class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-8 py-3 w-fit"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.18);"
      >
        <span
          class="h-3 w-3 rounded-full"
          style="background: var(--brand-accent); animation: {room.roundPaused ? 'none' : 'pulse-dot 1.5s ease-in-out infinite'};"
        ></span>
        <span
          class="text-[2.4rem] font-bold tabular-nums leading-none tracking-wide"
          role="timer"
          aria-label="Time remaining"
        >
          {Math.floor(secondsLeft / 60)}:{(secondsLeft % 60).toString().padStart(2, '0')}
        </span>
      </div>
      <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
        Round {round.round_number}
      </span>
    </div>
  {/if}

  <!-- Pokémon-style card: image + caption slot live inside one bordered frame.
       Narrow max-width keeps it feeling like a held card, not a banner. -->
  <div class="w-full max-w-md mx-auto flex flex-col gap-3">
    <div
      use:dealCard={{ delay: 80, rotate: -2 }}
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-4 flex flex-col gap-3"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <!-- Image slot -->
      {#if round.item?.media_url}
        <div
          class="w-full rounded-[14px] overflow-hidden border-[2.5px] border-brand-border-heavy bg-brand-white flex items-center justify-center"
          style="box-shadow: inset 0 2px 0 rgba(0,0,0,0.04);"
        >
          <img
            src={mediaSrc(round.item.media_url, room.code)}
            alt="Round prompt"
            class="block w-full max-h-72 object-contain"
          />
        </div>
      {/if}

      {#if (round.item?.payload as { prompt?: string } | undefined)?.prompt}
        <p class="text-center text-brand-text-mid font-semibold italic text-sm">
          "{(round.item.payload as { prompt?: string }).prompt}"
        </p>
      {/if}

      <!-- Caption slot — dashed while empty so it reads as a fillable field -->
      {#if submitted}
        <div
          class="w-full rounded-[12px] border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2 text-sm font-semibold text-brand-text text-center"
          style="box-shadow: 0 2px 0 rgba(0,0,0,0.04);"
        >
          {caption.trim()}
        </div>
      {:else}
        <textarea
          bind:value={caption}
          disabled={isExpired}
          maxlength={MAX_CHARS}
          rows={3}
          placeholder="Write your caption…"
          onkeydown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              submit();
            }
          }}
          class="w-full rounded-[12px] border-[2.5px] border-dashed border-brand-border-heavy bg-brand-white/70 px-3 py-3 text-sm font-semibold text-center resize-none focus:outline-none focus:border-solid focus:bg-brand-white transition-colors disabled:opacity-50"
        ></textarea>
      {/if}
    </div>

    <!-- Counter + submit live below the card so the card stays the subject. -->
    {#if submitted}
      <p class="text-center text-xs font-bold text-brand-text-mid">
        Submitted — waiting for others…
      </p>
    {:else}
      <div class="flex items-center justify-between">
        <span class="text-xs font-semibold text-brand-text-muted">{caption.length}/{MAX_CHARS}</span>
        <button
          use:pressPhysics={'dark'}
          use:hoverEffect={'gradient'}
          type="button"
          onclick={submit}
          disabled={submitted || isExpired || caption.trim().length === 0}
          class="h-11 px-7 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold disabled:opacity-50 cursor-pointer inline-flex items-center gap-2"
        >
          <Send size={16} strokeWidth={2.5} />
          Submit
        </button>
      </div>
    {/if}
  </div>
</div>
