<script lang="ts">
  import { toast } from '$lib/state/toast.svelte';
  import { PartyPopper, Bell, XCircle } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  const iconFor = {
    success: PartyPopper,
    warning: Bell,
    error: XCircle,
  } as const;
</script>

<!--
  The `toast-root` id promotes this container into its own named
  view-transition group (`toast-layer` — see rule in app.css). That group
  is forced to paint above ::view-transition-group(page-main) so the
  incoming page's snapshot cannot briefly cover a visible toast during
  navigation.
-->
<div
  id="toast-root"
  class="fixed bottom-4 right-4 z-[100] flex flex-col-reverse gap-3 pointer-events-none"
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
      {#if item.action}
        <button
          type="button"
          onclick={() => { item.action!.fn(); toast.dismiss(item.id); }}
          class="shrink-0 underline text-sm font-bold hover:opacity-70 transition-opacity cursor-pointer"
        >
          {item.action.label}
        </button>
      {/if}
      <button
        type="button"
        onclick={() => toast.dismiss(item.id)}
        class="shrink-0 ml-1 opacity-40 hover:opacity-100 transition-opacity inline-flex items-center cursor-pointer"
        aria-label={m.common_dismiss()}
      >
        <XCircle size={16} strokeWidth={2.5} />
      </button>
    </div>
  {/each}
</div>
