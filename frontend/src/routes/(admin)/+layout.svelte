<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const navItems = [
    { href: '/admin', label: 'Dashboard' },
    { href: '/admin/users', label: 'Users' },
    { href: '/admin/invites', label: 'Invites' },
    { href: '/admin/packs', label: 'Packs' },
    { href: '/admin/game-types', label: 'Game Types' },
  ] as const;
</script>

<div class="min-h-screen flex bg-background text-foreground">
  <!-- Sidebar -->
  <nav class="w-48 shrink-0 border-r border-border flex flex-col py-4">
    <div class="px-4 mb-6">
      <a href="/" class="font-bold text-base">FabDoYouMeme</a>
      <p class="text-xs text-muted-foreground mt-0.5">Admin</p>
    </div>

    <ul class="flex flex-col gap-0.5 px-2">
      {#each navItems as item}
        <li>
          <a
            href={item.href}
            class="flex items-center gap-2 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            {item.label}
          </a>
        </li>
      {/each}
      <li>
        <a
          href="/admin/notifications"
          class="relative flex items-center gap-2 px-3 py-2 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        >
          Notifications
          {#if data.unreadNotifications > 0}
            <span class="absolute top-1 right-2 h-4 w-4 rounded-full bg-red-500 text-xs text-white flex items-center justify-center leading-none">
              {data.unreadNotifications > 9 ? '9+' : data.unreadNotifications}
            </span>
          {/if}
        </a>
      </li>
    </ul>

    <div class="mt-auto px-4 pt-4 border-t border-border">
      <a href="/profile" class="text-xs text-muted-foreground hover:text-foreground transition-colors">
        {data.user.username}
      </a>
    </div>
  </nav>

  <!-- Main content -->
  <main class="flex-1 overflow-y-auto">
    {@render children()}
  </main>

  <Toast />
</div>
