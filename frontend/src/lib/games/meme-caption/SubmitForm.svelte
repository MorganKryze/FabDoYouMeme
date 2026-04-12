<script lang="ts">
  import { untrack } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Send } from '$lib/icons';
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
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold w-fit mx-auto"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.14);"
    >
      Submission window has closed.
    </div>
  {:else}
    <div class="flex flex-col items-center gap-3">
      <!-- Brand timer pill -->
      <div
        class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-8 py-3 w-fit"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.18);"
      >
        <span
          class="h-3 w-3 rounded-full"
          style="background: var(--brand-accent); animation: pulse-dot 1.5s ease-in-out infinite;"
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

  <!-- Media prompt (if present) -->
  {#if round.item?.media_url ?? round.media_url}
    <img
      src={round.item?.media_url ?? round.media_url}
      alt="Round prompt"
      class="w-full aspect-video object-cover rounded-[22px] border-[2.5px] border-brand-border-heavy"
    />
  {/if}

  {#if round.text_prompt}
    <p class="text-center text-brand-text-mid font-semibold italic">"{round.text_prompt}"</p>
  {/if}

  <!-- Caption input -->
  {#if submitted}
    <div
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 text-center text-sm font-bold text-brand-text-mid"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      Submitted — waiting for others…
    </div>
  {:else}
    <div class="flex flex-col gap-2">
      <textarea
        bind:value={caption}
        disabled={isExpired}
        maxlength={MAX_CHARS}
        rows={3}
        placeholder="Write your caption…"
        class="w-full rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white p-4 text-sm font-semibold resize-none focus:outline-none focus:border-brand-text transition-colors disabled:opacity-50"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
      ></textarea>
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
    </div>
  {/if}

  <!-- Player submission status -->
  <div class="flex flex-wrap gap-2">
    {#each room.players as player}
      {@const hasSub = room.submissions.some((s) => s.user_id === player.user_id)}
      <span
        class="flex items-center gap-1 text-xs font-bold px-3 py-1.5 rounded-full border-[2.5px]
          {hasSub
            ? 'border-brand-border-heavy bg-brand-white text-brand-text'
            : 'border-brand-border bg-brand-surface text-brand-text-muted'}"
      >
        {hasSub ? '\u2713' : '\u23F3'} {player.username}
      </span>
    {/each}
  </div>
</div>
