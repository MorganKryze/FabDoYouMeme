<script lang="ts">
  import '../../app.css';
  import Toast from '$lib/components/Toast.svelte';
  import { ws } from '$lib/state/ws.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { user } from '$lib/state/user.svelte';
  import { page } from '$app/stores';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Wrench, HelpCircle } from '$lib/icons';
  import LabHelpDrawer from '$lib/components/studio/LabHelpDrawer.svelte';
  import type { LayoutData } from './$types';

  let { children, data }: { children: any; data: LayoutData } = $props();
  let showLabHelp = $state(false);

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
  <!-- Top bar: wordmark + Lab + avatar link -->
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
      <a
        href="/studio"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy no-underline"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
      >
        <Wrench size={16} strokeWidth={2.5} />
        Lab
      </a>

      {#if isLab}
        <button
          type="button"
          onclick={() => (showLabHelp = true)}
          use:hoverEffect={'swap'}
          aria-label="How packs work"
          class="inline-flex items-center justify-center h-10 w-10 rounded-full bg-brand-white border-[2.5px] border-brand-border-heavy"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          <HelpCircle size={16} strokeWidth={2.5} />
        </button>
      {/if}

      <a
        href="/profile"
        use:hoverEffect={'swap'}
        class="inline-flex items-center gap-2 px-4 py-2.5 rounded-full text-sm font-bold bg-brand-white border-[2.5px] border-brand-border-heavy no-underline"
        style="box-shadow: 0 3px 0 rgba(0,0,0,0.08);"
        aria-label="Profile"
      >
        <span
          class="h-5 w-5 rounded-full border-[2.5px] border-brand-border-heavy shrink-0"
          style="background: var(--brand-accent);"
          aria-hidden="true"
        ></span>
        <span>{data.user.username}</span>
      </a>
    </div>
  </header>

  <main class="flex-1 flex flex-col">
    {@render children()}
  </main>

  <Toast />
  <LabHelpDrawer bind:open={showLabHelp} />
  {/if}
</div>
