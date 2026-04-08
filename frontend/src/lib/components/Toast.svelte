<script lang="ts">
  import { toast } from '$lib/state/toast.svelte';

  const typeClasses: Record<string, string> = {
    success: 'bg-green-600 text-white',
    warning: 'bg-yellow-500 text-white',
    error: 'bg-red-600 text-white',
  };
</script>

<div
  class="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-2 pointer-events-none"
  aria-live="polite"
  aria-atomic="false"
>
  {#each toast.items as item (item.id)}
    <div
      class="flex items-start gap-3 rounded-lg px-4 py-3 shadow-lg text-sm max-w-sm pointer-events-auto {typeClasses[item.type]}"
      role="alert"
    >
      <span class="flex-1 leading-snug">{item.message}</span>
      <button
        type="button"
        onclick={() => toast.dismiss(item.id)}
        class="shrink-0 text-current opacity-70 hover:opacity-100 transition-opacity text-lg leading-none"
        aria-label="Dismiss"
      >
        ×
      </button>
    </div>
  {/each}
</div>
