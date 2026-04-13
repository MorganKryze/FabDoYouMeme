<script lang="ts">
  import { onMount } from 'svelte';
  import { tone } from '$lib/state/tone.svelte';
  import { TONE_LABELS, DEFAULT_TONE, type ToneLevel } from '$lib/content/tonePools';

  const LEVELS: ToneLevel[] = [0, 1, 2, 3, 4];

  let trackEl = $state<HTMLDivElement | null>(null);
  let dragging = $state(false);
  // `dragFraction` drives the thumb position during a drag (0..1).
  // While not dragging, the thumb sits at the snap position for tone.level.
  let dragFraction = $state<number | null>(null);

  // Hydration guard: during SSR and on first client render, `tone.level`
  // on the client-side singleton may not yet reflect localStorage (module
  // init vs. render ordering), and SSR always sees DEFAULT_TONE anyway.
  // Rendering DEFAULT_TONE until onMount prevents an aria-valuenow
  // hydration mismatch warning.
  let mounted = $state(false);
  onMount(() => {
    mounted = true;
  });

  const displayLevel = $derived<ToneLevel>(mounted ? tone.level : DEFAULT_TONE);
  const snapFraction = $derived(displayLevel / 4);
  const thumbFraction = $derived(dragFraction ?? snapFraction);

  const THUMB_HALF = 13; // half of the 26px thumb width

  function fractionFromEvent(e: PointerEvent): number {
    if (!trackEl) return 0;
    const rect = trackEl.getBoundingClientRect();
    // Map pointer position to the inset travel range [THUMB_HALF, width - THUMB_HALF]
    // so drag behaviour matches the thumb's visual position exactly.
    const usable = rect.width - THUMB_HALF * 2;
    const x = e.clientX - rect.left - THUMB_HALF;
    return Math.max(0, Math.min(1, x / usable));
  }

  function fractionToLevel(f: number): ToneLevel {
    // Round to nearest of {0, 0.25, 0.5, 0.75, 1}.
    const idx = Math.round(f * 4);
    return Math.max(0, Math.min(4, idx)) as ToneLevel;
  }

  function onPointerDown(e: PointerEvent) {
    if (!trackEl) return;
    trackEl.setPointerCapture(e.pointerId);
    dragging = true;
    dragFraction = fractionFromEvent(e);
  }

  function onPointerMove(e: PointerEvent) {
    if (!dragging) return;
    dragFraction = fractionFromEvent(e);
  }

  function onPointerUp(e: PointerEvent) {
    if (!dragging) return;
    if (trackEl) trackEl.releasePointerCapture(e.pointerId);
    const finalFraction = dragFraction ?? snapFraction;
    const nextLevel = fractionToLevel(finalFraction);
    tone.setLevel(nextLevel);
    dragging = false;
    dragFraction = null;
  }

  function onPointerCancel() {
    dragging = false;
    dragFraction = null;
  }

  function onKeyDown(e: KeyboardEvent) {
    const lv = tone.level;
    if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') {
      e.preventDefault();
      if (lv > 0) tone.setLevel((lv - 1) as ToneLevel);
    } else if (e.key === 'ArrowRight' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (lv < 4) tone.setLevel((lv + 1) as ToneLevel);
    } else if (e.key === 'Home') {
      e.preventDefault();
      tone.setLevel(0);
    } else if (e.key === 'End') {
      e.preventDefault();
      tone.setLevel(4);
    }
  }
</script>

<div class="wrap">
  <div
    bind:this={trackEl}
    class="track"
    role="slider"
    tabindex="0"
    aria-label="Greeting tone"
    aria-valuemin={0}
    aria-valuemax={4}
    aria-valuenow={displayLevel}
    aria-valuetext={TONE_LABELS[displayLevel]}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerCancel}
    onkeydown={onKeyDown}
  >
    <div class="fill"></div>
    {#each LEVELS as lv (lv)}
      <div class="tick" style="left: calc(13px + (100% - 26px) * {lv / 4});" aria-hidden="true"></div>
    {/each}
    <div
      class="thumb"
      class:dragging
      style="left: calc(13px + (100% - 26px) * {thumbFraction});"
      aria-hidden="true"
    ></div>
  </div>

  <div class="labels" aria-hidden="true">
    {#each LEVELS as lv (lv)}
      <span class="label" class:active={displayLevel === lv}>{TONE_LABELS[lv]}</span>
    {/each}
  </div>
</div>

<style>
  .wrap {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-width: 32rem;
  }

  .track {
    position: relative;
    height: 34px;
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 999px;
    background: var(--brand-white);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.1);
    cursor: pointer;
    touch-action: none;
    user-select: none;
  }

  .track:focus-visible {
    outline: 3px solid var(--brand-text-muted);
    outline-offset: 3px;
  }

  .fill {
    position: absolute;
    inset: 4px;
    border-radius: 999px;
    background: linear-gradient(
      90deg,
      #b5e2d0 0%,
      #d4edda 25%,
      #fddcb5 50%,
      #f9b4ab 75%,
      #d4a5c9 100%
    );
    pointer-events: none;
  }

  .tick {
    position: absolute;
    top: 50%;
    width: 2px;
    height: 60%;
    background: var(--brand-border-heavy);
    transform: translate(-50%, -50%);
    pointer-events: none;
  }

  .thumb {
    position: absolute;
    top: 50%;
    width: 26px;
    height: 26px;
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 999px;
    background: var(--brand-text);
    transform: translate(-50%, -50%);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.2);
    transition:
      left 0.4s cubic-bezier(0.22, 1, 0.36, 1),
      box-shadow 0.15s ease,
      transform 0.15s ease;
    pointer-events: none;
  }

  .thumb.dragging {
    /* Disable the spring transition during drag so the thumb tracks the
       pointer in real time. Snap animation resumes on release. */
    transition: none;
    transform: translate(-50%, calc(-50% - 2px));
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.22);
  }

  .labels {
    display: flex;
    justify-content: space-between;
    padding: 0 2px;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    color: var(--brand-text-muted);
  }

  .label {
    transition: color 0.15s ease, font-weight 0.15s ease;
  }

  .label.active {
    color: var(--brand-text);
    font-weight: 700;
  }
</style>
