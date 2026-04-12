<!-- frontend/src/routes/(public)/auth/register/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { UserPlus } from '$lib/icons';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let smtpWarning = $derived(form?.warning === 'smtp_failure');
  let success = $derived(form?.success === true);
  // Seed checkbox state from the server-returned form object exactly once;
  // the checkboxes are user-owned after that. See `untrack` usage note above.
  let consent = $state(untrack(() => form?.consent ?? false));
  let ageAffirmation = $state(untrack(() => form?.age_affirmation ?? false));
</script>

<svelte:head>
  <title>Register — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6" use:reveal>
  <div class="text-center">
    <h1 class="text-2xl font-bold">Create your account</h1>
    <p class="text-sm font-semibold text-brand-text-muted mt-1">You need an invite to join.</p>
  </div>

  {#if success && !smtpWarning}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-center"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      Account created! Check your email for your login link.
    </div>
  {/if}

  {#if smtpWarning}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-center"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      Account created, but the login email couldn't be sent. Ask your admin to resend.
    </div>
  {/if}

  {#if !success}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      {#if form?.error}
        <div
          class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 text-sm font-bold"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {form.error}
        </div>
      {/if}

      <div class="flex flex-col gap-1">
        <label for="invite_token" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Invite Token</label>
        <input
          id="invite_token"
          name="invite_token"
          type="text"
          required
          value={form?.invite_token ?? data.inviteToken}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder="abc123xyz789"
        />
      </div>

      <div class="flex flex-col gap-1">
        <label for="username" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          required
          minlength={3}
          maxlength={30}
          autocomplete="username"
          value={form?.username ?? ''}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder="your_username"
        />
        <p class="text-xs font-semibold text-brand-text-muted">3–30 characters, letters, numbers, underscore.</p>
      </div>

      <div class="flex flex-col gap-1">
        <label for="email" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          value={form?.email ?? ''}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder="you@example.com"
        />
      </div>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="consent"
          bind:checked={consent}
          class="mt-0.5 h-4 w-4 rounded border-brand-border-heavy"
          required
        />
        <span class="text-sm font-semibold leading-snug">
          I have read and agree to the
          <a href="/privacy" class="underline hover:text-brand-text" target="_blank">Privacy Policy</a>.
        </span>
      </label>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="age_affirmation"
          bind:checked={ageAffirmation}
          class="mt-0.5 h-4 w-4 rounded border-brand-border-heavy"
          required
        />
        <span class="text-sm font-semibold leading-snug">I confirm I am at least 16 years old.</span>
      </label>

      <button
        type="submit"
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <UserPlus size={18} strokeWidth={2.5} />
        Create Account
      </button>
    </form>

    <p class="text-center text-sm font-semibold text-brand-text-muted">
      Already have an account?
      <a href="/auth/magic-link" class="underline hover:text-brand-text">Sign in</a>
    </p>
  {/if}
</div>
