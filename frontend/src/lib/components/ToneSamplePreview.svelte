<script lang="ts">
  import { onMount } from 'svelte';
  import { tone } from '$lib/state/tone.svelte';
  import { pickForSlot } from '$lib/content/toneSelect';
  import { TONE_LABELS, DEFAULT_TONE, type TonePair, type ToneLevel } from '$lib/content/tonePools';
  import { Shuffle } from '$lib/icons';

  let { username }: { username: string } = $props();

  // Null until mount — this avoids a hydration mismatch where the SSR
  // render would bake in a different randomly-picked sample than the
  // client (since Math.random() is non-deterministic).
  let sample = $state<TonePair | null>(null);
  let mounted = $state(false);

  onMount(() => {
    mounted = true;
  });

  // Repick whenever the tone level changes, but only after mount.
  $effect(() => {
    if (!mounted) return;
    const level = tone.level; // register reactivity on tone.level
    sample = pickForSlot('home_greeting', level);
  });

  function shuffle() {
    sample = pickForSlot('home_greeting', tone.level);
  }

  const displayLevel = $derived<ToneLevel>(mounted ? tone.level : DEFAULT_TONE);
  const renderedH1 = $derived(
    sample ? sample.h1.replaceAll('{username}', username) : null
  );
  const renderedSub = $derived(
    sample ? sample.subline.replaceAll('{username}', username) : null
  );
</script>

<div class="card" aria-live="polite">
  <div class="header">
    <span class="eyebrow">A taste — {TONE_LABELS[displayLevel]}</span>
    <button
      type="button"
      class="shuffle"
      aria-label="Shuffle sample greeting"
      onclick={shuffle}
      disabled={!mounted}
    >
      <Shuffle size={14} strokeWidth={2.5} />
    </button>
  </div>
  {#if renderedH1 && renderedSub}
    <h3 class="h1">{renderedH1}</h3>
    <p class="sub">{renderedSub}</p>
  {:else}
    <!-- Pre-mount skeleton — reserves layout, reveals on mount -->
    <h3 class="h1 blur-sm opacity-40 select-none">Sample greeting headline.</h3>
    <p class="sub blur-sm opacity-40 select-none">A matching subline goes here.</p>
  {/if}
</div>

<style>
  .card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 16px 18px;
    border: 2.5px dashed var(--brand-border-heavy);
    border-radius: 22px;
    background: var(--brand-surface);
    max-width: 32rem;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .eyebrow {
    font-size: 0.6rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.2em;
    color: var(--brand-text-muted);
  }

  .shuffle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: 2px solid var(--brand-border-heavy);
    border-radius: 999px;
    background: var(--brand-white);
    cursor: pointer;
    box-shadow: 0 2px 0 rgba(0, 0, 0, 0.12);
    transition: transform 0.1s ease, box-shadow 0.1s ease;
  }

  .shuffle:hover {
    transform: translateY(-1px);
    box-shadow: 0 3px 0 rgba(0, 0, 0, 0.15);
  }

  .shuffle:active {
    transform: translateY(1px);
    box-shadow: 0 1px 0 rgba(0, 0, 0, 0.15);
  }

  .h1 {
    font-size: 1.4rem;
    font-weight: 700;
    line-height: 1.2;
    color: var(--brand-text);
    margin: 0;
    min-height: 1.4em; /* reserve space to dampen jitter on repick */
  }

  .sub {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--brand-text-muted);
    margin: 0;
    min-height: 1.2em;
  }
</style>
