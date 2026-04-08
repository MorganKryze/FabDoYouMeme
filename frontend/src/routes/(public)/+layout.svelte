<!-- frontend/src/routes/(public)/+layout.svelte -->
<script lang="ts">
  import '../../app.css';
  let { children } = $props();
</script>

<div class="min-h-screen flex flex-col items-center justify-center bg-background text-foreground px-4">
  <main class="w-full max-w-sm flex flex-col gap-6">
    {@render children()}
  </main>
  <footer class="mt-8 text-xs text-muted-foreground">
    <a href="/privacy" class="underline hover:text-foreground">Privacy Policy</a>
  </footer>
</div>
