<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  $effect(() => {
    user.setFrom(data.user);
  });

  const statusDot: Record<string, string> = {
    connected: 'bg-green-500 opacity-0 group-hover:opacity-100',
    reconnecting: 'bg-amber-400 animate-pulse',
    error: 'bg-red-500',
    closed: 'bg-gray-400',
  };
</script>

<div class="min-h-screen flex flex-col bg-background text-foreground">
  <nav class="h-14 border-b border-border flex items-center px-4 gap-4">
    <a href="/" class="font-bold text-lg tracking-tight">FabDoYouMeme</a>
    <div class="flex-1"></div>

    <!-- Connection status indicator -->
    <div class="group relative flex items-center gap-1.5 cursor-default" title={ws.status}>
      <span class="h-2.5 w-2.5 rounded-full transition-all {statusDot[ws.status]}"></span>
      {#if ws.status === 'reconnecting'}
        <span class="text-xs text-amber-600 hidden sm:inline">Reconnecting…</span>
      {:else if ws.status === 'error'}
        <span class="text-xs text-red-600 hidden sm:inline">Connection lost</span>
        <button
          type="button"
          onclick={() => ws.reconnect()}
          class="text-xs underline text-red-600 hover:text-red-800"
        >
          Retry
        </button>
      {/if}
    </div>

    <a href="/profile" class="text-sm text-muted-foreground hover:text-foreground transition-colors">
      {data.user.username}
    </a>
    {#if data.user.role === 'admin'}
      <a href="/admin" class="text-sm text-muted-foreground hover:text-foreground transition-colors">
        Admin
      </a>
    {/if}
  </nav>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <Toast />
</div>
