<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
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

<div class="max-w-lg mx-auto p-6 flex flex-col gap-8">
  <h1 class="text-2xl font-bold">Profile</h1>

  <!-- Username section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Username</h2>
    {#if editingUsername}
      <form method="POST" action="?/updateUsername" use:enhance class="flex flex-col gap-2">
        {#if form?.usernameError}
          <p class="text-sm text-red-600">{form.usernameError}</p>
        {/if}
        <div class="flex gap-2">
          <input
            bind:this={usernameInput}
            name="username"
            type="text"
            value={data.user.username}
            minlength={3}
            maxlength={30}
            class="flex-1 h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
          />
          <button type="submit" class="h-10 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
            Save
          </button>
          <button type="button" onclick={() => editingUsername = false}
            class="h-10 px-4 rounded-md border border-border text-sm hover:bg-muted">
            Cancel
          </button>
        </div>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm">{data.user.username}</span>
        <button type="button" onclick={() => editingUsername = true}
          class="text-xs text-muted-foreground underline hover:text-foreground">
          Edit
        </button>
      </div>
    {/if}
  </section>

  <!-- Email section -->
  <section class="flex flex-col gap-3">
    <h2 class="text-base font-semibold">Email</h2>
    {#if editingEmail}
      <form method="POST" action="?/requestEmailChange" use:enhance class="flex flex-col gap-2">
        {#if form?.emailError}
          <p class="text-sm text-red-600">{form.emailError}</p>
        {/if}
        <div class="flex gap-2">
          <input
            bind:this={emailInput}
            name="email"
            type="email"
            class="flex-1 h-10 rounded-md border border-input bg-background px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="new@example.com"
          />
          <button type="submit" class="h-10 px-4 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
            Send Verification
          </button>
          <button type="button" onclick={() => editingEmail = false}
            class="h-10 px-4 rounded-md border border-border text-sm hover:bg-muted">
            Cancel
          </button>
        </div>
        <p class="text-xs text-muted-foreground">Your current email stays active until you click the verification link.</p>
      </form>
    {:else}
      <div class="flex items-center gap-3">
        <span class="text-sm">{data.user.email}</span>
        <button type="button" onclick={() => editingEmail = true}
          class="text-xs text-muted-foreground underline hover:text-foreground">
          Change Email
        </button>
      </div>
    {/if}
  </section>

  <!-- Data & Privacy section -->
  <section class="flex flex-col gap-4">
    <h2 class="text-base font-semibold">Data & Privacy</h2>

    <div class="flex flex-col gap-2">
      <p class="text-sm text-muted-foreground">
        Download a copy of all your personal data stored in this service.
      </p>
      <button
        type="button"
        onclick={downloadExport}
        class="self-start h-10 px-5 rounded-lg border border-border text-sm font-medium hover:bg-muted transition-colors"
      >
        Download My Data
      </button>
    </div>

    <div class="rounded-lg border border-border bg-muted/40 p-4 flex flex-col gap-1">
      <p class="text-sm font-medium">Delete My Account</p>
      <p class="text-sm text-muted-foreground">
        To request deletion of your account and all associated data, contact your admin.
        See the <a href="/privacy" class="underline hover:text-foreground">Privacy Policy</a> for details.
      </p>
    </div>
  </section>
</div>
