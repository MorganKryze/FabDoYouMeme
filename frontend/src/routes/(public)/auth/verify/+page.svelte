<!-- frontend/src/routes/(public)/auth/verify/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let isExpired = $derived(
    form?.error === 'invalid_token' ||
    form?.error === 'token_expired' ||
    form?.error === 'token_used' ||
    form?.error === 'account_inactive'
  );
</script>

<svelte:head>
  <title>Log In — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6 text-center">
  <div>
    <h1 class="text-2xl font-bold">Welcome back</h1>
    {#if !isExpired}
      <p class="text-sm text-muted-foreground mt-1">Click below to log in to your account.</p>
    {/if}
  </div>

  {#if isExpired}
    <div class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
      This link has expired or already been used.
    </div>
    <a
      href="/auth/magic-link"
      class="inline-block h-11 rounded-lg bg-primary text-primary-foreground font-semibold leading-[2.75rem] hover:bg-primary/90 transition-colors px-6"
    >
      Request a new link →
    </a>
  {:else}
    {#if data.token}
      <form method="POST" use:enhance>
        <input type="hidden" name="token" value={data.token} />
        <input type="hidden" name="next" value={data.next} />
        <button
          type="submit"
          class="w-full h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
        >
          Log In
        </button>
      </form>
    {:else}
      <div class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        No token found in this link. Please check the email and try again.
      </div>
      <a href="/auth/magic-link" class="underline text-sm hover:text-foreground">
        Request a new link →
      </a>
    {/if}
  {/if}
</div>
