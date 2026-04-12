<script lang="ts">
  import { theme, type ThemePref } from '$lib/state/theme.svelte';
  import { Sparkles, Sunrise, Sunset, Moon } from '$lib/icons';

  type Option = { value: ThemePref; label: string; Icon: typeof Sparkles };

  const options: Option[] = [
    { value: 'auto', label: 'Auto', Icon: Sparkles },
    { value: 'morning', label: 'Morning', Icon: Sunrise },
    { value: 'evening', label: 'Evening', Icon: Sunset },
    { value: 'night', label: 'Night', Icon: Moon },
  ];
</script>

<div class="theme-toggle" role="group" aria-label="Theme preference">
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
    background: #fefefe;
    border: 2.5px solid rgba(26, 26, 26, 0.7);
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
    color: rgba(26, 26, 26, 0.6);
    cursor: pointer;
    transition: background 0.2s ease, color 0.2s ease;
  }

  .segment:hover:not(.active) {
    color: rgba(26, 26, 26, 0.9);
  }

  .segment.active {
    background: #1a1a1a;
    color: #fefefe;
  }
</style>
