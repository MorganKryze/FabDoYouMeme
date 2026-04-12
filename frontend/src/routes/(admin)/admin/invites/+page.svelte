<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let showCreateForm = $state(false);
  // Snapshot from load data; list is locally mutated by the effect below.
  let invites = $state(untrack(() => data.invites));
  let revealedTokens = $state<Set<string>>(new Set());

  $effect(() => {
    if (form?.created) {
      invites = [...invites, form.created];
      showCreateForm = false;
      toast.show('Invite created.', 'success');
    }
    if (form?.revoked) {
      invites = invites.filter((i: { id: string }) => i.id !== form.revoked);
      toast.show('Invite revoked.', 'success');
    }
    if (form?.createError || form?.revokeError) {
      toast.show(form.createError ?? form.revokeError, 'error');
    }
  });

  function copyLink(token: string) {
    const url = `${window.location.origin}/auth/register?invite=${token}`;
    navigator.clipboard.writeText(url).then(() => toast.show('Link copied to clipboard.', 'success'));
  }

  function toggleReveal(id: string) {
    const next = new Set(revealedTokens);
    if (next.has(id)) next.delete(id); else next.add(id);
    revealedTokens = next;
  }
</script>

<svelte:head>
  <title>Invites — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Invites</h1>
    <button type="button" onclick={() => showCreateForm = !showCreateForm}
      class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
      + Create Invite
    </button>
  </div>

  {#if showCreateForm}
    <form method="POST" action="?/createInvite" use:enhance
      class="rounded-xl border border-brand-border bg-brand-white p-4 flex flex-col gap-3">
      <h2 class="text-sm font-semibold">New Invite</h2>
      <div class="grid grid-cols-2 gap-3">
        <div class="flex flex-col gap-1">
          <label for="label" class="text-xs font-medium">Label</label>
          <input id="label" name="label" type="text" placeholder="Gaming night 2026"
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="restricted_email" class="text-xs font-medium">Restrict to email</label>
          <input id="restricted_email" name="restricted_email" type="email" placeholder="Optional"
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="max_uses" class="text-xs font-medium">Max uses (0 = unlimited)</label>
          <input id="max_uses" name="max_uses" type="number" min={0} value={0}
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
        <div class="flex flex-col gap-1">
          <label for="expires_at" class="text-xs font-medium">Expires at</label>
          <input id="expires_at" name="expires_at" type="datetime-local"
            class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
        </div>
      </div>
      <div class="flex gap-2 justify-end">
        <button type="button" onclick={() => showCreateForm = false}
          class="h-9 px-4 rounded border border-brand-border text-sm hover:bg-muted">Cancel</button>
        <button type="submit"
          class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
          Create
        </button>
      </div>
    </form>
  {/if}

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">Label</th>
          <th class="text-left px-4 py-3">Token</th>
          <th class="text-left px-4 py-3">Restricted Email</th>
          <th class="text-left px-4 py-3">Uses</th>
          <th class="text-left px-4 py-3">Expires</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each invites as inv}
          <tr class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">{inv.label ?? '—'}</td>
            <td class="px-4 py-3 font-mono">
              <button type="button" onclick={() => toggleReveal(inv.id)}
                class="text-brand-text-muted hover:text-brand-text transition-colors">
                {revealedTokens.has(inv.id) ? inv.token : `${inv.token.slice(0, 4)}…`}
              </button>
            </td>
            <td class="px-4 py-3 text-brand-text-muted">{inv.restricted_email ?? '—'}</td>
            <td class="px-4 py-3 text-brand-text-muted">
              {inv.uses_count}/{inv.max_uses === 0 ? '∞' : inv.max_uses}
            </td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">
              {inv.expires_at ? new Date(inv.expires_at).toLocaleDateString() : 'Never'}
            </td>
            <td class="px-4 py-3">
              <div class="flex gap-2 justify-end">
                <button type="button" onclick={() => copyLink(inv.token)}
                  class="text-xs text-brand-text-muted underline hover:text-brand-text">
                  Copy Link
                </button>
                <form method="POST" action="?/revokeInvite" use:enhance
                  onsubmit={(e) => !confirm('Revoke this invite?') && e.preventDefault()}>
                  <input type="hidden" name="invite_id" value={inv.id} />
                  <button type="submit" class="text-xs text-red-600 underline hover:text-red-800">
                    Revoke
                  </button>
                </form>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
