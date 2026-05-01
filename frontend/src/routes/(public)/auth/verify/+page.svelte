<!-- frontend/src/routes/(public)/auth/verify/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { LogIn, ArrowRight, AlertTriangle } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
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
  <title>{m.auth_verify_page_title()}</title>
</svelte:head>

<div class="flex flex-col gap-6 text-center" use:reveal>
  <div>
    <h1 class="text-2xl font-bold">{m.auth_verify_title()}</h1>
    {#if !isExpired}
      <p class="text-sm font-semibold text-brand-text-muted mt-1">{m.auth_verify_subtitle()}</p>
    {/if}
  </div>

  {#if isExpired}
    <div
      role="alert"
      class="rounded-2xl border-[2.5px] border-red-300 bg-red-50 px-4 py-3 flex items-start gap-3 text-left"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      <AlertTriangle size={20} strokeWidth={2.5} class="text-red-600 shrink-0 mt-0.5" />
      <p class="text-sm font-bold text-red-700 leading-snug">{m.auth_verify_expired()}</p>
    </div>
    <a
      href="/auth/magic-link"
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      class="inline-flex items-center justify-center gap-2 h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold px-7"
    >
      <ArrowRight size={18} strokeWidth={2.5} />
      {m.auth_verify_request_new()}
    </a>
  {:else}
    {#if data.token}
      <form method="POST" use:enhance>
        <input type="hidden" name="token" value={data.token} />
        <input type="hidden" name="next" value={data.next} />
        <button
          type="submit"
          use:pressPhysics={'dark'}
          use:hoverEffect={'swap'}
          class="w-full h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold cursor-pointer inline-flex items-center justify-center gap-2"
        >
          <LogIn size={18} strokeWidth={2.5} />
          {m.auth_verify_submit()}
        </button>
      </form>
    {:else}
      <div
        role="alert"
        class="rounded-2xl border-[2.5px] border-red-300 bg-red-50 px-4 py-3 flex items-start gap-3 text-left"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
      >
        <AlertTriangle size={20} strokeWidth={2.5} class="text-red-600 shrink-0 mt-0.5" />
        <p class="text-sm font-bold text-red-700 leading-snug">{m.auth_verify_no_token()}</p>
      </div>
      <a href="/auth/magic-link" class="underline text-sm font-bold hover:text-brand-text text-brand-text-muted">
        {m.auth_verify_request_new()}
      </a>
    {/if}
  {/if}
</div>
