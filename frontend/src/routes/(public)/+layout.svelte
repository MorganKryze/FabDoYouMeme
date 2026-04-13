<!-- frontend/src/routes/(public)/+layout.svelte -->
<script lang="ts">
  import '../../app.css';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  let { children } = $props();
</script>

<div class="relative z-[2] min-h-screen flex flex-col text-brand-text px-4">
  <!-- Persistent wordmark → back to the marketing landing. Shared across
       all public pages (magic-link, register, verify, /join, /join/[code])
       so every anonymous visitor has a visible exit. -->
  <header class="flex items-center py-5">
    <a
      href="/"
      use:pressPhysics={'ghost'}
      class="text-lg font-bold tracking-tight no-underline"
      aria-label="FabDoYouMeme home"
    >
      FabDoYouMeme
    </a>
  </header>

  <div class="flex-1 flex flex-col items-center justify-center">
    <main
      class="w-full max-w-sm flex flex-col gap-6 rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-6"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      {@render children()}
    </main>
    <footer class="mt-8 text-xs text-brand-text-muted">
      <a href="/privacy" class="underline hover:text-brand-text transition-colors">Privacy Policy</a>
    </footer>
  </div>
</div>
