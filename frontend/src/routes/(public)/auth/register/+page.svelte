<!-- frontend/src/routes/(public)/auth/register/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let smtpWarning = $derived(form?.warning === 'smtp_failure');
  let success = $derived(form?.success === true);
</script>

<svelte:head>
  <title>Register — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6">
  <div class="text-center">
    <h1 class="text-2xl font-bold">Create your account</h1>
    <p class="text-sm text-muted-foreground mt-1">You need an invite to join.</p>
  </div>

  {#if success && !smtpWarning}
    <div class="rounded-lg border border-green-200 bg-green-50 p-4 text-sm text-green-800">
      Account created! Check your email for your login link.
    </div>
  {/if}

  {#if smtpWarning}
    <div class="rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800">
      Account created, but the login email couldn't be sent. Ask your admin to resend.
    </div>
  {/if}

  {#if !success}
    <form method="POST" use:enhance class="flex flex-col gap-4">
      {#if form?.error}
        <div class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {form.error}
        </div>
      {/if}

      <div class="flex flex-col gap-1">
        <label for="invite_token" class="text-sm font-medium">Invite Token</label>
        <input
          id="invite_token"
          name="invite_token"
          type="text"
          required
          value={form?.invite_token ?? data.inviteToken}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="abc123xyz789"
        />
      </div>

      <div class="flex flex-col gap-1">
        <label for="username" class="text-sm font-medium">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          required
          minlength={3}
          maxlength={30}
          autocomplete="username"
          value={form?.username ?? ''}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="your_username"
        />
        <p class="text-xs text-muted-foreground">3–30 characters, letters, numbers, underscore.</p>
      </div>

      <div class="flex flex-col gap-1">
        <label for="email" class="text-sm font-medium">Email</label>
        <input
          id="email"
          name="email"
          type="email"
          required
          autocomplete="email"
          value={form?.email ?? ''}
          class="h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          placeholder="you@example.com"
        />
      </div>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="consent"
          value="true"
          class="mt-0.5 h-4 w-4 rounded border-input"
          required
        />
        <span class="text-sm leading-snug">
          I have read and agree to the
          <a href="/privacy" class="underline hover:text-foreground" target="_blank">Privacy Policy</a>.
        </span>
      </label>

      <label class="flex items-start gap-3 cursor-pointer">
        <input
          type="checkbox"
          name="age_affirmation"
          value="true"
          class="mt-0.5 h-4 w-4 rounded border-input"
          required
        />
        <span class="text-sm leading-snug">I confirm I am at least 16 years old.</span>
      </label>

      <button
        type="submit"
        class="h-11 rounded-lg bg-primary text-primary-foreground font-semibold hover:bg-primary/90 transition-colors"
      >
        Create Account
      </button>
    </form>

    <p class="text-center text-sm text-muted-foreground">
      Already have an account?
      <a href="/auth/magic-link" class="underline hover:text-foreground">Sign in</a>
    </p>
  {/if}
</div>
