<!-- frontend/src/routes/(public)/auth/magic-link/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let sent = $derived(form?.sent === true);
</script>

<svelte:head>
  <title>Sign In — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <div class="text-center">
    <h1 class="text-2xl font-bold">Sign in</h1>
    <p class="text-sm text-muted-foreground mt-1">We'll email you a magic link.</p>
  </div>

  {#if sent}
    <div class="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-800 text-center">
      If that email is registered, a link is on its way.
    </div>
    <p class="text-center text-sm text-muted-foreground">
      <a href="/auth/magic-link" class="underline hover:text-foreground">Send another link</a>
    </p>
  {:else}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      <div class="flex flex-col gap-1">
        <label for="email" class="text-sm font-medium">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          autofocus
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="you@example.com"
        />
      </div>

      <button
        type="submit"
        class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
      >
        Send Magic Link
      </button>
    </form>

    <p class="text-center text-sm text-muted-foreground">
      Don't have an account?
      <a href="/auth/register" class="underline hover:text-foreground">Register with an invite</a>
    </p>
  {/if}
</div>
