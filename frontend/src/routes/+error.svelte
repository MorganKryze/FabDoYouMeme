<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Home, Hash, Search, AlertTriangle, Lock } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';

  // Root-level error boundary. SvelteKit renders this whenever a load
  // function throws and no nearer +error.svelte exists along the route
  // path. It composes inside whichever parent layout(s) finished loading
  // successfully — so auth'd users still see the (app) header chrome,
  // guests see the bare TimeBackground from the root layout, and pure
  // "route doesn't exist" 404s also land here.

  type Preset = { Icon: typeof AlertTriangle; title: string; body: string };

  const presets: Record<number, Preset> = {
    404: {
      Icon: Search,
      title: m.error_404_title(),
      body: m.error_404_body()
    },
    401: {
      Icon: Lock,
      title: m.error_401_title(),
      body: m.error_401_body()
    },
    403: {
      Icon: Lock,
      title: m.error_403_title(),
      body: m.error_403_body()
    },
    500: {
      Icon: AlertTriangle,
      title: m.error_500_title(),
      body: m.error_500_body()
    }
  };

  const fallback: Preset = {
    Icon: AlertTriangle,
    title: m.error_fallback_title(),
    body: m.error_fallback_body()
  };

  const status = $derived($page.status);
  const errMessage = $derived($page.error?.message ?? '');
  // $page.data merges every parent layout loader that ran before the
  // error. For errors thrown inside (app)/(admin)/(marketing), `user`
  // is populated; for "route doesn't exist" 404s no group matches, so
  // data is empty and we fall back to sending the visitor to landing.
  const signedIn = $derived(Boolean(($page.data as { user?: unknown } | undefined)?.user));

  const preset = $derived(presets[status] ?? fallback);
  const Icon = $derived(preset.Icon);

  const primaryHref = $derived(signedIn ? '/home' : '/');
  const primaryLabel = $derived(signedIn ? m.error_back_to_dashboard() : m.error_back_to_home());
</script>

<svelte:head>
  <title>{status} — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex flex-col items-center justify-center px-4 py-16 min-h-[80vh]">
  <main
    class="w-full max-w-lg flex flex-col items-center gap-6 rounded-[28px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-8 sm:p-10 text-center"
    style="box-shadow: 0 6px 0 rgba(0,0,0,0.08);"
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
      {#if errMessage && errMessage.toLowerCase() !== preset.title.toLowerCase()}
        <p
          class="text-xs font-mono text-brand-text-mid mt-2 px-3 py-1.5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white max-w-full truncate"
          title={errMessage}
        >
          {errMessage}
        </p>
      {/if}
    </div>

    <div class="flex flex-col sm:flex-row items-stretch gap-3 w-full pt-2">
      <a
        href={primaryHref}
        use:pressPhysics={'dark'}
        use:hoverEffect={'gradient'}
        class="flex-1 whitespace-nowrap inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
      >
        <Home size={16} strokeWidth={2.5} />
        {primaryLabel}
      </a>
      {#if status === 404}
        <a
          href="/join"
          use:pressPhysics={'ghost'}
          use:hoverEffect={'swap'}
          class="flex-1 whitespace-nowrap inline-flex items-center justify-center gap-2 px-6 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white font-bold no-underline focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
        >
          <Hash size={16} strokeWidth={2.5} />
          {m.error_join_with_code()}
        </a>
      {/if}
    </div>
  </main>
</div>
