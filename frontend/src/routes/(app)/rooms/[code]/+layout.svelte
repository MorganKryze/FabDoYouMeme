<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { room } from '$lib/state/room.svelte';
  import { user } from '$lib/state/user.svelte';
  import EndRoomButton from '$lib/components/room/EndRoomButton.svelte';
  import type { WsMessage } from '$lib/api/types';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  let unsubscribe: (() => void) | null = null;

  onMount(() => {
    room.init(data.room);
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
  <!-- Slim status bar: only shown during playing/finished — the lobby owns
       the whole viewport and renders the room code in its hero card. -->
  {#if room.state !== 'lobby'}
    <div class="border-b border-brand-border px-4 py-2 flex items-center gap-4">
      <div class="flex items-center gap-2">
        <span class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Room</span>
        <span class="font-mono font-bold text-lg">{data.room.code}</span>
      </div>

      {#if room.gameType}
        <span class="text-sm font-semibold text-brand-text-mid">{room.gameType.name}</span>
      {/if}

      <div class="flex-1"></div>

      {#if ws.status === 'reconnecting'}
        <div class="text-xs font-bold animate-pulse" style="color: var(--brand-accent);">
          Reconnecting… (attempt {ws.retryCount} / 10)
        </div>
      {/if}

      {#if room.state === 'playing' && (user.id !== null && room.hostUserId === user.id)}
        <EndRoomButton compact />
      {/if}
    </div>
  {/if}

  <div class="flex-1 flex overflow-hidden">
    <!-- Main game area -->
    <div class="flex-1 overflow-y-auto">
      {@render children()}
    </div>

    <!-- Player panel: shown only during playing/finished phases.
         In the lobby, WaitingStage renders its own prominent player grid. -->
    {#if room.state !== 'lobby'}
      <aside class="w-52 shrink-0 border-l border-brand-border overflow-y-auto px-3 py-4">
        <h3 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted mb-3">
          Players ({room.players.length})
        </h3>
        <ul class="flex flex-col gap-2">
          {#each room.players as player, i}
            {@const colors = ['#E8937F', '#8BC9B1', '#D4A5C9', '#A8D8EA', '#FDDCB5', '#B5E2D0']}
            {@const hasSubmitted = room.submittedPlayerIds.has(player.user_id)}
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
              <span class="truncate text-sm font-bold flex-1 text-left">
                {player.username}
                {#if player.is_guest}
                  <span class="text-[0.6rem] font-bold text-brand-text-muted uppercase tracking-[0.1em] ml-1">guest</span>
                {/if}
              </span>
              {#if room.phase === 'submitting'}
                <span
                  class="text-base leading-none shrink-0"
                  title={hasSubmitted ? 'Submitted' : 'Still writing…'}
                  aria-label={hasSubmitted ? 'Submitted' : 'Still writing'}
                >
                  {hasSubmitted ? '\u2713' : '\u23F3'}
                </span>
              {/if}
              {#if player.is_host}
                <span class="text-[0.7rem] font-semibold text-brand-text-muted shrink-0">Host</span>
              {/if}
              {#if !player.connected}
                <span class="h-2 w-2 rounded-full bg-gray-300 shrink-0" title="Disconnected"></span>
              {/if}
            </li>
          {/each}
        </ul>
      </aside>
    {/if}
  </div>
</div>
