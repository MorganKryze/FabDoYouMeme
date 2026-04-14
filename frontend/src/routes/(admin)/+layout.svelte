<script lang="ts">
  import '../../app.css';
  import { page } from '$app/stores';
  import Toast from '$lib/components/Toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Home, Users, Mail, Package, Sliders, User } from '$lib/icons';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const navItems = [
    { href: '/admin', label: 'Dashboard', Icon: Home },
    { href: '/admin/users', label: 'Users', Icon: Users },
    { href: '/admin/invites', label: 'Invites', Icon: Mail },
    { href: '/admin/packs', label: 'Packs', Icon: Package },
    { href: '/admin/game-types', label: 'Game Types', Icon: Sliders },
  ] as const;

  // Dashboard uses exact match because every other admin path also starts
  // with `/admin`; all other items use "exact OR prefix+boundary" so nested
  // routes like /admin/packs/[id] still highlight "Packs".
  function isActive(href: string, pathname: string): boolean {
    if (href === '/admin') return pathname === '/admin';
    return pathname === href || pathname.startsWith(href + '/');
  }
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
        {@const active = isActive(item.href, $page.url.pathname)}
        <li>
          <a
            href={item.href}
            use:hoverEffect={'swap'}
            aria-current={active ? 'page' : undefined}
            class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors
              {active
                ? 'bg-brand-text text-brand-white'
                : 'text-brand-text-mid hover:text-brand-text hover:bg-muted'}"
          >
            <item.Icon size={16} strokeWidth={2.5} />
            {item.label}
          </a>
        </li>
      {/each}
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
