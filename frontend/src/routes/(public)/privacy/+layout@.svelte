<!-- frontend/src/routes/(public)/privacy/+layout@.svelte -->
<!-- The @ resets layout inheritance so this page is NOT wrapped by (public)/+layout.svelte -->
<script lang="ts">
  import '../../../app.css';
  let { children } = $props();
</script>

<div class="min-h-screen bg-background text-foreground px-4 py-8">
  {@render children()}
</div>
