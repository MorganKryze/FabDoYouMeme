<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { user } from '$lib/state/user.svelte';
  import { page } from '$app/stores';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Wrench, HelpCircle, Shield, Settings, X } from '$lib/icons';
  import { onMount } from 'svelte';
  import LabHelpDrawer from '$lib/components/studio/LabHelpDrawer.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();
  let showLabHelp = $state(false);

  // Floating Lab-help affordance: visible on /studio until the user
  // dismisses it. Persisted in localStorage so it doesn't keep nagging
  // across reloads — key is versioned in case copy changes later.
  const LAB_HELP_DISMISSED_KEY = 'fdym:labHelpDismissed:v1';
  let labHelpDismissed = $state(false);
  onMount(() => {
    try {
      labHelpDismissed = localStorage.getItem(LAB_HELP_DISMISSED_KEY) === '1';
    } catch {
      /* privacy mode / storage disabled — non-fatal, default to visible */
    }
  });
  function dismissLabHelp() {
    labHelpDismissed = true;
    try {
      localStorage.setItem(LAB_HELP_DISMISSED_KEY, '1');
    } catch {
      /* non-fatal */
    }
  }

  $effect(() => {
    if (data.user) user.setFrom(data.user);
  });

  const isLab = $derived($page.url.pathname.startsWith('/studio'));

  // WS status toasts — silent when healthy, informative on failure/recovery.
  // prevWsStatus starts null so the first effect run (initial mount) is
  // skipped without showing any toast.
  let prevWsStatus = $state<string | null>(null);

  $effect(() => {
    const status = ws.status;
    if (prevWsStatus === null) {
      prevWsStatus = status;
      return;
    }
    if (status === prevWsStatus) return;
    const prev = prevWsStatus;
    prevWsStatus = status;

    if (status === 'reconnecting') {
      toast.show('Connection lost — reconnecting…', 'warning');
    } else if (status === 'error') {
      toast.show('Connection failed.', 'error', { label: 'Retry', fn: () => ws.reconnect() });
    } else if (status === 'connected' && prev === 'reconnecting') {
      toast.show('Reconnected.', 'success');
    }
  });
</script>

<div class="relative z-[2] min-h-screen flex flex-col text-brand-text">
  {#if data.isGuest || !data.user}
    <!-- Guest room visit: minimal chrome — the room page's own header
         already shows the room code and connection status. Anonymous users
         no longer reach other (app) routes since /` lives in (marketing). -->
    <main class="flex-1 flex flex-col">
      {@render children()}
    </main>
    <Toast />
  {:else}
  <!-- Top bar: wordmark on the left; Admin (admins only) / Lab / Settings
       on the right. Admin sits immediately left of Lab so the two
       "operator" tools cluster together. -->
  <header class="flex items-center justify-between gap-4 px-6 pt-5 pb-4">
    <a
      href="/home"
      use:pressPhysics={'ghost'}
      class="text-lg font-bold tracking-tight no-underline"
      aria-label="FabDoYouMeme home"
    >
      FabDoYouMeme
    </a>

    <div class="flex items-center gap-3">
      {#if data.user.role === 'admin'}
        <a
          href="/admin"
          use:hoverEffect={'swap'}
          class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy no-underline"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
          aria-label="Admin panel"
        >
          <Shield size={16} strokeWidth={2.5} />
          Admin
        </a>
      {/if}

      <a
        href="/studio"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy no-underline"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <Wrench size={16} strokeWidth={2.5} />
        Lab
      </a>

      <a
        href="/profile"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy no-underline"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        aria-label="Settings"
      >
        <Settings size={16} strokeWidth={2.5} />
        Settings
      </a>
    </div>
  </header>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <!-- Floating Lab-help capsule — bottom-left, dismissable. The main
       pill opens the drawer; a small dark badge overlapping its
       top-right corner acts as a secondary "dismiss forever" control,
       matching the conventional dismissable-tooltip pattern so it
       never reads as a detached button. -->
  {#if isLab && !labHelpDismissed}
    <div class="fixed bottom-6 left-6 z-40">
      <div class="relative inline-block">
        <button
          type="button"
          onclick={() => (showLabHelp = true)}
          use:hoverEffect={'swap'}
          class="inline-flex items-center gap-2 h-11 px-5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy cursor-pointer"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.12);"
        >
          <HelpCircle size={16} strokeWidth={2.5} />
          How packs work
        </button>
        <button
          type="button"
          onclick={dismissLabHelp}
          aria-label="Dismiss help tip"
          class="absolute -top-2 -right-2 inline-flex items-center justify-center h-6 w-6 rounded-full bg-brand-text text-brand-white border-[2.5px] border-brand-border-heavy cursor-pointer hover:scale-110 transition-transform"
          style="box-shadow: 0 2px 0 rgba(0,0,0,0.18);"
        >
          <X size={10} strokeWidth={3.5} />
        </button>
      </div>
    </div>
  {/if}

  <Toast />
  <LabHelpDrawer bind:open={showLabHelp} />
  {/if}
</div>
