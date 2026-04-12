<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { uploadImageItem } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import { ArrowLeft, Upload, Trash2, ImageIcon, Type } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { GameItem } from '$lib/api/types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let items = $state<GameItem[]>(untrack(() => data.items));
  let uploading = $state(false);

  $effect(() => {
    if (form?.deleted) { items = items.filter((i) => i.id !== form.deleted); toast.show('Item deleted.', 'success'); }
    if (form?.deleteError) toast.show(form.deleteError, 'error');
  });

  async function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    if (files.length === 0) return;
    uploading = true;
    for (const file of files) {
      try {
        const item = await uploadImageItem(data.pack.id, file.name.replace(/\.[^.]+$/, ''), file);
        items = [...items, item];
      } catch {
        toast.show(`Failed to upload ${file.name}.`, 'error');
      }
    }
    uploading = false;
    input.value = '';
    toast.show(`${files.length} item(s) uploaded.`, 'success');
  }
</script>

<svelte:head>
  <title>{data.pack.name} — Admin</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-3">
    <a
      href="/admin/packs"
      use:hoverEffect={'swap'}
      class="inline-flex items-center gap-1 text-sm text-brand-text-muted hover:text-brand-text px-2 py-1 rounded-full"
    >
      <ArrowLeft size={14} strokeWidth={2.5} />
      Packs
    </a>
    <span class="text-brand-text-muted">/</span>
    <h1 class="text-xl font-bold">{data.pack.name}</h1>
    <span class="text-sm text-brand-text-muted ml-1">({items.length} items)</span>
    <div class="flex-1"></div>
    <label
      use:pressPhysics={'ghost'}
      use:hoverEffect={'swap'}
      class="h-9 px-4 rounded-lg border border-brand-border text-sm font-medium cursor-pointer flex items-center gap-1.5"
    >
      <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={handleFileInput} disabled={uploading} />
      <Upload size={14} strokeWidth={2.5} />
      {uploading ? 'Uploading…' : 'Add Items'}
    </label>
  </div>

  {#if data.pack.description}
    <p class="text-sm text-brand-text-muted">{data.pack.description}</p>
  {/if}

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="w-10 px-4 py-3">#</th>
          <th class="text-left px-4 py-3">Preview</th>
          <th class="text-left px-4 py-3">Name</th>
          <th class="text-left px-4 py-3">Type</th>
          <th class="text-left px-4 py-3">Version</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each items as item, i}
          <tr class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3 text-brand-text-muted text-xs">{i + 1}</td>
            <td class="px-4 py-3">
              {#if item.thumbnail_url}
                <img src={item.thumbnail_url} alt="" class="h-10 w-10 rounded object-cover" />
              {:else}
                <div class="h-10 w-10 rounded bg-muted flex items-center justify-center text-brand-text-muted">
                  {#if item.type === 'image'}
                    <ImageIcon size={16} strokeWidth={2.5} />
                  {:else}
                    <Type size={16} strokeWidth={2.5} />
                  {/if}
                </div>
              {/if}
            </td>
            <td class="px-4 py-3 font-medium">{item.name}</td>
            <td class="px-4 py-3">
              <span class="text-xs px-2 py-0.5 rounded-full bg-muted text-brand-text-muted">{item.type}</span>
            </td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">v{item.version_number ?? 1}</td>
            <td class="px-4 py-3 text-right">
              <form method="POST" action="?/deleteItem" use:enhance
                onsubmit={(e) => !confirm(`Delete "${item.name}"?`) && e.preventDefault()}>
                <input type="hidden" name="item_id" value={item.id} />
                <button
                  type="submit"
                  use:hoverEffect={'swap'}
                  class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                  aria-label="Delete item">
                  <Trash2 size={14} strokeWidth={2.5} />
                </button>
              </form>
            </td>
          </tr>
        {/each}
        {#if items.length === 0}
          <tr>
            <td colspan={6} class="px-4 py-8 text-center text-brand-text-muted text-sm">
              No items yet. Upload images to get started.
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
</div>
