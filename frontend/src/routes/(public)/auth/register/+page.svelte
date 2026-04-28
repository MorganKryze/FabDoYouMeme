<!-- frontend/src/routes/(public)/auth/register/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { UserPlus, CheckCircle, Mail } from '$lib/icons';
  import * as m from '$lib/paraglide/messages';
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

  // Phase 2 — when a platform+group invite is in the URL, we hide the
  // generic invite_token input (the URL token replaces it) and show the
  // target group identity. NSFW groups demand an extra age-affirmation
  // checkbox; non-NSFW groups skip it entirely.
  const groupInviteToken = $derived(data.groupInviteToken);
  const groupPreview = $derived(data.groupPreview);
  const isPlatformPlus = $derived(!!groupInviteToken && !!groupPreview);
  const groupIsNSFW = $derived(!!groupPreview && groupPreview.group.classification === 'nsfw');
  let nsfwAgeAffirmation = $state(untrack(() => form?.nsfw_age_affirmation ?? false));

  let canSubmit = $derived(consent && ageAffirmation && (!groupIsNSFW || nsfwAgeAffirmation));

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
  <title>{m.auth_register_page_title()}</title>
</svelte:head>

<div class="flex flex-col gap-4 sm:gap-6" use:reveal>
  {#if !success}
    <div class="text-center">
      <h1 class="text-2xl font-bold">{m.auth_register_title()}</h1>
      <p class="hidden sm:block text-sm font-semibold text-brand-text-muted mt-1">{m.auth_register_subtitle()}</p>
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
        <h1 class="text-2xl font-bold text-brand-success">{m.auth_register_success_title()}</h1>
        <p class="text-sm font-semibold text-brand-text">{m.auth_register_success_body()}</p>
      </div>
    </div>

    <div class="flex flex-col gap-3">
      <p class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted text-center">{m.auth_register_next_steps()}</p>

      <ol class="flex flex-col gap-3">
        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">1</span>
          <div class="flex flex-col gap-1 min-w-0">
            <span class="text-sm font-bold leading-snug">{m.auth_register_step1_title()}</span>
            <span class="flex items-center gap-1.5 text-xs font-semibold text-brand-text-muted min-w-0">
              <Mail size={12} strokeWidth={2.5} class="shrink-0" />
              <span class="truncate">{submittedEmail}</span>
            </span>
          </div>
        </li>

        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">2</span>
          <div class="flex flex-col gap-1">
            <span class="text-sm font-bold leading-snug">{m.auth_register_step2_title()}</span>
            <span class="text-xs font-semibold text-brand-text-muted leading-snug">
              {m.auth_register_step2_body()}
            </span>
          </div>
        </li>

        <li class="flex items-start gap-3 rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3" style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);">
          <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-[2px] border-brand-border-heavy bg-brand-success-soft text-xs font-bold text-brand-success">3</span>
          <div class="flex flex-col gap-1">
            <span class="text-sm font-bold leading-snug">{m.auth_register_step3_title()}</span>
            <span class="text-xs font-semibold text-brand-text-muted leading-snug">
              {m.auth_register_step3_body()}
            </span>
          </div>
        </li>
      </ol>

      <p class="text-xs font-semibold text-brand-text-muted leading-snug text-center mt-1">
        {m.auth_register_link_expires()}
      </p>
    </div>
  {/if}

  {#if smtpWarning}
    <div
      class="rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-5 py-3 text-sm font-bold text-center"
      style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
    >
      {m.auth_register_smtp_warning()}
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
      class="flex flex-col gap-3 sm:gap-4"
    >
      {#if displayError}
        <div
          class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 text-sm font-bold"
          style="box-shadow: 0 4px 0 rgba(0,0,0,0.06);"
        >
          {displayError}
        </div>
      {/if}

      {#if isPlatformPlus && groupPreview}
        <!-- Platform+group invite: identify the target group instead of
             asking for a token, and pass the URL token through hidden. -->
        <input type="hidden" name="group_invite_token" value={groupInviteToken} />
        <div
          class="rounded-2xl border-[2.5px] border-brand-border-heavy bg-brand-white px-4 py-3 flex flex-col gap-1"
          style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
        >
          <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">
            {m.auth_register_joining_group_kicker()}
          </p>
          <p class="text-sm font-bold m-0">{groupPreview.group.name}</p>
          <p class="text-xs text-brand-text-muted m-0">{groupPreview.group.description}</p>
        </div>
      {:else}
        <div class="flex flex-col gap-1">
          <label for="invite_token" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">{m.auth_register_invite_label()}</label>
          <input
            id="invite_token"
            name="invite_token"
            type="text"
            required
            bind:value={inviteToken}
            oninput={clearError}
            class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
            placeholder={m.auth_register_invite_placeholder()}
          />
        </div>
      {/if}

      <div class="flex flex-col gap-1">
        <label for="username" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">{m.auth_register_username_label()}</label>
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
          placeholder={m.auth_register_username_placeholder()}
        />
        <p class="text-xs font-semibold text-brand-text-muted">{m.auth_register_username_hint()}</p>
      </div>

      <div class="flex flex-col gap-1">
        <label for="email" class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">{m.auth_register_email_label()}</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          bind:value={email}
          oninput={clearError}
          class="h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
          placeholder={m.auth_register_email_placeholder()}
        />
      </div>

      <label class="flex items-start gap-2 cursor-pointer">
        <input
          type="checkbox"
          name="consent"
          bind:checked={consent}
          onchange={clearError}
          class="h-4 w-4 rounded border-brand-border-heavy"
          required
        />
        <span class="text-sm font-semibold leading-tight">
          {m.auth_register_consent_prefix()}
          <a href="/privacy" class="underline hover:text-brand-text" target="_blank">{m.common_privacy_policy()}</a>{m.auth_register_consent_suffix()}
        </span>
      </label>

      <label class="flex items-start gap-2 cursor-pointer">
        <input
          type="checkbox"
          name="age_affirmation"
          bind:checked={ageAffirmation}
          onchange={clearError}
          class="h-4 w-4 rounded border-brand-border-heavy"
          required
        />
        <span class="text-sm font-semibold leading-tight">{m.auth_register_age_affirmation()}</span>
      </label>

      {#if groupIsNSFW}
        <label class="flex items-start gap-3 cursor-pointer">
          <input
            type="checkbox"
            name="nsfw_age_affirmation"
            bind:checked={nsfwAgeAffirmation}
            onchange={clearError}
            class="mt-0.5 h-4 w-4 rounded border-brand-border-heavy"
            required
          />
          <span class="text-sm font-semibold leading-snug">{m.groups_join_nsfw_age_affirmation()}</span>
        </label>
      {/if}

      <button
        type="submit"
        disabled={!canSubmit}
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold cursor-pointer inline-flex items-center justify-center gap-2 disabled:cursor-not-allowed disabled:opacity-50"
      >
        <UserPlus size={18} strokeWidth={2.5} />
        {m.auth_register_submit()}
      </button>
    </form>

    <p class="text-center text-sm font-semibold text-brand-text-muted">
      {m.auth_register_signin_prompt()}
      <a href="/auth/magic-link" class="underline hover:text-brand-text">{m.auth_register_signin_link()}</a>
    </p>
  {/if}
</div>
