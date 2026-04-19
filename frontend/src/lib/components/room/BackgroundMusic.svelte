<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { Volume2, VolumeX, XCircle } from '$lib/icons';
  import { pressPhysics } from '$lib/actions/pressPhysics';

  const TRACK_AMBIENT = '/audio/monument_music-pure-159612.mp3';
  const TRACK_GAMEPLAY = '/audio/moodmode-for-fashion-luxury-223930.mp3';

  const STORAGE_KEY = 'bg-music:v3';
  const LEVELS = 5;
  const DEFAULT_LEVEL = 1;
  // Level → volume. Keeps background quiet even at max — 1 ≈ 7%, 5 ≈ 35%.
  const volumeFor = (level: number) => level * 0.07;
  const FADE_MS = 350;
  const CROSSFADE_OUT_MS = 450;
  const CROSSFADE_IN_MS = 450;

  let playing = $state(true);
  // Tracks the *audible* state, not user intent. When the browser forces a
  // muted-autoplay fallback, `playing` stays true (user wants music) but
  // `muted` flips so the icon reflects reality until a gesture unmutes us.
  let muted = $state(false);
  let level = $state(DEFAULT_LEVEL);
  let showSlider = $state(false);
  let error = $state<string | null>(null);
  let audioEl: HTMLAudioElement | undefined = $state();
  let currentSrc = $state('');

  const targetSrc = $derived(
    $page.url.pathname.startsWith('/rooms/') ? TRACK_GAMEPLAY : TRACK_AMBIENT
  );

  $effect(() => {
    if (typeof window === 'undefined') return;
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ playing, level }));
  });

  // Level changes: apply immediately unless a fade is in flight.
  $effect(() => {
    if (!audioEl) return;
    if (fadeRaf === null && !audioEl.paused && !audioEl.muted) {
      audioEl.volume = volumeFor(level);
    }
  });

  // Track-change orchestrator: whenever the route's target src diverges
  // from what's currently loaded, fade the old one down, swap, and fade
  // the new one up. When nothing is playing yet we skip the fade-out step.
  $effect(() => {
    const target = targetSrc;
    if (!audioEl) return;
    if (currentSrc === target) return;

    const actuallyAudible = !audioEl.paused && !audioEl.muted && playing;

    if (!actuallyAudible) {
      swapSrc(target);
      if (playing && !isHidden()) startPlayback();
      return;
    }

    fadeTo(0, CROSSFADE_OUT_MS, () => {
      swapSrc(target);
      if (playing && !isHidden()) startPlayback();
    });
  });

  function swapSrc(next: string) {
    if (!audioEl) return;
    currentSrc = next;
    audioEl.src = next;
  }

  function isHidden() {
    return typeof document !== 'undefined' && document.visibilityState === 'hidden';
  }

  // Fade audioEl.volume toward `to`. Cosine ease-in-out — linear volume
  // ramps sound unnatural at the ends. `to` may be a getter so fade-ins
  // track the current level even if the user clicks a new bar mid-fade.
  let fadeRaf: number | null = null;
  function fadeTo(to: number | (() => number), duration: number, onDone?: () => void) {
    if (!audioEl) return;
    if (fadeRaf !== null) cancelAnimationFrame(fadeRaf);
    const from = audioEl.volume;
    const start = performance.now();
    const step = (now: number) => {
      if (!audioEl) return;
      const t = Math.min(1, (now - start) / duration);
      const eased = 0.5 - 0.5 * Math.cos(Math.PI * t);
      const target = typeof to === 'function' ? to() : to;
      audioEl.volume = Math.max(0, Math.min(1, from + (target - from) * eased));
      if (t < 1) {
        fadeRaf = requestAnimationFrame(step);
      } else {
        fadeRaf = null;
        onDone?.();
      }
    };
    fadeRaf = requestAnimationFrame(step);
  }

  // Start (or restart) playback with fade-in. Handles muted-autoplay
  // fallback if the browser refuses an unmuted start.
  let gestureCleanup: (() => void) | null = null;
  function startPlayback(fadeMs: number = CROSSFADE_IN_MS) {
    if (!audioEl) return;
    audioEl.muted = false;
    muted = false;
    audioEl.volume = 0;
    audioEl.play()
      .then(() => fadeTo(() => volumeFor(level), fadeMs))
      .catch((err) => {
        // Any pre-gesture rejection is almost certainly an autoplay block.
        // Mobile browsers don't all name it NotAllowedError (Samsung Internet,
        // older WebViews, iOS versions differ), so we always try the muted
        // fallback and only surface a banner if that ALSO fails.
        if (!audioEl) return;
        audioEl.muted = true;
        muted = true;
        audioEl.play()
          .then(() => {
            gestureCleanup?.();
            gestureCleanup = armGestureUnmute();
          })
          .catch((mutedErr) => {
            error = `Audio: ${mutedErr?.message ?? err?.message ?? err}`;
            console.warn('[bg-music] play() rejected', { err, mutedErr });
          });
      });
  }

  onMount(() => {
    try {
      const raw = window.localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const saved = JSON.parse(raw) as { playing?: boolean; level?: number };
        if (typeof saved.playing === 'boolean') playing = saved.playing;
        if (typeof saved.level === 'number') {
          level = Math.min(LEVELS, Math.max(1, Math.round(saved.level)));
        }
      }
    } catch {
      // ignore malformed state
    }

    // The track-change effect handles initial src load + first play. Nothing
    // else needed here besides visibility wiring.

    const onVis = () => {
      if (!audioEl) return;
      if (isHidden()) {
        // rAF throttles in hidden tabs, so a fade-out would stall. Cut cleanly.
        if (fadeRaf !== null) cancelAnimationFrame(fadeRaf);
        fadeRaf = null;
        audioEl.pause();
      } else if (playing) {
        startPlayback(FADE_MS);
      }
    };
    document.addEventListener('visibilitychange', onVis);

    return () => {
      document.removeEventListener('visibilitychange', onVis);
      gestureCleanup?.();
    };
  });

  function armGestureUnmute(): () => void {
    const unmute = () => {
      if (!audioEl || !playing) return;
      if (!audioEl.muted && !audioEl.paused) {
        muted = false;
        cleanup();
        return;
      }
      audioEl.muted = false;
      muted = false;
      if (audioEl.paused) {
        audioEl.volume = 0;
        audioEl.play()
          .then(() => fadeTo(() => volumeFor(level), FADE_MS))
          .catch((err) => {
            error = `Audio: ${err?.message ?? err}`;
            console.warn('[bg-music] play() rejected after gesture', err);
          });
      } else {
        fadeTo(() => volumeFor(level), FADE_MS);
      }
      cleanup();
    };
    const cleanup = () => {
      window.removeEventListener('pointerdown', unmute);
      window.removeEventListener('keydown', unmute);
      window.removeEventListener('touchstart', unmute);
    };
    window.addEventListener('pointerdown', unmute);
    window.addEventListener('keydown', unmute);
    window.addEventListener('touchstart', unmute);
    return cleanup;
  }

  function toggle() {
    if (!audioEl) return;
    if (playing) {
      playing = false;
      fadeTo(0, FADE_MS, () => audioEl?.pause());
    } else {
      playing = true;
      startPlayback(FADE_MS);
    }
  }

  function setLevel(n: number) {
    level = Math.min(LEVELS, Math.max(1, n));
    if (audioEl && playing) {
      audioEl.muted = false;
      muted = false;
      if (audioEl.paused) {
        startPlayback(FADE_MS);
      }
    }
  }

  function onAudioError(e: Event) {
    const el = e.currentTarget as HTMLAudioElement;
    const code = el.error?.code;
    const msg = el.error?.message || 'unknown';
    error = `Audio failed (code ${code})`;
    console.error('[bg-music] <audio> error', { code, msg, src: currentSrc });
    playing = false;
  }
