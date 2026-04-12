<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Home, Users, Mail, Package, Sliders, Bell, User } from '$lib/icons';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const navItems = [
    { href: '/admin', label: 'Dashboard', Icon: Home },
    { href: '/admin/users', label: 'Users', Icon: Users },
    { href: '/admin/invites', label: 'Invites', Icon: Mail },
    { href: '/admin/packs', label: 'Packs', Icon: Package },
    { href: '/admin/game-types', label: 'Game Types', Icon: Sliders },
  ] as const;
</script>

<div class="relative z-[2] min-h-screen flex text-brand-text">
  <!-- Sidebar -->
  <nav class="w-48 shrink-0 border-r border-brand-border bg-brand-white flex flex-col py-4">
    <div class="px-4 mb-6">
      <a href="/" class="font-bold text-base">FabDoYouMeme</a>
      <p class="text-xs text-brand-text-muted mt-0.5">Admin</p>
    </div>

    <ul class="flex flex-col gap-0.5 px-2">
      {#each navItems as item}
        <li>
          <a
            href={item.href}
            use:hoverEffect={'swap'}
            class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm text-brand-text-mid hover:text-brand-text transition-colors"
          >
            <item.Icon size={16} strokeWidth={2.5} />
            {item.label}
          </a>
        </li>
      {/each}
      <li>
        <a
          href="/admin/notifications"
          use:hoverEffect={'swap'}
          class="relative flex items-center gap-2 px-3 py-2 rounded-lg text-sm text-brand-text-mid hover:text-brand-text transition-colors"
        >
          <Bell size={16} strokeWidth={2.5} />
          Notifications
          {#if data.unreadNotifications > 0}
            <span class="absolute top-1 right-2 h-4 w-4 rounded-full bg-red-500 text-xs text-white flex items-center justify-center leading-none">
              {data.unreadNotifications > 9 ? '9+' : data.unreadNotifications}
            </span>
          {/if}
        </a>
      </li>
    </ul>

    <div class="mt-auto px-4 pt-4 border-t border-brand-border">
      <a
        href="/profile"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 text-xs text-brand-text-muted hover:text-brand-text transition-colors px-2 py-1 rounded-full"
      >
        <User size={12} strokeWidth={2.5} />
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
