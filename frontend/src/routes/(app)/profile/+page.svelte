<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import { physCard } from '$lib/actions/physCard';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { goto, invalidateAll } from '$app/navigation';
  import { authApi } from '$lib/api/auth';
  import { Download, Save as SaveIcon, XCircle, Mail, Edit as EditIcon, Shield, LogOut, Sparkles } from '$lib/icons';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import ToneSlider from '$lib/components/ToneSlider.svelte';
  import ToneSamplePreview from '$lib/components/ToneSamplePreview.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  // Maker Card identity derivations — the big initial badge and the
  // playful "member ID" are both derived from the authenticated user so
  // the card has visual identity without needing extra backend fields.
  const initialLetter = $derived((data.user.username?.[0] ?? '?').toUpperCase());
  const serial = $derived(
    ((data.user.id ?? '').replace(/-/g, '').slice(0, 6).toUpperCase() || 'XXXXXX')
      .replace(/^(.{3})(.{3})$/, '$1-$2')
  );

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

  $effect(() => {
    if (form?.usernameSuccess) {
      editingUsername = false;
      toast.show('Username updated.', 'success');
    }
    if (form?.emailSent) {
      editingEmail = false;
      toast.show('Check your new email address for a verification link.', 'success');
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
        toast.show('Could not prepare download file. Try again.', 'error');
        return;
      }
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'my-fabyoumeme-data.json';
      a.click();
      URL.revokeObjectURL(url);
      toast.show('Your data export is ready.', 'success');
    } catch {
      toast.show('Export failed. Please try again.', 'error');
    }
  }
</script>

<svelte:head>
  <title>Profile — FabDoYouMeme</title>
</svelte:head>

