<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { goto, invalidateAll } from '$app/navigation';
  import { authApi } from '$lib/api/auth';
  import { Download, Save as SaveIcon, XCircle, Mail, Edit as EditIcon, LogOut } from '$lib/icons';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import ToneSlider from '$lib/components/ToneSlider.svelte';
  import ToneSamplePreview from '$lib/components/ToneSamplePreview.svelte';
  import type { ActionData, PageData } from './$types';
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let editingUsername = $state(false);
  let editingEmail = $state(false);
  // Imperative focus for the inline edit inputs — replaces the raw
  // `autofocus` attribute so screen readers announce the focus change
  // when a user switches into an edit mode (a11y_autofocus).
  let usernameInput = $state<HTMLInputElement | null>(null);
  let emailInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (editingUsername && usernameInput) usernameInput.focus();
  });
  $effect(() => {
    if (editingEmail && emailInput) emailInput.focus();
  });

  // `use:enhance` updates the `form` prop several times per submission
  // (pending → result → post-invalidate refetch), each update firing the
  // effect. Without this guard we got 3× toasts. A plain `let` (not
  // `$state`) skips reactivity, so writing from inside the effect is safe.
  let lastForm: ActionData | undefined;
  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.usernameSuccess) {
      editingUsername = false;
      toast.show(m.profile_username_toast_updated(), 'success');
    }
    if (form?.emailSent) {
      editingEmail = false;
      toast.show(m.profile_email_toast_sent(), 'success');
    }
  });

  async function logout() {
    try {
      await authApi.logout();
    } catch {
      /* non-fatal — session cleared server-side on next request */
    }
    await invalidateAll();
    await goto('/');
  }

  async function downloadExport() {
    try {
      const res = await fetch('/api/users/me/export');
      if (!res.ok) throw new Error(`Export failed: ${res.status}`);
      let blob: Blob;
      try {
        blob = await res.blob();
      } catch {
        toast.show(m.profile_export_prepare_error(), 'error');
        return;
      }
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'my-fabdoyoumeme-data.json';
      a.click();
      URL.revokeObjectURL(url);
      toast.show(m.profile_export_success(), 'success');
    } catch {
      toast.show(m.profile_export_error(), 'error');
    }
  }
</script>

<svelte:head>
  <title>{m.profile_page_title()}</title>
</svelte:head>

