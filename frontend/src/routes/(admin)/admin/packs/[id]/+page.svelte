<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { bulkUploadImageItems, validateImageFile, BULK_ABORTED_REASON } from '$lib/api/studio';
  import { compressImage } from '$lib/api/imageCompress';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { hoverEffect } from '$lib/actions/hoverEffect';
  import {
    ArrowLeft,
    Upload,
    Trash2,
    ImageIcon,
    Gavel,
    Flag,
    Ban,
    CheckCircle,
    ChevronDown
  } from '$lib/icons';
  import type { ActionData, PageData } from './$types';
  import type { GameItem } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';
  import BulkUploadProgress, { type BulkUploadEntry } from '$lib/components/studio/BulkUploadProgress.svelte';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let items = $state<GameItem[]>(untrack(() => data.items));
  let uploading = $state(false);
  let modMenuOpen = $state(false);
  let bulkEntries = $state<BulkUploadEntry[]>([]);
  let bulkPanelOpen = $state(false);
  let bulkAborter: AbortController | null = null;
  function closeBulkPanel() {
    if (uploading) return;
    bulkPanelOpen = false;
    bulkEntries = [];
  }
  function abortBulk() {
    bulkAborter?.abort();
  }
  function mapEvent(event: { ok: boolean; reason?: string; filename: string }): BulkUploadEntry {
    if (event.ok) return { filename: event.filename, status: 'success' };
    if (event.reason === BULK_ABORTED_REASON) return { filename: event.filename, status: 'cancelled' };
    return { filename: event.filename, status: 'failed', reason: event.reason };
  }

  // `use:enhance` updates the `form` prop several times per submission
  // (pending → result → post-invalidate refetch), each update firing the
  // effect. Without this guard we got 3× toasts and the menu snapping shut
  // mid-reopen. A plain `let` (not `$state`) skips reactivity, so writing
  // `lastForm = form` from inside the effect is safe.
  let lastForm: ActionData | undefined;
  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.deleted) { items = items.filter((i) => i.id !== form.deleted); toast.show(m.admin_pack_detail_toast_deleted(), 'success'); }
    if (form?.deleteError) toast.show(form.deleteError, 'error');
    if (form?.statusUpdated) { toast.show(m.admin_pack_detail_toast_status_updated({ status: form.statusUpdated }), 'success'); modMenuOpen = false; }
    if (form?.statusError) toast.show(form.statusError, 'error');
  });

  async function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length === 0) return;

    const rejected: { filename: string; reason: string }[] = [];
    const skipped: { filename: string }[] = [];
    const accepted: File[] = [];
    const existingNames = new Set(items.map((i) => i.name));
    const stripExt = (n: string) => n.replace(/\.[^.]+$/, '');
    for (const f of files) {
      const err = validateImageFile(f);
      if (err) { rejected.push({ filename: f.name, reason: err }); continue; }
      if (existingNames.has(stripExt(f.name))) { skipped.push({ filename: f.name }); continue; }
      accepted.push(f);
    }

    bulkEntries = [
      ...rejected.map<BulkUploadEntry>((r) => ({ filename: r.filename, status: 'failed', reason: r.reason })),
      ...skipped.map<BulkUploadEntry>((s) => ({ filename: s.filename, status: 'skipped' })),
      ...accepted.map<BulkUploadEntry>((f) => ({ filename: f.name, status: 'pending' }))
    ];
    bulkPanelOpen = true;
    if (accepted.length === 0) return;
    bulkAborter = new AbortController();
    uploading = true;
    const offset = rejected.length + skipped.length;
    const result = await bulkUploadImageItems(
      data.pack.id,
      accepted,
      undefined,
      (event) => {
        const idx = offset + event.index;
        const next = bulkEntries.slice();
        next[idx] = mapEvent(event);
        bulkEntries = next;
      },
      bulkAborter.signal,
      (file) => compressImage(file, { maxBytes: 256 * 1024, maxDimension: 1920 })
    );
    uploading = false;
    bulkAborter = null;

    items = [...items, ...result.succeeded];
  }
</script>

