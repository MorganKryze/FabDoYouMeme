<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { Volume2, VolumeX, XCircle } from '$lib/icons';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { music } from '$lib/state/music.svelte';
  import * as m from '$lib/paraglide/messages';

  // On `/rooms/*` the music button is rendered inline inside RoomHeader
  // (top sticky card) — keep the audio element mounted globally but
  // suppress this component's floating UI so we don't render two buttons.
  const inRoom = $derived($page.url.pathname.startsWith('/rooms/'));

  const TRACK_AMBIENT = '/audio/monument_music-pure-159612.mp3';
  const TRACK_GAMEPLAY = '/audio/moodmode-for-fashion-luxury-223930.mp3';

  const STORAGE_KEY = 'bg-music:v3';
  const LEVELS = 5;
  const DEFAULT_LEVEL = 2;
  // Level → element volume. Curve picked so steps are clearly audible
  // (perceived loudness is roughly logarithmic, so a power curve gives
  // even-feeling jumps across the range): 1 ≈ 9%, 2 ≈ 25%, 3 ≈ 41%,
  // 4 ≈ 59%, 5 ≈ 80%. Capped under 1.0 so it stays "background".
  const volumeFor = (level: number) =>
    Math.max(0.05, Math.min(0.85, Math.pow(level / LEVELS, 1.4) * 0.85));
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
  let floatingWrap: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (!showSlider) return;
    function onDocClick(e: MouseEvent) {
      if (floatingWrap && !floatingWrap.contains(e.target as Node)) {
        showSlider = false;
      }
    }
    document.addEventListener('click', onDocClick);
    return () => document.removeEventListener('click', onDocClick);
  });
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
            error = m.room_music_error({ reason: mutedErr?.message ?? err?.message ?? err });
            console.warn('[bg-music] play() rejected', { err, mutedErr });
          });
      });
  }

  // Mirror reactive state into the music singleton so consumers (the
  // inline RoomHeader button) read the same flags without their own
  // wiring. Effects run after every change, so this stays in lockstep.
  $effect(() => { music.playing = playing; });
  $effect(() => { music.muted = muted; });
  $effect(() => { music.level = level; });

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

    // Bind command handlers + advertise availability. RoomHeader gates
    // its rendering on `music.available` so it can never call a stale
    // closure if this component happens to remount.
    music.toggleHandler = toggle;
    music.setLevelHandler = setLevel;
    music.available = true;

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
      music.toggleHandler = null;
      music.setLevelHandler = null;
      music.available = false;
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
            error = m.room_music_error({ reason: err?.message ?? err });
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
    if (!audioEl || !playing) return;
    // Treat a level click as an explicit "I want to hear this" gesture
    // — even if the browser had us in muted-autoplay fallback.
    audioEl.muted = false;
    muted = false;
    if (audioEl.paused) {
      startPlayback(FADE_MS);
      return;
    }
    // Cancel any in-flight fade and apply the new volume *immediately*.
    // Relying on the $effect alone is fragile: it skips while
    // `fadeRaf !== null`, which means level clicks landing during a
    // crossfade or initial fade-in were silently dropped.
    if (fadeRaf !== null) {
      cancelAnimationFrame(fadeRaf);
      fadeRaf = null;
    }
    audioEl.volume = volumeFor(level);
  }

  function onAudioError(e: Event) {
    const el = e.currentTarget as HTMLAudioElement;
    const code = el.error?.code;
    const msg = el.error?.message || 'unknown';
    error = m.room_music_error_code({ code: code ?? 0 });
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
<div class="fixed bottom-24 right-6 z-40 flex flex-col items-end gap-2 pointer-events-none">
  {#if error}
    <div
      class="pointer-events-auto bg-brand-white border-[2.5px] border-red-400 rounded-2xl pl-3 pr-2 py-2 text-xs font-semibold text-red-600 max-w-xs inline-flex items-start gap-2"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
    >
      <span class="leading-snug">{error}</span>
      <button
        type="button"
        onclick={() => (error = null)}
        class="shrink-0 opacity-50 hover:opacity-100 transition-opacity inline-flex items-center cursor-pointer"
        aria-label={m.common_dismiss()}
      >
        <XCircle size={14} strokeWidth={2.5} />
      </button>
    </div>
  {/if}

  <!--
    Floating button only shows off-room. On `/rooms/*` the same controls
    are rendered inline inside RoomHeader. Error banner stays visible in
    either context so a play() failure can't be missed.
  -->
  {#if !inRoom}
    <!-- Floating slot: same click-only behaviour as the inline RoomHeader
         control (hover-toggle proved fragile — pointer crossing the gap
         to the slider closed the popover before the user could click a
         level). Click toggles the slider; click-outside dismisses. The
         mute/play pill lives inside the slider so play/pause is always
         reachable without having to first re-discover the main button. -->
    <div bind:this={floatingWrap} class="pointer-events-auto relative">
      {#if showSlider}
        <div
          class="absolute right-0 bottom-full mb-2 z-30 bg-brand-white border-[2.5px] border-brand-border-heavy rounded-2xl p-2 flex items-center gap-1 whitespace-nowrap"
          style="box-shadow: 0 6px 0 rgba(0,0,0,0.12);"
          role="group"
          aria-label={m.room_music_volume_aria()}
        >
          <button
            type="button"
            onclick={toggle}
            class="h-8 w-8 mr-1 shrink-0 rounded-full border-[2px] border-brand-border-heavy {playing ? 'bg-brand-surface text-brand-text-mid' : 'bg-brand-text text-brand-white'} inline-flex items-center justify-center cursor-pointer"
            aria-label={playing ? m.room_music_mute_aria() : m.room_music_play_aria()}
            aria-pressed={!playing}
          >
            {#if playing}
              <VolumeX size={14} strokeWidth={2.5} />
            {:else}
              <Volume2 size={14} strokeWidth={2.5} />
            {/if}
          </button>
          {#each Array(LEVELS) as _, i (i)}
            {@const n = i + 1}
            {@const active = playing && level >= n}
            <button
              type="button"
              onclick={() => setLevel(n)}
              disabled={!playing}
              class="w-7 h-8 shrink-0 flex items-end justify-center cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
              aria-label={m.room_music_volume_level_aria({ level: n })}
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
        onclick={() => (showSlider = !showSlider)}
        class="h-11 w-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white text-brand-text-mid hover:text-brand-accent inline-flex items-center justify-center cursor-pointer transition-colors"
        title={playing ? m.room_music_mute_title() : m.room_music_play_title()}
        aria-label={m.room_music_volume_aria()}
        aria-expanded={showSlider}
      >
        {#if playing && !muted}
          <Volume2 size={18} strokeWidth={2.5} />
        {:else}
          <VolumeX size={18} strokeWidth={2.5} />
        {/if}
      </button>
    </div>
  {/if}
</div>
