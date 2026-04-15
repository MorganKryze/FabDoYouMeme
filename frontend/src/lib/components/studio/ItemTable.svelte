<!-- frontend/src/lib/components/studio/ItemTable.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import {
    deleteItem,
    bulkUploadImageItems,
    validateImageFile,
    listVersions
  } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Upload, Trash2, ImageIcon } from '$lib/icons';
  import type { GameItem } from '$lib/api/types';

  let dragOverZone = $state(false);
  let uploading = $state(false);
  let uploadProgress = $state<{ name: string; done: number; total: number } | null>(null);

  const isSystem = $derived(
    studio.packs.find((p) => p.id === studio.selectedPackId)?.is_system ?? false
  );

  async function selectItem(item: GameItem) {
    studio.selectItem(item.id);
    studio.versions = await listVersions(studio.selectedPackId!, item.id).catch(() => []);
  }

  async function handleDelete(item: GameItem) {
    if (!confirm(`Delete "${item.name}"? This cannot be undone.`)) return;
    try {
      await deleteItem(studio.selectedPackId!, item.id);
      studio.items = studio.items.filter((i) => i.id !== item.id);
      if (studio.selectedItemId === item.id) {
        studio.selectedItemId = null;
        studio.versions = [];
        studio.selectedVersionIds = [];
      }
    } catch {
      toast.show('Failed to delete item.', 'error');
    }
  }

  async function bulkUpload(files: File[]) {
    // Pre-filter so rejected files appear in the same summary as network failures.
    const rejected: { filename: string; reason: string }[] = [];
    const accepted: File[] = [];
    for (const f of files) {
      const err = validateImageFile(f);
      if (err) rejected.push({ filename: f.name, reason: err });
      else accepted.push(f);
    }

    if (accepted.length === 0 && rejected.length === 0) return;

    uploading = true;
    const result = await bulkUploadImageItems(
      studio.selectedPackId!,
      accepted,
      (done, total, name) => {
        uploadProgress = { name, done, total };
      }
    );
    uploadProgress = null;
    uploading = false;

    studio.items = [...studio.items, ...result.succeeded];

    const failed = [...rejected, ...result.failed];
    const ok = result.succeeded.length;
    const ko = failed.length;
    if (ok > 0 && ko === 0) {
      toast.show(`${ok} item${ok === 1 ? '' : 's'} uploaded.`, 'success');
    } else if (ok > 0 && ko > 0) {
      toast.show(`${ok} uploaded, ${ko} failed.`, 'warning');
    } else {
      toast.show(`Upload failed (${ko}/${ko}).`, 'error');
    }
  }

  function onDropZone(e: DragEvent) {
    e.preventDefault();
    dragOverZone = false;
    if (isSystem) return;
    const files = Array.from(e.dataTransfer?.files ?? []);
    void bulkUpload(files);
  }

  function onFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    void bulkUpload(files);
    input.value = '';
  }
</script>

<div class="flex flex-col h-full">
  <!-- Header -->
  <div class="flex items-center gap-3 px-4 py-3 border-b border-brand-border shrink-0">
    <h2 class="text-sm font-semibold flex-1">
      {studio.packs.find((p) => p.id === studio.selectedPackId)?.name ?? 'Items'}
      <span class="text-brand-text-muted font-normal">
        {studio.items.length === 0 ? 'empty' : `(${studio.items.length})`}
      </span>
    </h2>

    {#if isSystem}
      <span class="text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted border border-brand-border rounded-full px-2 py-1" title="Bundled system pack — managed on the server filesystem">
        Read-only
      </span>
    {:else}
      <label
        use:pressPhysics={'ghost'}
        class="h-8 px-3 rounded-md border border-brand-border text-xs font-medium cursor-pointer flex items-center gap-1.5"
      >
        <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={onFileInput} />
        <Upload size={12} strokeWidth={2.5} />
        Bulk Import
      </label>
    {/if}
  </div>

  <!-- Drop zone overlay -->
  <div
    class="flex-1 overflow-y-auto relative"
    ondragover={(e) => { e.preventDefault(); dragOverZone = true; }}
    ondragleave={() => dragOverZone = false}
    ondrop={onDropZone}
    role="region"
    aria-label="Item list — drag images here to import"
  >
    {#if dragOverZone}
      <div class="absolute inset-0 z-10 border-2 border-dashed border-primary bg-primary/5 flex items-center justify-center">
        <p class="text-primary font-medium">Drop images to import</p>
      </div>
    {/if}

    {#if uploading && uploadProgress}
      <div class="px-4 py-2 bg-muted/50 text-xs text-brand-text-muted border-b border-brand-border">
        Uploading {uploadProgress.name}… ({uploadProgress.done + 1}/{uploadProgress.total})
      </div>
    {/if}

    <!-- Item table -->
    {#if studio.items.length === 0}
      <div class="flex flex-col items-center justify-center h-48 text-brand-text-muted text-sm gap-2">
        <p>No items yet.</p>
        <p class="text-xs">Drag images here or use Bulk Import.</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-brand-border text-xs text-brand-text-muted font-medium">
            <th class="text-left px-4 py-2">Name</th>
            <th class="text-right px-4 py-2">Ver.</th>
            <th class="px-4 py-2"></th>
          </tr>
        </thead>
        <tbody>
          {#each studio.items as item, i}
            <tr
              use:reveal={{ delay: i }}
              class="border-b border-brand-border/50 hover:bg-muted/40 cursor-pointer transition-colors
                {studio.selectedItemId === item.id ? 'bg-primary/5' : ''}"
              onclick={() => selectItem(item)}
            >
              <td class="px-4 py-2">
                <div class="flex items-center gap-2">
                  {#if item.thumbnail_url}
                    <img src={item.thumbnail_url} alt="" class="h-8 w-8 rounded object-cover shrink-0" />
                  {:else}
                    <div class="h-8 w-8 rounded bg-muted shrink-0 flex items-center justify-center text-brand-text-muted">
                      <ImageIcon size={14} strokeWidth={2.5} />
                    </div>
                  {/if}
                  <span class="truncate max-w-[8rem]">{item.name}</span>
                </div>
              </td>
              <td class="px-4 py-2 text-right text-brand-text-muted">v{item.version_number ?? 1}</td>
              <td class="px-4 py-2 text-right">
                {#if !isSystem}
                  <button
                    type="button"
                    onclick={(e) => { e.stopPropagation(); handleDelete(item); }}
                    class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                    aria-label="Delete item"
                  >
                    <Trash2 size={14} strokeWidth={2.5} />
                  </button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>
