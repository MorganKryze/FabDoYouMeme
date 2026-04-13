<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import AvatarMenu from '$lib/components/AvatarMenu.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import { page } from '$app/stores';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Wrench } from '$lib/icons';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  $effect(() => {
    if (data.user) user.setFrom(data.user);
  });

  const statusDot: Record<string, string> = {
    connected: 'bg-green-500 opacity-0 group-hover:opacity-100',
    reconnecting: 'bg-brand-accent animate-pulse',
    error: 'bg-red-500',
    closed: 'bg-gray-400',
  };

  const isLab = $derived($page.url.pathname.startsWith('/studio'));
</script>

<div class="relative z-[2] min-h-screen flex flex-col text-brand-text">
  {#if data.isGuest || !data.user}
    <!-- Guest room visit: minimal chrome — the room page's own header
         already shows the room code and connection status. -->
    <main class="flex-1 flex flex-col">
      {@render children()}
    </main>
    <Toast />
  {:else}
  <!-- Top bar: wordmark + Lab + status + avatar -->
  <header class="flex items-center justify-between gap-4 px-6 pt-5 pb-4">
    <a
      href="/"
      use:pressPhysics={'ghost'}
      class="text-lg font-bold tracking-tight no-underline"
      aria-label="FabDoYouMeme home"
    >
      FabDoYouMeme
    </a>

    <div class="flex items-center gap-3">
      <a
        href="/studio"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy transition-colors
          {isLab ? '' : 'opacity-60 hover:opacity-100'}"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <Wrench size={16} strokeWidth={2.5} />
        Lab
      </a>

      <!-- Connection status dot -->
      <div
        class="group relative inline-flex items-center gap-2 px-3 h-[42px] rounded-full bg-brand-white border-[2.5px] border-brand-border-heavy cursor-default"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        title={ws.status}
      >
        <span class="h-2.5 w-2.5 rounded-full transition-all {statusDot[ws.status]}"></span>
        {#if ws.status === 'reconnecting'}
          <span class="text-xs hidden sm:inline font-bold" style="color: var(--brand-accent);">Reconnecting…</span>
        {:else if ws.status === 'error'}
          <button
            type="button"
            onclick={() => ws.reconnect()}
            class="text-xs underline text-red-600 hover:text-red-800 font-bold"
          >
            Retry
          </button>
        {/if}
      </div>

      <AvatarMenu username={data.user.username} role={data.user.role} />
    </div>
  </header>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <Toast />
  {/if}
</div>
