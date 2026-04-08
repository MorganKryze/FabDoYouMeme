<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import type { WsMessage } from '$lib/api/types';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  onMount(() => {
    room.init(data.room);
    ws.connect(data.room.code);
    ws.onMessage('*', (msg) => room.handleMessage(msg as WsMessage));
  });

  onDestroy(() => {
    ws.disconnect();
    room.reset();
  });
</script>

<div class="flex-1 flex flex-col">
  <!-- Room header bar -->
  <div class="border-b border-border px-4 py-2 flex items-center gap-4">
    <div class="flex items-center gap-2">
      <span class="text-xs text-muted-foreground uppercase tracking-wider">Room</span>
      <span class="font-mono font-bold text-lg">{data.room.code}</span>
      <button
        type="button"
        onclick={() => navigator.clipboard.writeText(data.room.code)}
        class="text-xs text-muted-foreground hover:text-foreground transition-colors"
        title="Copy room code"
      >
        Copy
      </button>
    </div>

    {#if room.gameType}
      <span class="text-sm text-muted-foreground">{room.gameType.name}</span>
    {/if}

    <div class="flex-1"></div>

    <!-- Reconnecting banner -->
    {#if ws.status === 'reconnecting'}
      <div class="text-xs text-amber-600 animate-pulse">
        Reconnecting… (attempt {ws.retryCount} / 10)
      </div>
    {/if}
  </div>

  <div class="flex-1 flex overflow-hidden">
    <!-- Main game area -->
    <div class="flex-1 overflow-y-auto">
      {@render children()}
    </div>

    <!-- Player panel -->
    <aside class="w-48 shrink-0 border-l border-border overflow-y-auto px-3 py-4">
      <h3 class="text-xs font-semibold uppercase text-muted-foreground tracking-wider mb-3">
        Players ({room.players.length})
      </h3>
      <ul class="flex flex-col gap-1.5">
        {#each room.players as player}
          <li class="flex items-center gap-2 text-sm">
            <span class="h-2 w-2 rounded-full {player.connected ? 'bg-green-500' : 'bg-gray-300'}"></span>
            <span class="truncate">{player.username}</span>
            {#if player.is_host}
              <span class="text-xs text-muted-foreground ml-auto">(host)</span>
            {/if}
          </li>
        {/each}
      </ul>
    </aside>
  </div>
</div>
