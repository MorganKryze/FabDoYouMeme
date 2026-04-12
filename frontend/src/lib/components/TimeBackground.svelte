<script lang="ts">
  import { theme } from '$lib/state/theme.svelte';

  /**
   * Time-aware animated gradient background + grain overlay.
   *
   * Reads `theme.active` (user-override aware) and smoothly transitions
   * :root CSS custom properties between time bands. The 2s transition on
   * :root (in app.css) ensures shifts are imperceptible.
   *
   * The clock-to-band mapping and the clock tick both live in
   * `theme.svelte.ts` — this component only applies the palette when
   * `theme.active` changes.
   *
   * Note: the `afternoon` palette below is retained as a placeholder for
   * the (still-pending) afternoon time band. ThemeState currently collapses
   * 6:00–17:00 into `morning`, so `afternoon` is unreachable dead code
   * until the afternoon palette decision lands.
   */

  interface TimePalette {
    grad: [string, string, string, string];
    text: string;
    textMid: string;
    textMuted: string;
    white: string;
    surface: string;
    border: string;
    borderHeavy: string;
  }

  const palettes: Record<string, TimePalette> = {
    morning: {
      grad: ['#D4EDDA', '#B5E2D0', '#FDDCB5', '#A8D8EA'],
      text: '#1A1A1A',
      textMid: '#3A3A3A',
      textMuted: 'rgba(26,26,26,0.4)',
      white: '#FEFEFE',
      surface: 'rgba(255,255,255,0.82)',
      border: 'rgba(26,26,26,0.18)',
      borderHeavy: 'rgba(26,26,26,0.7)',
    },
    afternoon: {
      // Blend of morning + evening gradient stops
      grad: ['#E9E5CA', '#D7CBB1', '#ECBDB0', '#B5E2D0'],
      text: '#1A1A1A',
      textMid: '#3A3A3A',
      textMuted: 'rgba(26,26,26,0.4)',
      white: '#FEFEFE',
      surface: 'rgba(255,255,255,0.82)',
      border: 'rgba(26,26,26,0.18)',
      borderHeavy: 'rgba(26,26,26,0.7)',
    },
    evening: {
      grad: ['#FDDCB5', '#F9B4AB', '#D4A5C9', '#B5E2D0'],
      text: '#1A1A1A',
      textMid: '#3A3A3A',
      textMuted: 'rgba(26,26,26,0.4)',
      white: '#FEFEFE',
      surface: 'rgba(255,255,255,0.82)',
      border: 'rgba(26,26,26,0.18)',
      borderHeavy: 'rgba(26,26,26,0.7)',
    },
    night: {
      // Slightly lifted dark purples so the gradient stays visible (was nearly black)
      grad: ['#2A2040', '#3D2B5A', '#4A3470', '#1B2838'],
      text: '#F2EBFF',
      textMid: '#C8BCE0',
      textMuted: 'rgba(242,235,255,0.55)',
      // bg-brand-white in night = lifted dark purple panel
      white: '#3A2D55',
      // Surface is the card body — needs real opacity to be readable
      surface: 'rgba(58,45,85,0.72)',
      border: 'rgba(255,255,255,0.18)',
      borderHeavy: 'rgba(255,255,255,0.55)',
    },
  };

  function applyPalette(band: string) {
    const p = palettes[band];
    const root = document.documentElement.style;
    root.setProperty('--brand-text', p.text);
    root.setProperty('--brand-text-mid', p.textMid);
    root.setProperty('--brand-text-muted', p.textMuted);
    root.setProperty('--brand-white', p.white);
    root.setProperty('--brand-surface', p.surface);
    root.setProperty('--brand-border', p.border);
    root.setProperty('--brand-border-heavy', p.borderHeavy);
    root.setProperty('--brand-grad-1', p.grad[0]);
    root.setProperty('--brand-grad-2', p.grad[1]);
    root.setProperty('--brand-grad-3', p.grad[2]);
    root.setProperty('--brand-grad-4', p.grad[3]);
  }

  $effect(() => {
    if (typeof document !== 'undefined') {
      applyPalette(theme.active);
    }
  });

  const grainSvg =
    "data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E";
</script>

<!--
  Grain texture overlay only.
  The animated gradient is owned by the <body> in app.css — that avoids any
  z-index/stacking ambiguity (the gradient becomes the canvas background).
  This component still owns the JS that updates :root variables by clock band.
-->
<div
  class="fixed inset-0 z-[1] pointer-events-none mix-blend-overlay"
  style="
    opacity: 0.22;
    background-image: url('{grainSvg}');
    background-size: 128px 128px;
  "
  aria-hidden="true"
></div>