<svelte:head>
  <title>{m.admin_pack_detail_page_title({ name: data.pack.name })}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-3">
    <a
      href="/admin/packs"
      use:hoverEffect={'swap'}
      class="inline-flex items-center gap-1 text-sm text-brand-text-muted hover:text-brand-text px-2 py-1 rounded-full"
    >
      <ArrowLeft size={14} strokeWidth={2.5} />
      {m.admin_packs_heading()}
    </a>
    <span class="text-brand-text-muted">/</span>
    <h1 class="text-xl font-bold">{data.pack.name}</h1>
    <span class="text-sm text-brand-text-muted ml-1">{m.admin_pack_detail_items_count({ count: items.length })}</span>
    <div class="flex-1"></div>

    {#if data.pack.is_system}
      <span class="h-9 px-3 rounded-lg border border-brand-border text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted inline-flex items-center"
            title={m.admin_pack_detail_system_tooltip()}>
        {m.admin_pack_detail_system_badge()}
      </span>
    {/if}

    <!-- Moderation dropdown — sits next to Add Items. The current status row
         is disabled so you can't re-submit the state that's already applied
         (which is how we got the triple-toast screenshot). -->
    {#if !data.pack.is_system}
    <div class="relative">
      <button
        type="button"
        onclick={() => (modMenuOpen = !modMenuOpen)}
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded-lg border border-brand-border text-sm font-medium inline-flex items-center gap-1.5"
        aria-haspopup="menu"
        aria-expanded={modMenuOpen}
      >
        <Gavel size={14} strokeWidth={2.5} />
        {m.admin_pack_detail_moderate()}
        <ChevronDown
          size={14}
          strokeWidth={2.5}
          class="transition-transform duration-150 {modMenuOpen ? 'rotate-180' : ''}"
        />
      </button>

      {#if modMenuOpen}
        <!-- Click-away backdrop. Kept transparent but covers the viewport so
             any outside click dismisses the menu without needing a document
             listener. -->
        <button
          type="button"
          class="fixed inset-0 z-10 cursor-default"
          aria-label={m.admin_pack_detail_close_menu()}
          onclick={() => (modMenuOpen = false)}
        ></button>

        <div
          class="absolute right-0 mt-1 z-20 w-48 bg-brand-white border border-brand-border rounded-lg shadow-lg overflow-hidden"
          role="menu"
        >
          <div class="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted border-b border-brand-border">
            {m.admin_pack_detail_current_status({ status: data.pack.status })}
          </div>
          <form method="POST" action="?/setStatus" use:enhance class="flex flex-col">
            <input type="hidden" name="pack_id" value={data.pack.id} />

            <button
              type="submit"
              name="status"
              value="active"
              disabled={data.pack.status === 'active'}
              use:pressPhysics={'ghost'}
              class="text-left px-3 py-2 text-sm inline-flex items-center gap-2 hover:bg-muted disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
            >
              <CheckCircle size={14} strokeWidth={2.5} class="text-green-600" />
              {m.admin_pack_detail_mark_active()}
            </button>

            <button
              type="submit"
              name="status"
              value="flagged"
              disabled={data.pack.status === 'flagged'}
              use:pressPhysics={'ghost'}
              class="text-left px-3 py-2 text-sm inline-flex items-center gap-2 hover:bg-muted disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
            >
              <Flag size={14} strokeWidth={2.5} class="text-yellow-600" />
              {m.admin_pack_detail_flag_review()}
            </button>

            <button
              type="submit"
              name="status"
              value="banned"
              disabled={data.pack.status === 'banned'}
              use:pressPhysics={'ghost'}
              class="text-left px-3 py-2 text-sm inline-flex items-center gap-2 hover:bg-muted disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent"
            >
              <Ban size={14} strokeWidth={2.5} class="text-red-600" />
              {m.admin_pack_detail_ban()}
            </button>
          </form>
        </div>
      {/if}
    </div>
    {/if}

    {#if !data.pack.is_system}
      <label
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded-lg border border-brand-border text-sm font-medium cursor-pointer flex items-center gap-1.5"
      >
        <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={handleFileInput} disabled={uploading} />
        <Upload size={14} strokeWidth={2.5} />
        {uploading ? m.admin_pack_detail_uploading() : m.admin_pack_detail_add_items()}
      </label>
    {/if}
  </div>

  {#if data.pack.description}
    <p class="text-sm text-brand-text-muted">{data.pack.description}</p>
  {/if}

  <!-- Metadata strip: owner / visibility / status (read-only, moderation
       actions live in the Gavel dropdown above). owner_username lookup is a
       follow-up; raw owner_id (UUID) is shown for now. -->
  <div class="flex items-center gap-3 text-xs text-brand-text-muted">
    <span>{m.admin_pack_detail_owner()} <b>{data.pack.owner_id ?? '—'}</b></span>
    <span>{m.admin_pack_detail_visibility()} <b>{data.pack.visibility}</b></span>
    <span>{m.admin_pack_detail_status()}
      <b class={
        data.pack.status === 'banned' ? 'text-red-600'
        : data.pack.status === 'flagged' ? 'text-yellow-700'
        : 'text-green-700'
      }>{data.pack.status}</b>
    </span>
  </div>

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="w-10 px-4 py-3">#</th>
          <th class="text-left px-4 py-3">{m.admin_pack_detail_col_preview()}</th>
          <th class="text-left px-4 py-3">{m.admin_pack_detail_col_name()}</th>
          <th class="text-left px-4 py-3">{m.admin_pack_detail_col_version()}</th>
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
                  <ImageIcon size={16} strokeWidth={2.5} />
                </div>
              {/if}
            </td>
            <td class="px-4 py-3 font-medium">{item.name}</td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">{m.admin_pack_detail_version_prefix({ version: item.version_number ?? 1 })}</td>
            <td class="px-4 py-3 text-right">
              {#if !data.pack.is_system}
                <form method="POST" action="?/deleteItem" use:enhance
                  onsubmit={(e) => !confirm(m.admin_pack_detail_delete_item_confirm({ name: item.name })) && e.preventDefault()}>
                  <input type="hidden" name="item_id" value={item.id} />
                  <button
                    type="submit"
                    use:hoverEffect={'swap'}
                    class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                    aria-label={m.admin_pack_detail_delete_item_aria()}>
                    <Trash2 size={14} strokeWidth={2.5} />
                  </button>
                </form>
              {/if}
            </td>
          </tr>
        {/each}
        {#if items.length === 0}
          <tr>
            <td colspan={6} class="px-4 py-8 text-center text-brand-text-muted text-sm">
              {m.admin_pack_detail_empty()}
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
</div>

{#if bulkPanelOpen}
  <BulkUploadProgress entries={bulkEntries} running={uploading} onClose={closeBulkPanel} onAbort={abortBulk} />
{/if}
