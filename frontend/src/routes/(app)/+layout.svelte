<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { user } from '$lib/state/user.svelte';
  import { page } from '$app/stores';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  $effect(() => {
    user.setFrom(data.user);
  });

  const statusDot: Record<string, string> = {
    connected: 'bg-green-500 opacity-0 group-hover:opacity-100',
    reconnecting: 'bg-brand-accent animate-pulse',
    error: 'bg-red-500',
    closed: 'bg-gray-400',
  };

  const navLinks = [
    { href: '/', label: 'Play' },
    { href: '/studio', label: 'Lab' },
  ] as const;
</script>

<div class="relative z-[2] min-h-screen flex flex-col text-brand-text">
  <!-- Pill Nav -->
  <div class="flex justify-center pt-5 pb-4 px-4">
    <nav
      class="flex items-center gap-1 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-1.5 py-1.5"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.12);"
    >
      {#each navLinks as link}
        <a
          href={link.href}
          class="px-5 py-2.5 rounded-full text-sm font-bold transition-colors
            {$page.url.pathname === link.href
              ? 'underline underline-offset-4 decoration-[2.5px]'
              : 'opacity-50 hover:opacity-100'}"
        >
          {link.label}
        </a>
      {/each}

      <!-- Connection status -->
      <div class="group relative flex items-center gap-1.5 px-3 cursor-default" title={ws.status}>
        <span class="h-2.5 w-2.5 rounded-full transition-all {statusDot[ws.status]}"></span>
        {#if ws.status === 'reconnecting'}
          <span class="text-xs hidden sm:inline" style="color: var(--brand-accent);">Reconnecting…</span>
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

      <a
        href="/profile"
        class="px-4 py-2.5 rounded-full text-sm font-bold opacity-50 hover:opacity-100 transition-colors"
      >
        {data.user.username}
      </a>

      {#if data.user.role === 'admin'}
        <a
          href="/admin"
          class="px-4 py-2.5 rounded-full text-sm font-bold opacity-50 hover:opacity-100 transition-colors"
        >
          Admin
        </a>
      {/if}
    </nav>
  </div>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <Toast />
</div>
