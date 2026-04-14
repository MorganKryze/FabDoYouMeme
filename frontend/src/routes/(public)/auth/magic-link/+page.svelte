<!-- frontend/src/routes/(public)/auth/magic-link/+page.svelte -->
<script lang="ts">
  import { enhance } from '$app/forms';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Send } from '$lib/icons';
  import type { ActionData } from './$types';

  let { form }: { form: ActionData } = $props();
  let sent = $derived(form?.sent === true);
  let sentEmail = $derived(form?.email ?? '');

  // Imperative focus for the email field — replaces raw `autofocus`.
  let emailInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (!sent && emailInput) emailInput.focus();
  });

  // 60-second cooldown after sending. Counts down each second; at 0 the
  // resend link becomes active. Timer is cleared on teardown (e.g. navigation).
  // `resendTick` is bumped by the resend form's enhance callback so the
  // effect re-runs and restarts the timer even when `sent` stays true.
  let cooldownSeconds = $state(0);
  let resendTick = $state(0);
  let justResent = $state(false);
  $effect(() => {
    resendTick;
    if (!sent) return;
    cooldownSeconds = 60;
    const id = setInterval(() => {
      cooldownSeconds -= 1;
      if (cooldownSeconds <= 0) clearInterval(id);
    }, 1000);
    return () => clearInterval(id);
  });
</script>

<svelte:head>
  <title>Sign in — FabDoYouMeme</title>
</svelte:head>

<div class="flex flex-col gap-6" use:reveal>
  <div class="text-center">
    <h1 class="text-2xl font-bold">
      {sent ? 'Check your inbox' : 'Sign in'}
    </h1>
    <p class="text-sm font-semibold text-brand-text-muted mt-1">
      {sent ? "Magic link sent — here's what to do." : "We'll email you a magic link."}
    </p>
  </div>

  {#if sent}
    <!-- Step checklist -->
    <div class="flex flex-col gap-3">
      <!-- Step 1: done -->
      <div class="flex items-start gap-3">
        <div
          class="w-[22px] h-[22px] rounded-full border-[2px] border-brand-border-heavy bg-brand-text text-brand-white flex items-center justify-center flex-shrink-0 mt-0.5 text-[0.6rem] font-bold"
        >
          ✓
        </div>
        <div>
          <p class="text-sm font-semibold leading-snug">Enter your email</p>
          <p class="text-xs text-brand-text-muted font-medium mt-0.5">Done — link is on its way</p>
        </div>
      </div>

      <!-- Step 2 -->
      <div class="flex items-start gap-3">
        <div
          class="w-[22px] h-[22px] rounded-full border-[2px] border-brand-border-heavy bg-brand-white text-brand-text flex items-center justify-center flex-shrink-0 mt-0.5 text-[0.6rem] font-bold"
        >
          2
        </div>
        <div>
          <p class="text-sm font-semibold leading-snug">Open the email</p>
          <p class="text-xs text-brand-text-muted font-medium mt-0.5">Check spam if you don't see it</p>
        </div>
      </div>

      <!-- Step 3 -->
      <div class="flex items-start gap-3">
        <div
          class="w-[22px] h-[22px] rounded-full border-[2px] border-brand-border-heavy bg-brand-white text-brand-text flex items-center justify-center flex-shrink-0 mt-0.5 text-[0.6rem] font-bold"
        >
          3
        </div>
        <div>
          <p class="text-sm font-semibold leading-snug">Click the link</p>
          <p class="text-xs text-brand-text-muted font-medium mt-0.5">You'll be signed in automatically</p>
        </div>
      </div>
    </div>

    <hr class="border-t border-brand-border" />

    <div class="flex flex-col items-center gap-2">
      <p
        aria-live="polite"
        class="text-xs {justResent ? 'text-brand-text font-bold' : 'text-brand-text-muted font-medium'}"
      >
        {justResent ? '✓ Link sent again — check your inbox' : 'You can safely close this tab.'}
      </p>

      {#if cooldownSeconds > 0}
        <span class="text-sm font-semibold text-brand-text-muted cursor-not-allowed select-none">
          Send another link
          <span
            class="inline-block rounded-full bg-brand-surface px-2 py-0.5 text-[0.68rem] font-bold text-brand-text-muted"
          >{cooldownSeconds}s</span>
        </span>
      {:else}
        <form
          method="POST"
          use:enhance={() => {
            return async ({ update }) => {
              await update({ reset: false });
              resendTick += 1;
              justResent = true;
              setTimeout(() => { justResent = false; }, 3000);
            };
          }}
        >
          <input type="hidden" name="email" value={sentEmail} />
          <button
            type="submit"
            class="text-sm font-semibold underline hover:text-brand-text cursor-pointer bg-transparent border-0 p-0"
          >
            Send another link
          </button>
        </form>
      {/if}
    </div>
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
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-12 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white font-bold cursor-pointer inline-flex items-center justify-center gap-2"
      >
        <Send size={18} strokeWidth={2.5} />
        Send Magic Link
      </button>
    </form>

    <p class="text-center text-sm font-semibold text-brand-text-muted">
      Don't have an account?
      <a href="/auth/register" class="underline hover:text-brand-text">Register with an invite</a>
    </p>
  {/if}
</div>
