<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { authApi } from '$lib/api/auth';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import ThemeToggle from './ThemeToggle.svelte';
  import { User, Shield, LogOut, ChevronRight } from '$lib/icons';

  interface Props {
    username: string;
    role: 'player' | 'admin';
  }

  let { username, role }: Props = $props();

  let open = $state(false);
  let rootEl = $state<HTMLDivElement | null>(null);

  $effect(() => {
    if (!open) return;
    function handlePointer(e: PointerEvent) {
      if (rootEl && !rootEl.contains(e.target as Node)) open = false;
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') open = false;
    }
    document.addEventListener('pointerdown', handlePointer);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('pointerdown', handlePointer);
      document.removeEventListener('keydown', handleKey);
    };
  });

  async function logout() {
    try {
      await authApi.logout();
    } catch {
      /* non-fatal — server-side session still cleared on next request */
    }
    await invalidateAll();
    await goto('/auth/magic-link');
  }
</script>

<div bind:this={rootEl} class="relative inline-block">
  <button
    type="button"
    use:pressPhysics={'ghost'}
    use:hoverEffect={'swap'}
    onclick={() => (open = !open)}
    aria-haspopup="menu"
    aria-expanded={open}
    class="inline-flex items-center gap-2 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy cursor-pointer"
    style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
  >
    <span
      class="h-5 w-5 rounded-full border-[2.5px] border-brand-border-heavy"
      style="background: var(--brand-accent);"
      aria-hidden="true"
    ></span>
    <span>{username}</span>
  </button>

  {#if open}
    <div
      role="menu"
      class="absolute right-0 top-full mt-3 min-w-[240px] rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-3 flex flex-col gap-2 z-50"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.10);"
    >
      <a
        href="/profile"
        role="menuitem"
        onclick={() => (open = false)}
        class="group inline-flex items-center gap-3 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy hover:bg-brand-surface transition-colors"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <User size={16} strokeWidth={2.5} />
        <span>Profile</span>
        <ChevronRight size={14} strokeWidth={2.5} class="ml-auto opacity-50 group-hover:opacity-100" />
      </a>

      {#if role === 'admin'}
        <a
          href="/admin"
          role="menuitem"
          onclick={() => (open = false)}
          class="group inline-flex items-center gap-3 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy hover:bg-brand-surface transition-colors"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          <Shield size={16} strokeWidth={2.5} />
          <span>Admin</span>
          <ChevronRight size={14} strokeWidth={2.5} class="ml-auto opacity-50 group-hover:opacity-100" />
        </a>
      {/if}

      <div class="flex justify-center py-1">
        <ThemeToggle />
      </div>

      <button
        type="button"
        role="menuitem"
        onclick={logout}
        class="inline-flex items-center gap-3 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-text text-brand-white border-[2.5px] border-brand-border-heavy cursor-pointer hover:opacity-90 transition-opacity"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.12);"
      >
        <LogOut size={16} strokeWidth={2.5} />
        <span>Log out</span>
      </button>
    </div>
  {/if}
</div>
