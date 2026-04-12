<script lang="ts">
  import { toast } from '$lib/state/toast.svelte';
  import { PartyPopper, Bell, XCircle } from '$lib/icons';

  const iconFor = {
    success: PartyPopper,
    warning: Bell,
    error: XCircle,
  } as const;
</script>

<div
  class="fixed bottom-4 right-4 z-50 flex flex-col-reverse gap-3 pointer-events-none"
  aria-live="polite"
  aria-atomic="false"
>
  {#each toast.items as item (item.id)}
    {@const Icon = iconFor[item.type as keyof typeof iconFor] ?? Bell}
    <div
      class="inline-flex items-center gap-2.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-brand-text pointer-events-auto w-fit"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.14);"
      role="alert"
    >
      <span class="inline-flex w-6 h-6 items-center justify-center shrink-0">
        <Icon size={18} strokeWidth={2.5} />
      </span>
      <span class="leading-snug">{item.message}</span>
      <button
        type="button"
        onclick={() => toast.dismiss(item.id)}
        class="shrink-0 ml-1 opacity-40 hover:opacity-100 transition-opacity inline-flex items-center"
        aria-label="Dismiss"
      >
        <XCircle size={16} strokeWidth={2.5} />
      </button>
    </div>
  {/each}
</div>
