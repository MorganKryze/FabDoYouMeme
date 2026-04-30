<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Home, Search, AlertTriangle, Lock } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  // Route-scoped boundary for /rooms/{code}. The layout loader throws when
  // the backend can't load the room — most often 404 ("no such room"), which
  // we want to handle differently from the global +error.svelte: a brief
  // explainer that auto-redirects home rather than waiting on a click.
  // 401/403/500 still surface here, so we fall back to a styled message.

  type Preset = { Icon: typeof AlertTriangle; title: string; body: string };

  const REDIRECT_MS = 2000;
  const status = $derived($page.status);
  const errMessage = $derived($page.error?.message ?? '');
  const signedIn = $derived(Boolean(($page.data as { user?: unknown } | undefined)?.user));
  const target = $derived(signedIn ? '/home' : '/');
  const isRoomMissing = $derived(status === 404);

  const preset = $derived<Preset>(
    isRoomMissing
      ? {
          Icon: Search,
          title: m.error_room_not_found_title(),
          body: m.error_room_not_found_body()
        }
      : status === 401
        ? { Icon: Lock, title: m.error_401_title(), body: m.error_401_body() }
        : status === 403
          ? { Icon: Lock, title: m.error_403_title(), body: m.error_403_body() }
          : status === 500
            ? { Icon: AlertTriangle, title: m.error_500_title(), body: m.error_500_body() }
            : { Icon: AlertTriangle, title: m.error_fallback_title(), body: m.error_fallback_body() }
  );
  const Icon = $derived(preset.Icon);

  let timer: ReturnType<typeof setTimeout> | null = null;

  onMount(() => {
    if (!isRoomMissing) return;
    timer = setTimeout(() => {
      // replaceState so the dead room URL doesn't sit in history — the back
      // button shouldn't bounce the user straight back into another redirect.
      void goto(target, { replaceState: true });
    }, REDIRECT_MS);
  });

  onDestroy(() => {
    if (timer) clearTimeout(timer);
  });
</script>

<svelte:head>
  <title>{status} — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex flex-col items-center justify-center px-4 py-16 min-h-[80vh]">
  <main
    class="w-full max-w-lg flex flex-col items-center gap-6 rounded-[28px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 sm:p-10 text-center"
    style="box-shadow: 0 6px 0 rgba(0,0,0,0.08);"
    role="status"
    aria-live="polite"
  >
    <div
      class="inline-flex h-16 w-16 items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
    >
      <Icon size={28} strokeWidth={2.5} />
    </div>

    <div class="flex flex-col items-center gap-2">
      <div class="text-[0.7rem] font-bold uppercase tracking-[0.2em] text-brand-text-mid">
        {m.error_status_prefix({ status })}
      </div>
      <h1 class="text-3xl font-bold">{preset.title}</h1>
      <p class="text-sm font-semibold text-brand-text-mid max-w-xs">
        {preset.body}
      </p>
      {#if !isRoomMissing && errMessage && errMessage.toLowerCase() !== preset.title.toLowerCase()}
        <p
          class="text-xs font-mono text-brand-text-mid mt-2 px-3 py-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white max-w-full truncate"
          title={errMessage}
        >
          {errMessage}
        </p>
      {/if}
    </div>

    {#if isRoomMissing}
      <div
        class="h-1.5 w-full max-w-[12rem] overflow-hidden rounded-full border-[2px] border-brand-border-heavy bg-brand-white"
      >
        <div
          class="h-full bg-brand-text"
          style="animation: room-redirect-bar {REDIRECT_MS}ms linear forwards;"
        ></div>
      </div>
    {:else}
      <a
        href={target}
        use:pressPhysics={'dark'}
        use:hoverEffect={'gradient'}
        class="whitespace-nowrap inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
      >
        <Home size={16} strokeWidth={2.5} />
        {signedIn ? m.error_back_to_dashboard() : m.error_back_to_home()}
      </a>
    {/if}
  </main>
</div>

<style>
  @keyframes room-redirect-bar {
    from {
      width: 0%;
    }
    to {
      width: 100%;
    }
  }
</style>
