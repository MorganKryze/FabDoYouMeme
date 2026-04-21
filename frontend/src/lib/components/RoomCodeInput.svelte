<script lang="ts">
  import * as m from '$lib/paraglide/messages';

  interface Props {
    value?: string;
    name?: string;
    autofocus?: boolean;
    onenter?: (code: string) => void;
  }

  let {
    value = $bindable(''),
    name = 'code',
    autofocus = false,
    onenter
  }: Props = $props();

  // Four letter tiles matching the RoomHeader code chip style. Each tile
  // accepts a single A-Z/0-9 character, auto-advances to the next tile on
  // type, and supports backspace-back and full paste fills.
  const SLOT_COUNT = 4;
  let tileEls = $state<(HTMLInputElement | null)[]>(Array(SLOT_COUNT).fill(null));

  // Derive the per-tile character from `value` so external updates (like
  // paste handlers on a parent or programmatic resets) stay in sync.
  const slots = $derived(
    Array.from({ length: SLOT_COUNT }, (_, i) => value[i] ?? '')
  );

  $effect(() => {
    if (autofocus) tileEls[0]?.focus();
  });

  function setChar(index: number, char: string) {
    const cleaned = char.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 1);
    const chars = Array.from({ length: SLOT_COUNT }, (_, i) => value[i] ?? '');
    chars[index] = cleaned;
    value = chars.join('').slice(0, SLOT_COUNT);
  }

  function focusSlot(index: number) {
    const clamped = Math.max(0, Math.min(SLOT_COUNT - 1, index));
    tileEls[clamped]?.focus();
    tileEls[clamped]?.select();
  }

  function onInput(index: number, e: Event) {
    const input = e.target as HTMLInputElement;
    const raw = input.value.toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (raw.length === 0) {
      setChar(index, '');
      return;
    }
    // Paste-into-one-slot scenario: splay the characters across subsequent slots.
    if (raw.length > 1) {
      const chars = Array.from({ length: SLOT_COUNT }, (_, i) => value[i] ?? '');
      for (let i = 0; i < raw.length && index + i < SLOT_COUNT; i++) {
        chars[index + i] = raw[i];
      }
      value = chars.join('').slice(0, SLOT_COUNT);
      focusSlot(Math.min(index + raw.length, SLOT_COUNT - 1));
      return;
    }
    setChar(index, raw);
    if (index < SLOT_COUNT - 1) focusSlot(index + 1);
  }

  function onKeydown(index: number, e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (onenter) onenter(value);
      return;
    }
    if (e.key === 'Backspace') {
      if (slots[index]) {
        // Slot has a char — let the default clear handle it.
        return;
      }
      e.preventDefault();
      if (index > 0) {
        setChar(index - 1, '');
        focusSlot(index - 1);
      }
      return;
    }
    if (e.key === 'ArrowLeft') {
      e.preventDefault();
      focusSlot(index - 1);
      return;
    }
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      focusSlot(index + 1);
      return;
    }
  }

  function onPaste(index: number, e: ClipboardEvent) {
    const text = e.clipboardData?.getData('text') ?? '';
    const cleaned = text.toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (!cleaned) return;
    e.preventDefault();
    const chars = Array.from({ length: SLOT_COUNT }, (_, i) => value[i] ?? '');
    for (let i = 0; i < cleaned.length && index + i < SLOT_COUNT; i++) {
      chars[index + i] = cleaned[i];
    }
    value = chars.join('').slice(0, SLOT_COUNT);
    focusSlot(Math.min(index + cleaned.length, SLOT_COUNT - 1));
  }

  function onFocus(index: number, e: FocusEvent) {
    (e.target as HTMLInputElement).select();
  }
</script>

<!-- Hidden field keeps the combined code in plain form submissions. -->
<input type="hidden" {name} bind:value />

<div
  class="code-tiles inline-flex gap-2 w-full justify-center"
  role="group"
  aria-label={m.room_code_input_aria()}
>
  {#each slots as char, i (i)}
    <input
      bind:this={tileEls[i]}
      type="text"
      inputmode="text"
      autocapitalize="characters"
      autocomplete="off"
      spellcheck="false"
      maxlength={1}
      value={char}
      oninput={(e) => onInput(i, e)}
      onkeydown={(e) => onKeydown(i, e)}
      onpaste={(e) => onPaste(i, e)}
      onfocus={(e) => onFocus(i, e)}
      aria-label="Character {i + 1} of {SLOT_COUNT}"
      class="code-tile"
    />
  {/each}
</div>

<style>
  /* Matches the RoomHeader code chip: flat tile, heavy border, stepped
     drop-shadow, subtle inner sheen. Sized for tap targets on mobile. */
  .code-tile {
    width: 3.25rem;
    height: 4rem;
    flex: 1 1 0;
    min-width: 0;
    max-width: 4.25rem;
    border-radius: 12px;
    border: 2.5px solid var(--brand-border-heavy);
    background: var(--brand-white);
    font-family: 'JetBrains Mono', ui-monospace, monospace;
    font-weight: 700;
    font-size: 1.75rem;
    text-align: center;
    text-transform: uppercase;
    color: var(--brand-text);
    box-shadow: 0 4px 0 rgba(0, 0, 0, 0.18), inset 0 1.5px 0 rgba(255, 255, 255, 0.8);
    outline: none;
    transition: border-color 0.15s ease, transform 0.08s ease, box-shadow 0.15s ease;
    caret-color: var(--brand-accent);
  }
  .code-tile:focus,
  .code-tile:focus-visible {
    border-color: var(--brand-accent);
    box-shadow: 0 4px 0 var(--brand-accent), inset 0 1.5px 0 rgba(255, 255, 255, 0.8);
  }
  .code-tile:not(:placeholder-shown) {
    background: var(--brand-white);
  }
  @media (max-width: 420px) {
    .code-tile {
      height: 3.5rem;
      font-size: 1.5rem;
    }
  }
</style>
