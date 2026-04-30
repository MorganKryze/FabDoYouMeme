<!-- frontend/src/lib/components/studio/ItemTable.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import {
    deleteItem,
    bulkUploadImageItems,
    bulkUploadTextItems,
    parseTextItemsJson,
    validateImageFile,
    listVersions
  } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Upload, Trash2, ImageIcon, FileText } from '$lib/icons';
  import type { GameItem } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';

  let dragOverZone = $state(false);
  let uploading = $state(false);
  let uploadProgress = $state<{ name: string; done: number; total: number } | null>(null);

  import { user } from '$lib/state/user.svelte';

  const selectedPack = $derived(
    studio.packs.find((p) => p.id === studio.selectedPackId) ?? null
  );
  const isSystem = $derived(selectedPack?.is_system ?? false);
  const kind = $derived(studio.kindFor(studio.selectedPackId));

  // Per-spec: deleting an item from a group pack is admin-only. Regular
  // members can still modify items (see uploads / version promote). Hide
  // the trash button entirely when the caller can't perform the action —
  // otherwise they'd click it and hit a 403 with no guidance.
  const canDeleteItems = $derived.by(() => {
    if (!selectedPack) return false;
    if (selectedPack.is_system) return false;
    if (user.role === 'admin') return true;
    if (selectedPack.group_id) {
      return (
        studio.groups.find((g) => g.id === selectedPack.group_id)?.member_role === 'admin'
      );
    }
    // Personal pack — only the owner can delete items. Public packs owned
    // by others fall through to false.
    return selectedPack.owner_id === user.id;
  });

  function textSnippet(item: GameItem): string {
    const payload = item.payload;
    if (!payload || typeof payload !== 'object') return '';
    // Text/filler items (payload v2/v3) carry a single `text` field.
    if ('text' in payload) {
      const t = (payload as { text: unknown }).text;
      if (typeof t === 'string') return t;
    }
    // Prompt items (payload v4) carry `prefix` + `suffix`; render them with
    // an underline placeholder so the table row reads as the live sentence.
    if ('prefix' in payload || 'suffix' in payload) {
      const p = (payload as { prefix?: unknown; suffix?: unknown });
      const prefix = typeof p.prefix === 'string' ? p.prefix : '';
      const suffix = typeof p.suffix === 'string' ? p.suffix : '';
      const joiner = '___';
      const parts = [prefix.trim(), joiner, suffix.trim()].filter((s) => s.length > 0);
      return parts.join(' ').trim();
    }
    return '';
  }

  async function selectItem(item: GameItem) {
    studio.selectItem(item.id);
    studio.versions = await listVersions(studio.selectedPackId!, item.id).catch(() => []);
  }

  async function handleDelete(item: GameItem) {
    if (!confirm(m.studio_confirm_delete_item({ name: item.name }))) return;
    try {
      await deleteItem(studio.selectedPackId!, item.id);
      studio.items = studio.items.filter((i) => i.id !== item.id);
      if (studio.selectedItemId === item.id) {
        studio.selectedItemId = null;
        studio.versions = [];
        studio.selectedVersionIds = [];
      }
    } catch {
      toast.show(m.studio_toast_item_delete_failed(), 'error');
    }
  }

  async function bulkUploadImages(files: File[]) {
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
    summarize(result.succeeded.length, [...rejected, ...result.failed].length);
  }

  async function bulkImportTextJson(file: File) {
    const raw = await file.text().catch(() => null);
    if (raw === null) {
      toast.show(m.studio_toast_read_file_failed(), 'error');
      return;
    }
    const parsed = parseTextItemsJson(raw);
    if (!parsed.ok) {
      toast.show(parsed.error, 'error');
      return;
    }
    if (parsed.items.length === 0) {
      toast.show(m.studio_toast_json_empty(), 'warning');
      return;
    }

    uploading = true;
    const result = await bulkUploadTextItems(
      studio.selectedPackId!,
      parsed.items,
      (done, total, name) => {
        uploadProgress = { name, done, total };
      }
    );
    uploadProgress = null;
    uploading = false;

    studio.items = [...studio.items, ...result.succeeded];
    summarize(result.succeeded.length, result.failed.length);
  }

  function summarize(ok: number, ko: number) {
    if (ok > 0 && ko === 0) {
      toast.show(
        ok === 1
          ? m.studio_toast_items_added_one({ count: ok })
          : m.studio_toast_items_added_other({ count: ok }),
        'success'
      );
    } else if (ok > 0 && ko > 0) {
      toast.show(m.studio_toast_items_partial({ ok, ko }), 'warning');
    } else if (ko > 0) {
      toast.show(
        ko === 1
          ? m.studio_toast_items_all_failed_one({ count: ko })
          : m.studio_toast_items_all_failed_other({ count: ko }),
        'error'
      );
    }
  }

  function onDropZone(e: DragEvent) {
    e.preventDefault();
    dragOverZone = false;
    if (isSystem) return;
    const files = Array.from(e.dataTransfer?.files ?? []);
    if (files.length === 0) return;
    if (kind === 'image') void bulkUploadImages(files);
    else void bulkImportTextJson(files[0]);
  }

  function onImageFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    void bulkUploadImages(files);
    input.value = '';
  }

  function onTextFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (file) void bulkImportTextJson(file);
    input.value = '';
  }
