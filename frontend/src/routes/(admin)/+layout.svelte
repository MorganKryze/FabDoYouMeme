<script lang="ts">
  import '../../app.css';
  import { page } from '$app/stores';
  import { env } from '$env/dynamic/public';
  import Toast from '$lib/components/Toast.svelte';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Home, Users, Mail, Package, Sliders, User, AlertTriangle } from '$lib/icons';
  import Wordmark from '$lib/components/Wordmark.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const navItems = [
    { href: '/admin', label: 'Dashboard', Icon: Home },
    { href: '/admin/users', label: 'Users', Icon: Users },
    { href: '/admin/invites', label: 'Invites', Icon: Mail },
    { href: '/admin/packs', label: 'Packs', Icon: Package },
    { href: '/admin/game-types', label: 'Game Types', Icon: Sliders },
  ] as const;

  // Danger zone visibility mirrors the backend's APP_ENV gate. Fails safe
  // to hidden when PUBLIC_APP_ENV is unset — the backend applies the same
  // default so an operator cannot accidentally expose the nav link via
  // partial env config.
  const dangerVisible = (env.PUBLIC_APP_ENV || 'prod') !== 'prod';

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
    <div class="px-2 mb-6">
      <Wordmark href="/" />
      <p class="text-xs font-bold uppercase tracking-[0.2em] text-brand-text-mid mt-3 px-2">
        Admin
      </p>
    </div>

    <ul class="flex flex-col gap-0.5 px-2">
      {#each navItems as item}
        {@const active = isActive(item.href, $page.url.pathname)}
        <li>
          <a
            href={item.href}
            use:hoverEffect={'swap'}
            aria-current={active ? 'page' : undefined}
            class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60
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

    {#if dangerVisible}
      {@const href = '/admin/danger'}
      {@const active = isActive(href, $page.url.pathname)}
      <div class="mx-2 mt-3 border-t border-brand-border pt-3">
        <a
          {href}
          use:hoverEffect={'swap'}
          aria-current={active ? 'page' : undefined}
          class="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-colors no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-red-300
            {active
              ? 'bg-red-600 text-white'
              : 'text-red-600 hover:text-red-700 hover:bg-red-50'}"
        >
          <AlertTriangle size={16} strokeWidth={2.5} />
          Danger zone
        </a>
      </div>
    {/if}

    <div class="mt-auto px-4 pt-4 border-t border-brand-border">
      <a
        href="/profile"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 text-xs text-brand-text-mid hover:text-brand-text transition-colors px-2 py-1 rounded-full focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
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
