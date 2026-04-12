<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { reveal } from '$lib/actions/reveal';
  import type { ActionData, PageData } from './$types';

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

<div class="max-w-lg mx-auto p-6 flex flex-col gap-8" use:reveal>
  <h1 style="font-size: clamp(2rem, 4.5vw, 3.2rem); font-weight: 700; line-height: 1; letter-spacing: -0.02em;">
    Maker Card
  </h1>

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
            type="submit"
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer"
          >
            Save
          </button>
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={() => editingUsername = false}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer"
          >
            Cancel
          </button>
        </div>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm font-bold">{data.user.username}</span>
        <button type="button" onclick={() => editingUsername = true}
          class="text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors">
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
            type="submit"
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-brand-text text-brand-white text-sm font-bold cursor-pointer"
          >
            Verify
          </button>
          <button
            use:pressPhysics={'ghost'}
            type="button"
            onclick={() => editingEmail = false}
            class="h-11 px-5 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold cursor-pointer"
          >
            Cancel
          </button>
        </div>
        <p class="text-xs font-semibold text-brand-text-muted">Your current email stays active until you click the verification link.</p>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm font-bold">{data.user.email}</span>
        <button type="button" onclick={() => editingEmail = true}
          class="text-xs font-bold text-brand-text-muted underline hover:text-brand-text transition-colors">
          Change Email
        </button>
      </div>
    {/if}
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
        type="button"
        onclick={downloadExport}
        class="self-start h-11 px-6 rounded-full border-[2.5px] border-brand-border-heavy bg-transparent text-sm font-bold transition-colors cursor-pointer"
      >
        Download My Data
      </button>
    </div>

    <div
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
</div>
