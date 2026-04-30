<script lang="ts">
  // Renders a sentence-with-blank where the blank is either an empty input
  // placeholder, or filled with a player-supplied text. Used by every prompt-*
  // game type's submit / vote / results / replay views, so the splice logic
  // lives in a single component instead of being duplicated.
  //
  // The blank always renders as an underline so it stays visually obvious even
  // when filled — the joke depends on knowing where the gap was.

  let {
    prefix = '',
    suffix = '',
    filler = null,
    placeholder = null,
    size = 'md',
  }: {
    prefix?: string | null;
    suffix?: string | null;
    /** When set, renders the value spliced into the blank. */
    filler?: string | null;
    /** When `filler` is null, this is the muted text shown in the blank slot. */
    placeholder?: string | null;
    size?: 'sm' | 'md' | 'lg';
  } = $props();

  const safePrefix = $derived(prefix ?? '');
  const safeSuffix = $derived(suffix ?? '');
  const blankWidth = $derived.by(() => {
    if (filler && filler.length > 0) return undefined;
    // Idle blank width scales with placeholder so empty cards still feel
    // balanced regardless of caller's locale.
    const len = (placeholder ?? '').length;
    return Math.max(4, Math.min(12, Math.ceil(len / 3)));
  });
</script>

<span class="swb" data-size={size}>
  {#if safePrefix}
    <span class="swb-prefix">{safePrefix}</span>
  {/if}
  <span class="swb-blank" class:is-filled={!!(filler && filler.length > 0)}>
    {#if filler && filler.length > 0}
      <span class="swb-filler">{filler}</span>
    {:else}
      <span class="swb-placeholder" style={blankWidth ? `min-width: ${blankWidth}ch;` : ''}>
        {placeholder ?? ''}
      </span>
    {/if}
  </span>
  {#if safeSuffix}
    <span class="swb-suffix">{safeSuffix}</span>
  {/if}
</span>

<style>
  .swb {
    display: inline;
    line-height: 1.35;
    font-weight: 700;
    color: inherit;
    text-wrap: balance;
  }
  .swb[data-size="sm"] { font-size: 0.95rem; }
  .swb[data-size="md"] { font-size: clamp(1.05rem, 2vw, 1.35rem); }
  .swb[data-size="lg"] { font-size: clamp(1.35rem, 2.6vw, 2rem); }

  .swb-prefix,
  .swb-suffix {
    white-space: pre-wrap;
  }

  .swb-blank {
    /* Idle blank uses a continuous underline — `inline` (not inline-block)
     * so the underline collapses naturally to a single short run. The
     * horizontal padding gives the underline visible width even when the
     * placeholder is short. */
    display: inline;
    padding: 0 0.4em;
    line-height: 1.15;
    font-weight: 700;
    background-image: linear-gradient(currentColor, currentColor);
    background-repeat: no-repeat;
    background-position: 0 100%;
    background-size: 100% 2.5px;
    transition: background-color 220ms ease;
  }
  .swb-blank.is-filled {
    /* When filled, the wrapper contributes no whitespace at all — the
     * filler's own highlighter band carries the visual identity, and the
     * prefix/suffix already include their own trailing/leading spaces. Any
     * extra padding/margin here would double-pad the gap (the bug in the
     * screenshot). */
    display: inline;
    padding: 0;
    background-image: none;
  }
  .swb-filler {
    color: inherit;
    font-weight: 800;
    /* `box-decoration-break: clone` repeats the highlighter band on every
     * line of a wrapped filler — without it the band only paints the first
     * line and the rest look unmarked. */
    background: linear-gradient(
      rgba(232, 147, 127, 0.28),
      rgba(232, 147, 127, 0.28)
    );
    /* No horizontal padding: the surrounding prefix/suffix already supply
     * a trailing and leading space, so adding more here would inflate the
     * gap on either side of the highlight. */
    padding: 0.05em 0;
    border-radius: 0.2em;
    box-decoration-break: clone;
    -webkit-box-decoration-break: clone;
  }
  .swb-placeholder {
    display: inline-block;
    opacity: 0.45;
    font-style: italic;
  }
</style>
