<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Copy, CheckCircle } from '$lib/icons';
  import type { WsMessage } from '$lib/api/types';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  let copied = $state(false);
  let copyTimeout: ReturnType<typeof setTimeout> | null = null;

  async function copyRoomCode() {
    await navigator.clipboard.writeText(data.room.code);
    copied = true;
    if (copyTimeout) clearTimeout(copyTimeout);
    copyTimeout = setTimeout(() => { copied = false; }, 2000);
  }

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
  <div class="border-b border-brand-border px-4 py-2 flex items-center gap-4">
    <div class="flex items-center gap-2">
      <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Room</span>
      <span class="font-mono font-bold text-lg">{data.room.code}</span>
      <button
        type="button"
        onclick={copyRoomCode}
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 text-xs font-bold text-brand-text-muted hover:text-brand-text transition-colors px-2 py-1 rounded-full"
        title="Copy room code"
      >
        {#if copied}
          <CheckCircle size={14} strokeWidth={2.5} />
          Copied!
        {:else}
          <Copy size={14} strokeWidth={2.5} />
          Copy
        {/if}
      </button>
    </div>

    {#if room.gameType}
      <span class="text-sm font-semibold text-brand-text-mid">{room.gameType.name}</span>
    {/if}

    <div class="flex-1"></div>

    <!-- Reconnecting banner -->
    {#if ws.status === 'reconnecting'}
      <div class="text-xs font-bold animate-pulse" style="color: var(--brand-accent);">
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
    <aside class="w-52 shrink-0 border-l border-brand-border overflow-y-auto px-3 py-4">
      <h3 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-3">
        Players ({room.players.length})
      </h3>
      <ul class="flex flex-col gap-2">
        {#each room.players as player, i}
          {@const colors = ['#E8937F', '#8BC9B1', '#D4A5C9', '#A8D8EA', '#FDDCB5', '#B5E2D0']}
          <li
            class="flex items-center gap-3 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-3 py-2 transition-transform duration-300"
            style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
          >
            <span
              class="h-9 w-9 shrink-0 rounded-full border-[2.5px] border-brand-border-heavy flex items-center justify-center text-xs font-bold text-white"
              style="background: {colors[i % colors.length]};"
            >
              {player.username.slice(0, 2).toUpperCase()}
            </span>
            <span class="truncate text-sm font-bold">{player.username}</span>
            {#if player.is_host}
              <span class="text-[0.7rem] font-semibold text-brand-text-muted ml-auto">Host</span>
            {/if}
            {#if !player.connected}
              <span class="h-2 w-2 rounded-full bg-gray-300 ml-auto"></span>
            {/if}
          </li>
        {/each}
      </ul>
    </aside>
  </div>
</div>
