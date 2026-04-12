<!-- frontend/src/lib/components/studio/TextEditor.svelte -->
<script lang="ts">
  import { untrack } from 'svelte';
  const MAX_CHARS = 500;
  let { initialValue = '', onSave }: { initialValue?: string; onSave: (text: string) => void } = $props();

  // Seed once from the prop via untrack — the editor owns `value` after
  // mount, so reactively tracking initialValue would clobber user edits.
  let value = $state(untrack(() => initialValue ?? ''));

  const remaining = $derived(MAX_CHARS - value.length);
</script>

<div class="flex flex-col gap-2">
  <div class="relative">
    <textarea
      bind:value
      rows={6}
      maxlength={MAX_CHARS}
      placeholder="Enter text content…"
      class="w-full rounded-lg border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
    ></textarea>
    <span class="absolute bottom-2 right-3 text-xs text-brand-text-muted">{remaining}</span>
  </div>

  <button
    type="button"
    onclick={() => onSave(value)}
    disabled={!value.trim()}
    class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50 hover:bg-primary/90 transition-colors"
  >
    Save as new version
  </button>
</div>