<div class="w-full max-w-lg mx-auto p-6 flex flex-col gap-6" use:reveal>
  <h1 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
    {m.profile_title()}
  </h1>

  <!-- ─── Identity card ─────────────────────────────────── -->
  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-5"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.profile_identity()}
    </h2>

    <!-- Username -->
    <div class="flex flex-col gap-2">
      <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">{m.profile_username_label()}</p>
      {#if editingUsername}
        <form method="POST" action="?/updateUsername" use:enhance class="flex flex-col gap-2">
          {#if form?.usernameError}
            <p class="text-sm font-bold text-red-600">{form.usernameError}</p>
          {/if}
          <div class="flex gap-2">
            <input
              bind:this={usernameInput}
              name="username"
              type="text"
              value={data.user.username}
              minlength={3}
              maxlength={30}
              class="flex-1 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
            />
            <button
              use:pressPhysics={'dark'}
              use:hoverEffect={'swap'}
              type="submit"
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
            >
              <SaveIcon size={16} strokeWidth={2.5} />
              {m.profile_save()}
            </button>
            <button
              use:pressPhysics={'ghost'}
              use:hoverEffect={'swap'}
              type="button"
              onclick={() => editingUsername = false}
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
            >
              <XCircle size={16} strokeWidth={2.5} />
              {m.profile_cancel()}
            </button>
          </div>
        </form>
      {:else}
        <div class="flex items-center gap-3">
          <span class="text-sm font-bold">{data.user.username}</span>
          <button
            type="button"
            onclick={() => editingUsername = true}
            class="inline-flex items-center gap-1 text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors"
          >
            <EditIcon size={12} strokeWidth={2.5} />
            {m.profile_edit()}
          </button>
        </div>
      {/if}
    </div>

    <!-- Email -->
    <div class="flex flex-col gap-2">
      <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">{m.profile_email_label()}</p>
      {#if editingEmail}
        <form method="POST" action="?/requestEmailChange" use:enhance class="flex flex-col gap-2">
          {#if form?.emailError}
            <p class="text-sm font-bold text-red-600">{form.emailError}</p>
          {/if}
          <div class="flex gap-2">
            <input
              bind:this={emailInput}
              name="email"
              type="email"
              class="flex-1 h-11 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-4 text-sm font-semibold focus:outline-none focus:border-brand-text transition-colors"
              placeholder={m.profile_email_new_placeholder()}
            />
            <button
              use:pressPhysics={'dark'}
              use:hoverEffect={'swap'}
              type="submit"
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
            >
              <Mail size={16} strokeWidth={2.5} />
              {m.profile_email_verify()}
            </button>
            <button
              use:pressPhysics={'ghost'}
              use:hoverEffect={'swap'}
              type="button"
              onclick={() => editingEmail = false}
              class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
            >
              <XCircle size={16} strokeWidth={2.5} />
              {m.profile_cancel()}
            </button>
          </div>
          <p class="text-xs font-semibold text-brand-text-muted">{m.profile_email_change_pending_hint()}</p>
        </form>
      {:else}
        <div class="flex items-center gap-3">
          <span class="text-sm font-bold">{data.user.email}</span>
          <button
            type="button"
            onclick={() => editingEmail = true}
            class="inline-flex items-center gap-1 text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors"
          >
            <EditIcon size={12} strokeWidth={2.5} />
            {m.profile_change_email()}
          </button>
        </div>
      {/if}
    </div>
  </section>

  <!-- ─── Appearance card ───────────────────────────────── -->
  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-5"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.profile_appearance()}
    </h2>

    <div class="flex flex-col gap-2">
      <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">{m.profile_theme()}</p>
      <ThemeToggle />
      <p class="text-xs font-semibold text-brand-text-muted">
        {m.profile_theme_hint()}
      </p>
    </div>

    <div class="flex flex-col gap-2">
      <p class="text-[0.6rem] font-bold uppercase tracking-[0.15em] text-brand-text-muted">{m.profile_vibes()}</p>
      <p class="text-sm font-semibold">{m.profile_greeting_tone()}</p>
      <p class="text-xs font-semibold text-brand-text-muted -mt-1">
        {m.profile_greeting_tone_hint()}
      </p>
      <ToneSlider />
      <ToneSamplePreview username={data.user?.username ?? m.profile_greeting_tone_default_name()} />
    </div>
  </section>

  <!-- ─── Data & Privacy card ───────────────────────────── -->
  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-5"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.profile_data_privacy()}
    </h2>

    <div class="flex flex-col gap-2">
      <p class="text-sm font-semibold text-brand-text-mid">
        {m.profile_download_body()}
      </p>
      <button
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        type="button"
        onclick={downloadExport}
        class="self-start h-11 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
      >
        <Download size={16} strokeWidth={2.5} />
        {m.profile_download_data()}
      </button>
    </div>

    <div class="flex flex-col gap-1">
      <p class="text-sm font-bold">{m.profile_delete_account_title()}</p>
      <p class="text-sm font-semibold text-brand-text-mid">
        {m.profile_delete_account_body_prefix()}
        <a href="/privacy" class="underline hover:text-brand-text">{m.common_privacy_policy()}</a>
        {m.profile_delete_account_body_suffix()}
      </p>
    </div>
  </section>

  <!-- ─── Session card ──────────────────────────────────── -->
  <section
    class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-3"
    style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
  >
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
      {m.profile_session()}
    </h2>
    <button
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      type="button"
      onclick={logout}
      class="self-start h-11 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <LogOut size={16} strokeWidth={2.5} />
      {m.profile_log_out()}
    </button>
  </section>
</div>

<footer class="border-t border-brand-border w-full px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted">
  <p>{m.profile_footer_copyright({ year: new Date().getFullYear() })}</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">{m.common_privacy_policy()}</a>
</footer>
