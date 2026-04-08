<script lang="ts">
  import { enhance } from '$app/forms';
  import { toast } from '$lib/state/toast.svelte';
  import type { ActionData, PageData } from './$types';
  import type { Pack } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let packs = $state<Pack[]>(data.packs);
  let showNewRow = $state(false);

  $effect(() => {
    if (form?.created) { packs = [...packs, form.created]; showNewRow = false; toast.show('Pack created.', 'success'); }
    if (form?.deleted) { packs = packs.filter((p) => p.id !== form.deleted); toast.show('Pack deleted.', 'success'); }
    if (form?.createError || form?.deleteError) toast.show(form.createError ?? form.deleteError, 'error');
  });
</script>

<svelte:head>
  <title>Packs — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4">
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">Packs</h1>
    <button type="button" onclick={() => showNewRow = !showNewRow}
      class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90">
      + New Pack
    </button>
  </div>

  {#if showNewRow}
    <form method="POST" action="?/createPack" use:enhance
      class="flex gap-3 items-end rounded-lg border border-dashed border-border p-3">
      <div class="flex flex-col gap-1 flex-1">
        <label class="text-xs font-medium">Name</label>
        <input name="name" type="text" required autofocus placeholder="Pack name"
          class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <div class="flex flex-col gap-1 flex-1">
        <label class="text-xs font-medium">Description</label>
        <input name="description" type="text" placeholder="Optional"
          class="h-9 rounded border border-input bg-background px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <button type="submit"
        class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 shrink-0">
        Create
      </button>
      <button type="button" onclick={() => showNewRow = false}
        class="h-9 px-4 rounded border border-border text-sm hover:bg-muted shrink-0">
        Cancel
      </button>
    </form>
  {/if}

  <div class="rounded-xl border border-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-border bg-muted/40 text-xs font-medium text-muted-foreground">
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Description</th>
          <th class="text-left px-4 py-3">Items</th>
          <th class="text-left px-4 py-3">Status</th>
          <th class="text-left px-4 py-3">Created</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each packs as pack}
          <tr class="border-b border-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <a href="/admin/packs/{pack.id}" class="font-medium hover:underline">{pack.name}</a>
            </td>
            <td class="px-4 py-3 text-muted-foreground">{pack.description ?? '—'}</td>
            <td class="px-4 py-3 text-muted-foreground">{pack.item_count ?? 0}</td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full
                {pack.status === 'active' ? 'bg-green-100 text-green-800' :
                 pack.status === 'flagged' ? 'bg-yellow-100 text-yellow-800' :
                 'bg-red-100 text-red-800'}">
                {pack.status}
              </span>
            </td>
            <td class="px-4 py-3 text-muted-foreground text-xs">
              {new Date(pack.created_at).toLocaleDateString()}
            </td>
            <td class="px-4 py-3 text-right">
              <form method="POST" action="?/deletePack" use:enhance
                onsubmit={(e) => {
                  if (!confirm('Delete this pack? In-use packs will no longer be available for new rooms.')) e.preventDefault();
                }}>
                <input type="hidden" name="pack_id" value={pack.id} />
                <button type="submit"
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete pack">
                  ×
                </button>
              </form>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
