<script lang="ts">
  import { theme, type ThemePref } from '$lib/state/theme.svelte';
  import { Sparkles, Sun, Moon } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  type Option = { value: ThemePref; label: string; Icon: typeof Sparkles };

  const options: Option[] = $derived([
    { value: 'auto', label: m.common_theme_auto(), Icon: Sparkles },
    { value: 'light', label: m.common_theme_light(), Icon: Sun },
    { value: 'dark', label: m.common_theme_dark(), Icon: Moon },
  ]);
</script>

<div class="theme-toggle" role="group" aria-label={m.common_theme_aria()}>
  {#each options as { value, label, Icon } (value)}
    <button
      type="button"
      class="segment"
      class:active={theme.preference === value}
      aria-pressed={theme.preference === value}
      onclick={() => theme.setPreference(value)}
    >
      <Icon size={16} strokeWidth={2.5} />
      <span>{label}</span>
    </button>
  {/each}
</div>

<style>
  .theme-toggle {
    display: inline-flex;
    align-self: flex-start;
    background: var(--brand-white);
    border: 2.5px solid var(--brand-border-heavy);
    border-radius: 999px;
    padding: 4px;
    gap: 2px;
    box-shadow: 0 5px 0 rgba(0, 0, 0, 0.12);
  }

  .segment {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 14px;
    border: none;
    background: transparent;
    border-radius: 999px;
    font-family: inherit;
    font-weight: 700;
    font-size: 0.8rem;
    color: var(--brand-text-muted);
    cursor: pointer;
    transition: background 0.2s ease, color 0.2s ease;
  }

  .segment:hover:not(.active) {
    color: var(--brand-text-mid);
  }

  .segment.active {
    background: var(--brand-text);
    color: var(--brand-white);
  }
</style>
