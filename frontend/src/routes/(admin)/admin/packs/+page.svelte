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
  import * as m from '$lib/paraglide/messages';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let packs = $state<Pack[]>(untrack(() => data.packs));
  let showNewRow = $state(false);

  // Re-seed local state when the URL filter changes. Optimistic create/delete
  // updates survive because they happen under a stable URL — `data.packs` is
  // read via `untrack` so this effect only fires on `data.language` changes.
  $effect(() => {
    void data.language;
    untrack(() => { packs = data.packs; });
  });

  // `use:enhance` updates the `form` prop several times per submission
  // (pending → result → post-invalidate refetch), each update firing the
  // effect. Without this guard we got 3× toasts. A plain `let` (not
  // `$state`) skips reactivity, so writing from inside the effect is safe.
  let lastForm: ActionData | undefined;
  // Imperative focus replaces `autofocus` (a11y_autofocus): when the inline
  // form opens, move focus once into the Name input so keyboard/screen-reader
  // users follow the context change. Finding 1.2 in the 2026-04-10 review.
  let newPackNameInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (showNewRow && newPackNameInput) newPackNameInput.focus();
  });

  $effect(() => {
    if (form === lastForm) return;
    lastForm = form;
    if (form?.created) { packs = [...packs, form.created]; showNewRow = false; toast.show(m.admin_packs_toast_created(), 'success'); }
    if (form?.deleted) { packs = packs.filter((p) => p.id !== form.deleted); toast.show(m.admin_packs_toast_deleted(), 'success'); }
    if (form?.createError || form?.deleteError) toast.show(form.createError ?? form.deleteError, 'error');
  });
</script>

<svelte:head>
  <title>{m.admin_packs_page_title()}</title>
</svelte:head>

<div class="p-6 flex flex-col gap-4" use:reveal>
  <div class="flex items-center gap-4">
    <h1 class="text-xl font-bold flex-1">{m.admin_packs_heading()}</h1>
    <button
      type="button"
      onclick={() => showNewRow = !showNewRow}
      use:pressPhysics={'dark'}
      use:hoverEffect={'swap'}
      class="h-9 px-4 rounded-lg border border-brand-border bg-primary text-primary-foreground text-sm font-medium inline-flex items-center gap-1.5">
      <Plus size={14} strokeWidth={2.5} />
      {m.admin_packs_new()}
    </button>
  </div>

  <div class="flex items-center gap-2 text-xs">
    <span class="text-brand-text-muted font-medium">{m.admin_packs_filter_language()}</span>
    <div class="inline-flex rounded-full border border-brand-border overflow-hidden">
      <a href="/admin/packs" data-sveltekit-replacestate
        class="px-3 py-1 transition-colors {data.language === null ? 'bg-brand-text text-brand-white' : 'hover:bg-muted/40'}">
        {m.admin_packs_filter_all()}
      </a>
      <a href="/admin/packs?language=en" data-sveltekit-replacestate
        class="px-3 py-1 border-l border-brand-border transition-colors {data.language === 'en' ? 'bg-brand-text text-brand-white' : 'hover:bg-muted/40'}">
        {m.admin_packs_filter_en()}
      </a>
      <a href="/admin/packs?language=fr" data-sveltekit-replacestate
        class="px-3 py-1 border-l border-brand-border transition-colors {data.language === 'fr' ? 'bg-brand-text text-brand-white' : 'hover:bg-muted/40'}">
        {m.admin_packs_filter_fr()}
      </a>
      <a href="/admin/packs?language=multi" data-sveltekit-replacestate
        class="px-3 py-1 border-l border-brand-border transition-colors {data.language === 'multi' ? 'bg-brand-text text-brand-white' : 'hover:bg-muted/40'}">
        {m.admin_packs_filter_multi()}
      </a>
    </div>
  </div>

  {#if showNewRow}
    <form method="POST" action="?/createPack" use:enhance
      class="flex gap-3 items-end rounded-lg border border-dashed border-brand-border p-3">
      <div class="flex flex-col gap-1 flex-1">
        <label for="new-pack-name" class="text-xs font-medium">{m.admin_packs_field_name()}</label>
        <input id="new-pack-name" name="name" type="text" required
          bind:this={newPackNameInput}
          placeholder={m.admin_packs_field_name_placeholder()}
          class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <div class="flex flex-col gap-1 flex-1">
        <label for="new-pack-description" class="text-xs font-medium">{m.admin_packs_field_description()}</label>
        <input id="new-pack-description" name="description" type="text" placeholder={m.admin_packs_field_description_placeholder()}
          class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring" />
      </div>
      <button
        type="submit"
        use:pressPhysics={'dark'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded bg-primary text-primary-foreground text-sm font-medium shrink-0 inline-flex items-center gap-1.5">
        <Plus size={14} strokeWidth={2.5} />
        {m.admin_create()}
      </button>
      <button
        type="button"
        onclick={() => showNewRow = false}
        use:pressPhysics={'ghost'}
        use:hoverEffect={'swap'}
        class="h-9 px-4 rounded border border-brand-border text-sm shrink-0 inline-flex items-center gap-1.5">
        <XCircle size={14} strokeWidth={2.5} />
        {m.admin_users_cancel()}
      </button>
    </form>
  {/if}

  <div class="rounded-xl border border-brand-border overflow-hidden">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b border-brand-border bg-muted/40 text-xs font-medium text-brand-text-muted">
          <th class="text-left px-4 py-3">{m.admin_packs_field_name()}</th>
          <th class="text-left px-4 py-3">{m.admin_packs_field_description()}</th>
          <th class="text-left px-4 py-3">{m.admin_packs_col_items()}</th>
          <th class="text-left px-4 py-3">{m.admin_packs_col_status()}</th>
          <th class="text-left px-4 py-3">{m.admin_packs_col_created()}</th>
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each packs as pack, i}
          <tr use:reveal={{ delay: i }} class="border-b border-brand-border/50 hover:bg-muted/20 transition-colors">
            <td class="px-4 py-3">
              <div class="inline-flex items-center gap-2">
                <a href="/admin/packs/{pack.id}" class="font-medium hover:underline">{pack.name}</a>
                <span class="text-[10px] font-semibold px-1.5 py-0.5 rounded border border-brand-border text-brand-text-muted uppercase">
                  {pack.language === 'multi'
                    ? m.admin_packs_filter_multi()
                    : pack.language === 'fr'
                      ? m.admin_packs_filter_fr()
                      : m.admin_packs_filter_en()}
                </span>
              </div>
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
                  if (!confirm(m.admin_packs_delete_confirm())) e.preventDefault();
                }}>
                <input type="hidden" name="pack_id" value={pack.id} />
                <button
                  type="submit"
                  use:hoverEffect={'swap'}
                  class="text-brand-text-muted hover:text-red-600 transition-colors inline-flex items-center p-1 rounded-full"
                  aria-label={m.admin_packs_delete_aria()}>
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
