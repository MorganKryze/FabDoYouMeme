<!-- frontend/src/routes/(public)/+layout.svelte -->
<script lang="ts">
  import '../../app.css';
  import Wordmark from '$lib/components/Wordmark.svelte';
  import * as m from '$lib/paraglide/messages';
  let { children } = $props();
</script>

<div class="relative z-[2] min-h-dvh flex flex-col text-brand-text px-4">
  <!-- Persistent wordmark → back to the marketing landing. Shared across
       all public pages (magic-link, register, verify, /join, /join/[code])
       so every anonymous visitor has a visible exit. /privacy resets
       layout inheritance via its own +layout@.svelte and renders without
       the auth-card chrome. -->
  <header class="flex items-center py-5">
    <Wordmark href="/" />
  </header>

  <div class="flex-1 flex flex-col items-center justify-center">
    <main
      class="w-full max-w-sm flex flex-col gap-6 rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      {@render children()}
    </main>
    <footer class="mt-8 text-xs text-brand-text-mid">
      <a
        href="/privacy"
        class="underline hover:text-brand-text transition-colors rounded-sm focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-brand-accent/60"
      >
        {m.common_privacy_policy()}
      </a>
    </footer>
  </div>
</div>
