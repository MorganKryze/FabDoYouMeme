<!-- frontend/src/routes/(public)/auth/register/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { UserPlus, CheckCircle, Mail } from '$lib/icons';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let smtpWarning = $derived(form?.warning === 'smtp_failure');
  let success = $derived(form?.success === true);
  // Seed all fields from the server-returned form object exactly once;
  // inputs are user-owned after that, so re-renders (e.g. toggling a
  // checkbox) don't reset what the user typed.
  let inviteToken = $state(untrack(() => form?.invite_token ?? data.inviteToken ?? ''));
  let username = $state(untrack(() => form?.username ?? ''));
  let email = $state(untrack(() => form?.email ?? ''));
  let consent = $state(untrack(() => form?.consent ?? false));
  let ageAffirmation = $state(untrack(() => form?.age_affirmation ?? false));
  let canSubmit = $derived(consent && ageAffirmation);

  // Captured at submit-time so the success card always shows the email the
  // server actually received, not whatever happens to be in the input now.
  let submittedEmail = $state('');

  // Dismiss stale server errors as soon as the user edits anything.
  // Reset whenever a new form response arrives so fresh errors are shown.
  let errorDismissed = $state(false);
  $effect(() => {
    form;
    errorDismissed = false;
  });
  let displayError = $derived(errorDismissed ? null : form?.error);

  function clearError() {
    errorDismissed = true;
  }
</script>

<svelte:head>
  <title>Register — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6" use:reveal>
  {#if !success}
    <div class="text-center">
      <h1 class="text-2xl font-bold">Create your account</h1>
      <p class="text-sm font-semibold text-brand-text-muted mt-1">You need an invite to join.</p>
    </div>
  {/if}

  {#if success && !smtpWarning}
    <div class="flex flex-col items-center gap-5 text-center">
      <div
        class="flex h-20 w-20 items-center justify-center rounded-full border-[2.5px] border-brand-border-heavy bg-brand-success-soft"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.08);"
      >
        <CheckCircle size={40} strokeWidth={2.5} class="text-brand-success" />
      </div>

      <div class="flex flex-col gap-1">
        <h1 class="text-2xl font-bold text-brand-success">You're in!</h1>
        <p class="text-sm font-semibold text-brand-text">Your account has been created.</p>
      </div>
    </div>

    <div class="flex flex-col gap-3">
      <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted text-center">Next steps</p>

      <ol class="flex flex-col gap-3">
        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">1</span>
          <div class="flex flex-col gap-1 min-w-0">
            <span class="text-sm font-bold leading-snug">Check your inbox</span>
            <span class="flex items-center gap-1.5 text-xs font-semibold text-brand-text-muted min-w-0">
              <Mail size={12} strokeWidth={2.5} class="shrink-0" />
              <span class="truncate">{submittedEmail}</span>
            </span>
          </div>
        </li>

        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">2</span>
          <div class="flex flex-col gap-1">
            <span class="text-sm font-bold leading-snug">Click the sign-in link</span>
            <span class="text-xs font-semibold text-brand-text-muted leading-snug">
              Look for an email from FabDoYouMeme and tap the button inside.
            </span>
          </div>
        </li>

        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">3</span>
          <div class="flex flex-col gap-1">
            <span class="text-sm font-bold leading-snug">You're signed in</span>
            <span class="text-xs font-semibold text-brand-text-muted leading-snug">
              The link will take you straight to your home page.
            </span>
          </div>
        </li>
      </ol>

      <p class="text-xs font-semibold text-brand-text-muted leading-snug text-center mt-1">
        The link expires soon. If you don't see it, check your spam folder.
      </p>
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
    <form
      method="POST"
      use:enhance={() => {
        const snapshot = email;
        return async ({ result, update }) => {
          if (result.type === 'success') {
            submittedEmail = snapshot;
          }
          await update();
        };
      }}
      class="flex flex-col gap-4"
    >
      {#if displayError}
        <div
          class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 text-sm font-bold"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {displayError}
        </div>
      {/if}

      <div class="flex flex-col gap-1">
        <label for="invite_token" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Invite Token</label>
        <input
          id="invite_token"
          name="invite_token"
          type="text"
          required
          bind:value={inviteToken}
          oninput={clearError}
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
          bind:value={username}
          oninput={clearError}
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
          bind:value={email}
          oninput={clearError}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder="you@example.com"
        />
      </div>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="consent"
          bind:checked={consent}
          onchange={clearError}
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
          onchange={clearError}
          class="mt-0.5 h-4 w-4 rounded border-brand-border-heavy"
          required
        />
        <span class="text-sm font-semibold leading-snug">I confirm I am at least 16 years old.</span>
      </label>

      <button
        type="submit"
        disabled={!canSubmit}
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold cursor-pointer inline-flex items-center justify-center gap-2 disabled:cursor-not-allowed disabled:opacity-50"
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
