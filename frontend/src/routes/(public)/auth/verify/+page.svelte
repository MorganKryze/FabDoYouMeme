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
      <p class="text-sm font-semibold text-brand-text-muted mt-1">Click below to log in to your account.</p>
    {/if}
  </div>

  {#if isExpired}
    <div
      class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      This link has expired or already been used.
    </div>
    <a
      href="/auth/magic-link"
      class="inline-flex items-center justify-center h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold px-7 transition-colors"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.35);"
    >
      Request a new link
    </a>
  {:else}
    {#if data.token}
      <form method="POST" use:enhance>
        <input type="hidden" name="token" value={data.token} />
        <input type="hidden" name="next" value={data.next} />
        <button
          type="submit"
          class="w-full h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold transition-colors cursor-pointer"
          style="box-shadow: 0 5px 0 rgba(0,0,0,0.35);"
        >
          Log In
        </button>
      </form>
    {:else}
      <div
        class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
      >
        No token found in this link. Please check the email and try again.
      </div>
      <a href="/auth/magic-link" class="underline text-sm font-bold hover:text-brand-text text-brand-text-muted">
        Request a new link
      </a>
    {/if}
  {/if}
</div>
