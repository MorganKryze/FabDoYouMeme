<!-- frontend/src/routes/(public)/auth/magic-link/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let sent = $derived(form?.sent === true);
  // Imperative focus for the email field — replaces raw `autofocus`.
  let emailInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (!sent && emailInput) emailInput.focus();
  });
</script>

<svelte:head>
  <title>Sign In — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <div class="text-center">
    <h1 class="text-2xl font-bold">Sign in</h1>
    <p class="text-sm font-semibold text-brand-text-muted mt-1">We'll email you a magic link.</p>
  </div>

  {#if sent}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-center"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      If that email is registered, a link is on its way.
    </div>
    <p class="text-center text-sm font-semibold text-brand-text-muted">
      <a href="/auth/magic-link" class="underline hover:text-brand-text">Send another link</a>
    </p>
  {:else}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      <div class="flex flex-col gap-1">
        <label for="email" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Email</label>
        <input
          id="email"
          name="email"
          bind:this={emailInput}
          type="email"
          required
          autocomplete="email"
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder="you@example.com"
        />
      </div>

      <button
        type="submit"
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold transition-colors cursor-pointer"
        style="box-shadow: 0 5px 0 rgba(0,0,0,0.35);"
      >
        Send Magic Link
      </button>
    </form>

    <p class="text-center text-sm font-semibold text-brand-text-muted">
      Don't have an account?
      <a href="/auth/register" class="underline hover:text-brand-text">Register with an invite</a>
    </p>
  {/if}
</div>
