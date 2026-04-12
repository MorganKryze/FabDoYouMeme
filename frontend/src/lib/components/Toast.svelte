<script lang="ts">
  import { toast } from '$lib/state/toast.svelte';

  const emojiMap: Record<string, string> = {
    success: '\u{1F389}',
    warning: '\u{1F514}',
    error: '\u{274C}',
  };
</script>

<div
  class="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-3 pointer-events-none"
  aria-live="polite"
  aria-atomic="false"
>
  {#each toast.items as item (item.id)}
    <div
      class="inline-flex items-center gap-2.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-brand-text pointer-events-auto w-fit"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.14);"
      role="alert"
    >
      <span>{emojiMap[item.type] ?? ''}</span>
      <span class="leading-snug">{item.message}</span>
      <button
        type="button"
        onclick={() => toast.dismiss(item.id)}
        class="shrink-0 ml-1 opacity-40 hover:opacity-100 transition-opacity text-lg leading-none"
        aria-label="Dismiss"
      >
        &times;
      </button>
    </div>
  {/each}
</div>
