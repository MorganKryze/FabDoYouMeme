<script lang="ts">
  import { enhance } from '$app/forms';
  import { untrack } from 'svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { bulkUploadImageItems, validateImageFile } from '$lib/api/studio';
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

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let items = $state<GameItem[]>(untrack(() => data.items));
  let uploading = $state(false);
  let modMenuOpen = $state(false);

  // `use:enhance` updates the `form` prop several times per submission
  // (pending → result → post-invalidate refetch), each update firing the
  // effect. Without this guard we got 3× toasts and the menu snapping shut
  // mid-reopen. A plain `let` (not `$state`) skips reactivity, so writing
  // `lastForm = form` from inside the effect is safe.
  let lastForm: ActionData | undefined;
  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.deleted) { items = items.filter((i) => i.id !== form.deleted); toast.show('Item deleted.', 'success'); }
    if (form?.deleteError) toast.show(form.deleteError, 'error');
    if (form?.statusUpdated) { toast.show(`Pack marked ${form.statusUpdated}.`, 'success'); modMenuOpen = false; }
    if (form?.statusError) toast.show(form.statusError, 'error');
  });

  async function handleFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length === 0) return;

    const rejected: { filename: string; reason: string }[] = [];
    const accepted: File[] = [];
    for (const f of files) {
      const err = validateImageFile(f);
      if (err) rejected.push({ filename: f.name, reason: err });
      else accepted.push(f);
    }

    uploading = true;
    const result = await bulkUploadImageItems(data.pack.id, accepted);
    uploading = false;

    items = [...items, ...result.succeeded];
    const failed = [...rejected, ...result.failed];
    const ok = result.succeeded.length;
    const ko = failed.length;
    if (ok > 0 && ko === 0) toast.show(`${ok} item${ok === 1 ? '' : 's'} uploaded.`, 'success');
    else if (ok > 0 && ko > 0) toast.show(`${ok} uploaded, ${ko} failed.`, 'warning');
    else toast.show(`Upload failed (${ko}/${ko}).`, 'error');
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

    {#if data.pack.is_system}
      <span class="h-9 px-3 rounded-lg border border-brand-border text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted inline-flex items-center"
            title="Bundled system pack — managed on the server filesystem">
        System · read-only
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
        Moderate
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
          aria-label="Close menu"
          onclick={() => (modMenuOpen = false)}
        ></button>

        <div
          class="absolute right-0 mt-1 z-20 w-48 bg-brand-white border border-brand-border rounded-lg shadow-lg overflow-hidden"
          role="menu"
        >
          <div class="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-brand-text-muted border-b border-brand-border">
            Current: {data.pack.status}
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
              Mark active
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
              Flag for review
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
              Ban pack
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
        {uploading ? 'Uploading…' : 'Add Items'}
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
    <span>Owner: <b>{data.pack.owner_id ?? '—'}</b></span>
    <span>Visibility: <b>{data.pack.visibility}</b></span>
    <span>Status:
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
          <th class="text-left px-4 py-3">Preview</th>
          <th class="text-left px-4 py-3">Name</th>
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
                  <ImageIcon size={16} strokeWidth={2.5} />
                </div>
              {/if}
            </td>
            <td class="px-4 py-3 font-medium">{item.name}</td>
            <td class="px-4 py-3 text-brand-text-muted text-xs">v{item.version_number ?? 1}</td>
            <td class="px-4 py-3 text-right">
              {#if !data.pack.is_system}
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
              {/if}
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