</script>

<div class="flex flex-col h-full">
  <!-- Header -->
  <div class="flex items-center gap-3 px-4 py-3 border-b border-brand-border shrink-0">
    <h2 class="text-sm font-semibold flex-1">
      {studio.packs.find((p) => p.id === studio.selectedPackId)?.name ?? m.studio_items_fallback_heading()}
      <span class="text-brand-text-muted font-normal">
        {studio.items.length === 0 ? m.studio_items_count_empty() : m.studio_items_count({ count: studio.items.length })}
      </span>
    </h2>

    {#if isSystem}
      <span class="text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted border border-brand-border rounded-full px-2 py-1" title={m.studio_items_readonly_title()}>
        {m.studio_items_readonly_badge()}
      </span>
    {:else if kind === 'image'}
      <label
        use:pressPhysics={'ghost'}
        class="h-8 px-3 rounded-md border border-brand-border text-xs font-medium cursor-pointer flex items-center gap-1.5"
      >
        <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={onImageFileInput} />
        <Upload size={12} strokeWidth={2.5} />
        {m.studio_items_bulk_import()}
      </label>
    {:else}
      <label
        use:pressPhysics={'ghost'}
        title={m.studio_items_import_json_title()}
        class="h-8 px-3 rounded-md border border-brand-border text-xs font-medium cursor-pointer flex items-center gap-1.5"
      >
        <input type="file" accept="application/json,.json" class="sr-only" onchange={onTextFileInput} />
        <Upload size={12} strokeWidth={2.5} />
        {m.studio_items_import_json()}
      </label>
    {/if}
  </div>

  <!-- Drop zone overlay (images for image packs, JSON for text packs) -->
  <div
    class="flex-1 overflow-y-auto relative"
    ondragover={(e) => {
      if (isSystem) return;
      e.preventDefault();
      dragOverZone = true;
    }}
    ondragleave={() => dragOverZone = false}
    ondrop={onDropZone}
    role="region"
    aria-label={kind === 'image' ? m.studio_items_drop_region_aria_image() : m.studio_items_drop_region_aria_text()}
  >
    {#if dragOverZone && !isSystem}
      <div class="absolute inset-0 z-10 border-2 border-dashed border-primary bg-primary/5 flex items-center justify-center">
        <p class="text-primary font-medium">
          {kind === 'image' ? m.studio_items_drop_label_image() : m.studio_items_drop_label_text()}
        </p>
      </div>
    {/if}

    {#if uploading && uploadProgress}
      <div class="px-4 py-2 bg-muted/50 text-xs text-brand-text-muted border-b border-brand-border">
        {m.studio_items_uploading_progress({ name: uploadProgress.name, done: uploadProgress.done + 1, total: uploadProgress.total })}
      </div>
    {/if}

    <!-- Item table -->
    {#if studio.items.length === 0}
      <div class="flex flex-col items-center justify-center h-48 text-brand-text-muted text-sm gap-2">
        <p>{m.studio_items_empty_title()}</p>
        {#if kind === 'image'}
          <p class="text-xs">{m.studio_items_empty_hint_image()}</p>
        {:else}
          <p class="text-xs">{m.studio_items_empty_hint_text_prefix()}<code class="text-[11px]">[{`{"name":"…","text":"…"}`}, …]</code></p>
        {/if}
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-brand-border text-xs text-brand-text-muted font-medium">
            <th class="text-left px-4 py-2">{m.studio_items_col_name()}</th>
            <th class="text-right px-4 py-2">{m.studio_items_col_version()}</th>
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
                <div class="flex items-center gap-2 min-w-0">
                  {#if item.payload_version === 2 || item.payload_version === 3 || item.payload_version === 4}
                    <div class="h-8 w-8 rounded bg-muted shrink-0 flex items-center justify-center text-brand-text-muted">
                      <FileText size={14} strokeWidth={2.5} />
                    </div>
                    <div class="flex flex-col min-w-0">
                      <span class="truncate max-w-[12rem] text-sm">{item.name}</span>
                      {#if textSnippet(item)}
                        <span class="truncate max-w-[12rem] text-[11px] text-brand-text-muted">{textSnippet(item)}</span>
                      {/if}
                    </div>
                  {:else if item.thumbnail_url}
                    <img src={item.thumbnail_url} alt="" class="h-8 w-8 rounded object-cover shrink-0" />
                    <span class="truncate max-w-[8rem]">{item.name}</span>
                  {:else}
                    <div class="h-8 w-8 rounded bg-muted shrink-0 flex items-center justify-center text-brand-text-muted">
                      <ImageIcon size={14} strokeWidth={2.5} />
                    </div>
                    <span class="truncate max-w-[8rem]">{item.name}</span>
                  {/if}
                </div>
              </td>
              <td class="px-4 py-2 text-right text-brand-text-muted">v{item.version_number ?? 1}</td>
              <td class="px-4 py-2 text-right">
                {#if canDeleteItems}
                  <button
                    type="button"
                    onclick={(e) => { e.stopPropagation(); handleDelete(item); }}
                    class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                    aria-label={m.studio_items_delete_aria()}
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