</script>

<audio
  bind:this={audioEl}
  loop
  preload="auto"
  onerror={onAudioError}
  onplaying={() => (error = null)}
></audio>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="fixed bottom-6 right-6 z-40 flex flex-col items-end gap-2">
  {#if error}
    <div
      class="bg-brand-white border-[2.5px] border-red-400 rounded-2xl pl-3 pr-2 py-2 text-xs font-semibold text-red-600 max-w-xs inline-flex items-start gap-2"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <span class="leading-snug">{error}</span>
      <button
        type="button"
        onclick={() => (error = null)}
        class="shrink-0 opacity-50 hover:opacity-100 transition-opacity inline-flex items-center cursor-pointer"
        aria-label="Dismiss"
      >
        <XCircle size={14} strokeWidth={2.5} />
      </button>
    </div>
  {/if}

  <!--
    w-11 locks the hover column to button width so the slider (which is
    naturally wider thanks to its padding) overflows symmetrically on both
    sides instead of shoving the button leftward when it appears.
  -->
  <div
    class="w-11 flex flex-col items-center gap-2"
    onmouseenter={() => (showSlider = true)}
    onmouseleave={() => (showSlider = false)}
    onfocusin={() => (showSlider = true)}
    onfocusout={() => (showSlider = false)}
  >
    {#if showSlider && playing}
      <div
        class="bg-brand-white border-[2.5px] border-brand-border-heavy rounded-2xl p-2 flex flex-col-reverse"
        style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
        role="group"
        aria-label="Music volume"
      >
        {#each Array(LEVELS) as _, i (i)}
          {@const n = i + 1}
          {@const active = level >= n}
          <button
            type="button"
            onclick={() => setLevel(n)}
            class="w-8 h-6 flex items-end justify-center cursor-pointer"
            aria-label="Volume level {n}"
            aria-pressed={level === n}
          >
            <span
              class="block w-full rounded-sm transition-colors {active ? 'bg-brand-accent' : 'bg-brand-border'}"
              style="height: {6 + i * 3}px;"
            ></span>
          </button>
        {/each}
      </div>
    {/if}

    <button
      use:pressPhysics={'ghost'}
      type="button"
      onclick={toggle}
      class="h-11 w-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid hover:text-brand-accent inline-flex items-center justify-center cursor-pointer transition-colors"
      title={playing ? 'Mute music' : 'Play music'}
      aria-label={playing ? 'Mute background music' : 'Play background music'}
      aria-pressed={playing}
    >
      {#if playing && !muted}
        <Volume2 size={18} strokeWidth={2.5} />
      {:else}
        <VolumeX size={18} strokeWidth={2.5} />
      {/if}
    </button>
  </div>
</div>
