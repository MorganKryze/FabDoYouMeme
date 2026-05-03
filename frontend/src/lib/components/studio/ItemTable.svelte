<!-- frontend/src/lib/components/studio/ItemTable.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { gameTypes } from '$lib/state/game-types.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import {
    deleteItem,
    bulkUploadImageItems,
    bulkUploadTextItems,
    parseTextItemsJson,
    validateImageFile,
    listVersions,
    BULK_ABORTED_REASON
  } from '$lib/api/studio';
  import { compressImage } from '$lib/api/imageCompress';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Upload, Trash2, ImageIcon, FileText } from '$lib/icons';
  import type { GameItem } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';
  import BulkUploadProgress, { type BulkUploadEntry } from './BulkUploadProgress.svelte';

  let dragOverZone = $state(false);
  let uploading = $state(false);
  // Per-item live status for the bulk import. Pre-populated with `pending`
  // entries before the upload starts; mutated in place as the studio API
  // emits onItemDone events. Survives upload completion until the user
  // dismisses the panel — that's how they review the 30-failure list.
  let bulkEntries = $state<BulkUploadEntry[]>([]);
  let bulkPanelOpen = $state(false);
  // AbortController for the in-flight import. Reset for each new run; the
  // panel's Stop button calls .abort(), which short-circuits the loop in
  // bulkUploadImageItems / bulkUploadTextItems and marks every remaining
  // entry as cancelled.
  let bulkAborter: AbortController | null = null;

  function mapEvent(event: { ok: boolean; reason?: string; filename: string }): BulkUploadEntry {
    if (event.ok) return { filename: event.filename, status: 'success' };
    if (event.reason === BULK_ABORTED_REASON) return { filename: event.filename, status: 'cancelled' };
    return { filename: event.filename, status: 'failed', reason: event.reason };
  }

  function abortBulk() {
    bulkAborter?.abort();
  }

  import { user } from '$lib/state/user.svelte';

  const selectedPack = $derived(
    studio.packs.find((p) => p.id === studio.selectedPackId) ?? null
  );
  const isSystem = $derived(selectedPack?.is_system ?? false);
  const kind = $derived(studio.kindFor(studio.selectedPackId));

  // Worst-case items the largest compatible game type would consume from a
  // pack of this kind (image/text/filler/prompt). 0 until the registry has
  // loaded — the template hides the badge in that case.
  const worstCaseNeeded = $derived(gameTypes.worstCaseItemsNeeded(kind));

  // Status palette for the capacity pill: green when the pack already
  // covers the worst case, amber while it's still being filled, muted when
  // empty. Green = "ready for any compatible room"; amber = "fine for a
  // smaller cap, short of a full lobby".
  const capacityStatus = $derived.by<'met' | 'partial' | 'empty'>(() => {
    if (studio.items.length === 0) return 'empty';
    if (studio.items.length >= worstCaseNeeded) return 'met';
    return 'partial';
  });

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
    // Three-bucket pre-filter (in display order in the panel):
    //   rejected: failed client-side validation (MIME / >10 MiB)
    //   skipped:  filename already exists in the pack — re-dropping the
    //             same folder thus retries only what's missing
    //   accepted: actually goes to the network
    const rejected: { filename: string; reason: string }[] = [];
    const skipped: { filename: string }[] = [];
    const accepted: File[] = [];
    const existingNames = new Set(studio.items.map((i) => i.name));
    const stripExt = (n: string) => n.replace(/\.[^.]+$/, '');
    for (const f of files) {
      const err = validateImageFile(f);
      if (err) { rejected.push({ filename: f.name, reason: err }); continue; }
      if (existingNames.has(stripExt(f.name))) { skipped.push({ filename: f.name }); continue; }
      accepted.push(f);
    }

    if (accepted.length === 0 && rejected.length === 0 && skipped.length === 0) return;

    // Pre-populate the panel with one row per file. Rejected and skipped
    // rows are terminal and rendered immediately; accepted rows flip in
    // place to success/failed via onItemDone. Index math for accepted in
    // bulkEntries is: rejected.length + skipped.length + i.
    bulkEntries = [
      ...rejected.map<BulkUploadEntry>((r) => ({ filename: r.filename, status: 'failed', reason: r.reason })),
      ...skipped.map<BulkUploadEntry>((s) => ({ filename: s.filename, status: 'skipped' })),
      ...accepted.map<BulkUploadEntry>((f) => ({ filename: f.name, status: 'pending' }))
    ];
    bulkPanelOpen = true;
    if (accepted.length === 0) return; // nothing to upload — panel reflects the result
    bulkAborter = new AbortController();
    uploading = true;
    const offset = rejected.length + skipped.length;
    const result = await bulkUploadImageItems(
      studio.selectedPackId!,
      accepted,
      undefined,
      (event) => {
        const idx = offset + event.index;
        const next = bulkEntries.slice();
        next[idx] = mapEvent(event);
        bulkEntries = next;
      },
      bulkAborter.signal,
      // Best-effort client-side compression. 256 KiB target keeps the body
      // well under any conceivable upstream proxy cap (the previous 800 KiB
      // target still tripped a 500 from the user's Pangolin/Traefik
      // ingress, so we shrank it further). 1920 px long edge stays above
      // any realistic display res while shaving 80-95 % of the bytes.
      // Console-debug the before/after so a bug report includes concrete
      // numbers — when an image still fails after compression, the dev
      // tools console proves whether the prepare step ran and what size
      // hit the wire.
      async (file) => {
        const compressed = await compressImage(file, { maxBytes: 256 * 1024, maxDimension: 1920 });
        console.debug(`[bulk import] ${file.name}: ${file.size} → ${compressed.size} bytes`);
        return compressed;
      }
    );
    uploading = false;
    bulkAborter = null;

    studio.items = [...studio.items, ...result.succeeded];
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

    bulkEntries = parsed.items.map<BulkUploadEntry>((it) => ({ filename: it.name, status: 'pending' }));
    bulkPanelOpen = true;
    bulkAborter = new AbortController();
    uploading = true;
    const result = await bulkUploadTextItems(
      studio.selectedPackId!,
      parsed.items,
      undefined,
      (event) => {
        const next = bulkEntries.slice();
        next[event.index] = mapEvent(event);
        bulkEntries = next;
      },
      bulkAborter.signal
    );
    uploading = false;
    bulkAborter = null;

    studio.items = [...studio.items, ...result.succeeded];
  }

  function closeBulkPanel() {
    if (uploading) return;
    bulkPanelOpen = false;
    bulkEntries = [];
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
      {#if worstCaseNeeded > 0}
        <span
          class="ml-2 inline-flex items-center text-[10px] font-semibold uppercase tracking-wider rounded-full px-2 py-0.5 border cursor-help
            {capacityStatus === 'met'
              ? 'text-emerald-700 border-emerald-500 bg-emerald-50'
              : capacityStatus === 'partial'
                ? 'text-amber-700 border-amber-500 bg-amber-50'
                : 'text-brand-text-muted border-brand-border bg-transparent'}"
          title={capacityStatus === 'met'
            ? m.studio_items_capacity_title_met({ need: worstCaseNeeded })
            : m.studio_items_capacity_title_partial({ have: studio.items.length, need: worstCaseNeeded })}
        >
          {m.studio_items_capacity_short({ have: studio.items.length, need: worstCaseNeeded })}
        </span>
      {/if}
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

    <!-- Inline running progress is intentionally absent: the floating
         BulkUploadProgress panel below handles both the running counter
         and the final per-file outcome list, so a second indicator here
         would just be noise. -->

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
          <tr class="border-b border-brand-border text-[11px] text-brand-text-muted font-medium">
            <th class="text-left px-3 py-1.5">{m.studio_items_col_name()}</th>
            <th class="text-right px-3 py-1.5">{m.studio_items_col_version()}</th>
            <th class="px-3 py-1.5"></th>
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
              <td class="px-3 py-1">
                <div class="flex items-center gap-2 min-w-0">
                  {#if item.payload_version === 2 || item.payload_version === 3 || item.payload_version === 4}
                    <div class="h-6 w-6 rounded bg-muted shrink-0 flex items-center justify-center text-brand-text-muted">
                      <FileText size={12} strokeWidth={2.5} />
                    </div>
                    <div class="flex flex-col min-w-0 leading-tight">
                      <span class="truncate text-[13px]">{item.name}</span>
                      {#if textSnippet(item)}
                        <span class="truncate text-[11px] text-brand-text-muted">{textSnippet(item)}</span>
                      {/if}
                    </div>
                  {:else if item.thumbnail_url}
                    <img src={item.thumbnail_url} alt="" class="h-6 w-6 rounded object-cover shrink-0" />
                    <span class="truncate text-[13px]">{item.name}</span>
                  {:else}
                    <div class="h-6 w-6 rounded bg-muted shrink-0 flex items-center justify-center text-brand-text-muted">
                      <ImageIcon size={12} strokeWidth={2.5} />
                    </div>
                    <span class="truncate text-[13px]">{item.name}</span>
                  {/if}
                </div>
              </td>
              <td class="px-3 py-1 text-right text-[11px] text-brand-text-muted">v{item.version_number ?? 1}</td>
              <td class="px-3 py-1 text-right">
                {#if canDeleteItems}
                  <button
                    type="button"
                    onclick={(e) => { e.stopPropagation(); handleDelete(item); }}
                    class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-0.5 rounded-full"
                    aria-label={m.studio_items_delete_aria()}
                  >
                    <Trash2 size={12} strokeWidth={2.5} />
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

{#if bulkPanelOpen}
  <BulkUploadProgress entries={bulkEntries} running={uploading} onClose={closeBulkPanel} onAbort={abortBulk} />
{/if}
