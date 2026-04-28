<!-- frontend/src/routes/(public)/privacy/+layout@.svelte -->
<!-- The @ resets layout inheritance so this page is NOT wrapped by
     (public)/+layout.svelte's narrow auth-card chrome — privacy is a
     long-form, full-width legal document and needs the full viewport. -->
<script lang="ts">
  import '../../../app.css';
  let { children } = $props();
</script>

<div class="relative z-[2] min-h-dvh text-brand-text px-4 py-8">
  {@render children()}
</div>
