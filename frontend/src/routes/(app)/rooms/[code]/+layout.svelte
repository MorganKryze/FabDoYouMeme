<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import { guest } from '$lib/state/guest.svelte';
  import type { WsMessage } from '$lib/api/types';
  import type { LayoutData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { children, data }: { children: any; data: LayoutData } = $props();

  let unsubscribe: (() => void) | null = null;

  onMount(() => {
    // Player identity: logged-in users expose `user.id`; guests resolve via
    // the per-tab sessionStorage record keyed by room code. room.svelte.ts
    // uses this to match WS broadcasts against "is this me?".
    const ownUserId = user.id ?? guest.playerId(data.room.code);
    room.init({ ...data.room, own_user_id: ownUserId });
    ws.connect(data.room.code);
    unsubscribe = ws.onMessage('*', (msg) => room.handleMessage(msg as WsMessage));
  });

  onDestroy(() => {
    unsubscribe?.();
    unsubscribe = null;
    ws.disconnect();
    room.reset();
  });
</script>

<div class="flex-1 flex flex-col">
  {@render children()}
</div>

<!-- Floating reconnect indicator — appears only while WS is cycling. -->
{#if ws.status === 'reconnecting'}
  <div
    class="fixed bottom-5 right-5 z-40 inline-flex items-center gap-2 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-2 text-xs font-bold animate-pulse"
    style="color: var(--brand-accent); box-shadow: 0 4px 0 rgba(0,0,0,0.12);"
    role="status"
    aria-live="polite"
  >
    <span class="h-2 w-2 rounded-full" style="background: var(--brand-accent);"></span>
    {m.room_reconnect_hint({ attempt: ws.retryCount })}
  </div>
{/if}