<div class="w-full max-w-lg mx-auto p-6 flex flex-col gap-8" use:reveal>
  <!-- Maker Card — identity header. This is the page's visual anchor:
       a physical, tilt-reactive card that carries the branding its name
       promises, instead of a bare heading. -->
  <section class="flex flex-col gap-3">
    <div
      use:physCard
      class="relative rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface px-6 py-7 flex items-center gap-5"
      style="box-shadow: 0 6px 0 rgba(0,0,0,0.1);"
    >
      <!-- Corner stamp — doubles as the semantic page title -->
      <div class="absolute top-4 right-4 inline-flex items-center gap-1.5">
        <Sparkles size={11} strokeWidth={2.75} />
        <h1 class="text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted m-0">
          Maker Card
        </h1>
      </div>

      <!-- Initial badge — accent-coloured, slightly tilted for sticker feel -->
      <div
        class="shrink-0 w-20 h-20 rounded-[18px] border-[2.5px] border-brand-border-heavy bg-brand-accent text-brand-text flex items-center justify-center text-4xl font-extrabold select-none"
        style="box-shadow: 0 4px 0 rgba(0,0,0,0.14); transform: rotate(-3deg);"
        aria-hidden="true"
      >
        {initialLetter}
      </div>

      <!-- Identity text -->
      <div class="flex flex-col min-w-0 flex-1 pt-3">
        <p class="text-[0.6rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">
          Signed in as
        </p>
        <p
          class="text-3xl font-extrabold leading-none truncate mt-1 text-brand-accent"
          style="letter-spacing: -0.02em;"
        >
          {data.user.username}
        </p>
        <div class="flex items-center gap-2 mt-3 flex-wrap">
          <span
            class="inline-flex items-center gap-1 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-white px-2.5 py-1 text-[0.6rem] font-bold uppercase tracking-[0.15em]"
            style="box-shadow: 0 2px 0 rgba(0,0,0,0.08);"
          >
            {#if data.user.role === 'admin'}
              <Shield size={10} strokeWidth={3} />
              Admin
            {:else}
              <Sparkles size={10} strokeWidth={3} />
              Maker
            {/if}
          </span>
          <span class="font-mono text-[0.65rem] font-bold text-brand-text-muted tracking-[0.1em]">
            ID · {serial}
          </span>
        </div>
      </div>
    </div>
  </section>

  <!-- Username section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Username</h2>
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
            Save
          </button>
          <button
            use:pressPhysics={'ghost'}
            use:hoverEffect={'swap'}
            type="button"
            onclick={() => editingUsername = false}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
          >
            <XCircle size={16} strokeWidth={2.5} />
            Cancel
          </button>
        </div>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm font-bold">{data.user.username}</span>
        <button type="button" onclick={() => editingUsername = true}
          class="inline-flex items-center gap-1 text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors">
          <EditIcon size={12} strokeWidth={2.5} />
          Edit
        </button>
      </div>
    {/if}
  </section>

  <!-- Email section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Email</h2>
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
            placeholder="new@example.com"
          />
          <button
            use:pressPhysics={'dark'}
            use:hoverEffect={'swap'}
            type="submit"
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
          >
            <Mail size={16} strokeWidth={2.5} />
            Verify
          </button>
          <button
            use:pressPhysics={'ghost'}
            use:hoverEffect={'swap'}
            type="button"
            onclick={() => editingEmail = false}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
          >
            <XCircle size={16} strokeWidth={2.5} />
            Cancel
          </button>
        </div>
        <p class="text-xs font-semibold text-brand-text-muted">Your current email stays active until you click the verification link.</p>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm font-bold">{data.user.email}</span>
        <button type="button" onclick={() => editingEmail = true}
          class="inline-flex items-center gap-1 text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors">
          <EditIcon size={12} strokeWidth={2.5} />
          Change Email
        </button>
      </div>
    {/if}
  </section>

  <!-- Preferences section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Theme</h2>
    <ThemeToggle />
    <p class="text-xs font-semibold text-brand-text-muted">
      Auto matches the time of day. Override stays until you change it.
    </p>
  </section>

  <!-- Vibes — greeting tone preference -->
  <section class="flex flex-col gap-3">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Vibes</h2>
    <p class="text-sm font-semibold">Greeting tone</p>
    <p class="text-xs font-semibold text-brand-text-muted -mt-1">
      How spicy should your dashboard hello be? Drag to taste.
    </p>
    <ToneSlider />
    <ToneSamplePreview username={data.user?.username ?? 'there'} />
  </section>

  <!-- Data & Privacy section -->
  <section class="flex flex-col gap-4">
    <h2 class="text-[0.65rem] font-bold uppercase tracking-[0.2em] text-brand-text-muted">Data & Privacy</h2>

    <div class="flex flex-col gap-2">
      <p class="text-sm font-semibold text-brand-text-mid">
        Download a copy of all your personal data stored in this service.
      </p>
      <button
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        type="button"
        onclick={downloadExport}
        class="self-start h-11 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer inline-flex items-center gap-2"
      >
        <Download size={16} strokeWidth={2.5} />
        Download My Data
      </button>
    </div>

    <div
      use:physCard
      class="rounded-[22px] border-[2.5px] border-brand-border-heavy bg-brand-surface p-5 flex flex-col gap-1"
      style="box-shadow: 0 5px 0 rgba(0,0,0,0.08);"
    >
      <p class="text-sm font-bold">Delete My Account</p>
      <p class="text-sm font-semibold text-brand-text-mid">
        To request deletion of your account and all associated data, contact your admin.
        See the <a href="/privacy" class="underline hover:text-brand-text">Privacy Policy</a> for details.
      </p>
    </div>
  </section>

  <!-- Logout -->
  <section class="flex flex-col gap-3">
    <button
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      type="button"
      onclick={logout}
      class="self-start h-11 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer inline-flex items-center gap-2"
      style="box-shadow: 0 3px 0 rgba(0,0,0,0.06);"
    >
      <LogOut size={16} strokeWidth={2.5} />
      Log out
    </button>
  </section>
</div>

<footer class="border-t border-brand-border px-6 py-6 flex items-center justify-between text-xs font-semibold text-brand-text-muted max-w-lg mx-auto w-full">
  <p>© {new Date().getFullYear()} FabDoYouMeme</p>
  <a href="/privacy" class="hover:text-brand-text transition-colors">Privacy Policy</a>
</footer>
