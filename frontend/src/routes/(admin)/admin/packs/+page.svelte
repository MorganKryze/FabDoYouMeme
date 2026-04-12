<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { Plus, Trash2, XCircle } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { Pack } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let packs = $state<Pack[]>(untrack(() => data.packs));
  let showNewRow = $state(false);
  // Imperative focus replaces `autofocus` (a11y_autofocus): when the inline
  // form opens, move focus once into the Name input so keyboard/screen-reader
  // users follow the context change. Finding 1.2 in the 2026-04-10 review.
  let newPackNameInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (showNewRow && newPackNameInput) newPackNameInput.focus();
  });

  $effect(() => {
    if (form?.created) { packs = [...packs, form.created]; showNewRow = false; toast.show('Pack created.', 'success'); }
    if (form?.deleted) { packs = packs.filter((p) => p.id !== form.deleted); toast.show('Pack deleted.', 'success'); }
    if (form?.createError || form?.deleteError) toast.show(form.createError ?? form.deleteError, 'error');
  });
</script>

<svelte:head>
  <title>Packs — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Packs</h1>
    <button
      type="button"
      onclick={() => showNewRow = !showNewRow}
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium inline-flex items-center gap-1.5">
      <Plus size={14} strokeWidth={2.5} />
      New Pack
    </button>
  </div>

  {#if showNewRow}
    <form method="POST" action="?/createPack" use:enhance
      class="flex gap-3 items-end rounded-lg border border-dashed border-brand-border p-3">
      <div class="flex flex-col gap-1 flex-1">
        <label for="new-pack-name" class="text-xs font-medium">Name</label>
        <input id="new-pack-name" name="name" type="text" required
          bind:this={newPackNameInput}
          placeholder="Pack name"
          class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <div class="flex flex-col gap-1 flex-1">
        <label for="new-pack-description" class="text-xs font-medium">Description</label>
        <input id="new-pack-description" name="description" type="text" placeholder="Optional"
          class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <button
        type="submit"
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium shrink-0 inline-flex items-center gap-1.5">
        <Plus size={14} strokeWidth={2.5} />
        Create
      </button>
      <button
        type="button"
        onclick={() => showNewRow = false}
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded border border-brand-border text-sm shrink-0 inline-flex items-center gap-1.5">
        <XCircle size={14} strokeWidth={2.5} />
        Cancel
      </button>
    </form>
  {/if}

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Description</th>
          <th class="text-left px-4 py-3">Items</th>
          <th class="text-left px-4 py-3">Status</th>
          <th class="text-left px-4 py-3">Created</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each packs as pack, i}
          <tr use:reveal={{ delay: i }} class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <a href="/admin/packs/{pack.id}" class="font-medium hover:underline">{pack.name}</a>
            </td>
            <td class="px-4 py-3 text-brand-text-muted">{pack.description ?? '—'}</td>
            <td class="px-4 py-3 text-brand-text-muted">{pack.item_count ?? 0}</td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full
                {pack.status === 'active' ? 'bg-green-100 text-green-800' :
                 pack.status === 'flagged' ? 'bg-yellow-100 text-yellow-800' :
                 'bg-red-100 text-red-800'}">
                {pack.status}
              </span>
            </td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">
              {new Date(pack.created_at).toLocaleDateString()}
            </td>
            <td class="px-4 py-3 text-right">
              <form method="POST" action="?/deletePack" use:enhance
                onsubmit={(e) => {
                  if (!confirm('Delete this pack? In-use packs will no longer be available for new rooms.')) e.preventDefault();
                }}>
                <input type="hidden" name="pack_id" value={pack.id} />
                <button
                  type="submit"
                  use:hoverEffect={'swap'}
                  class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                  aria-label="Delete pack">
                  <Trash2 size={14} strokeWidth={2.5} />
                </button>
              </form>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
