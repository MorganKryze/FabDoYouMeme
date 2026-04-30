<!-- frontend/src/lib/components/studio/PackNavigator.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { user } from '$lib/state/user.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { createPack, deletePack, listItems, updatePack } from '$lib/api/studio';
  import { groupsApi } from '$lib/api/groups';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus, Trash2, XCircle, Edit2, ImageIcon, Type } from '$lib/icons';
  import type { Pack } from '$lib/api/types';
  import type { PackKind, StudioGroup } from '$lib/state/studio.svelte';
  import * as m from '$lib/paraglide/messages';

  let showNewPackForm = $state(false);
  let newPackName = $state('');
  let newPackDesc = $state('');
  let newPackKind = $state<PackKind>('image');
  let creating = $state(false);
  // Imperative focus replaces the raw `autofocus` attribute so screen
  // readers announce the focus change when the inline form opens
  // (a11y_autofocus — finding 1.2 in the 2026-04-10 review).
  let newPackNameInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (showNewPackForm && newPackNameInput) newPackNameInput.focus();
  });

  // Inline rename state. `editInput` is bound to the input element that
  // appears in place of the pack label; the effect below focuses + selects
  // its contents the frame after it mounts so the user can immediately
  // overtype. Only one pack can be in rename mode at a time.
  let editingPackId = $state<string | null>(null);
  let editName = $state('');
  let editInput = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (editingPackId && editInput) {
      editInput.focus();
      editInput.select();
    }
  });

  function startRename(pack: Pack, e?: Event) {
    e?.stopPropagation();
    editingPackId = pack.id;
    editName = pack.name;
  }

  function cancelRename() {
    editingPackId = null;
    editName = '';
  }

  async function commitRename() {
    if (!editingPackId) return;
    const id = editingPackId;
    const next = editName.trim();
    const original = studio.packs.find((p) => p.id === id);
    editingPackId = null;
    editName = '';
    if (!original || !next || next === original.name) return;
    // Optimistic update; roll back the single field on failure so the
    // rest of the pack (status, description, etc.) isn't stomped by a
    // stale reference.
    studio.packs = studio.packs.map((p) => p.id === id ? { ...p, name: next } : p);
    try {
      await updatePack(id, { name: next });
    } catch {
      studio.packs = studio.packs.map((p) => p.id === id ? { ...p, name: original.name } : p);
      toast.show(m.studio_toast_pack_rename_failed(), 'error');
    }
  }

  function onEditKey(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); void commitRename(); }
    else if (e.key === 'Escape') { e.preventDefault(); cancelRename(); }
  }

  // Group-owned packs carry `group_id`; everything else follows the original
  // three-way split. Without excluding group packs here they leak into
  // "Official" (owner_id is also null for group packs) and confuse hosts.
  const officialPacks = $derived(
    studio.packs.filter((p) => p.owner_id === null && !p.group_id)
  );
  const myPacks = $derived(studio.packs.filter((p) => p.owner_id === user.id));
  const publicPacks = $derived(
    studio.packs.filter(
      (p) => p.owner_id !== null && p.owner_id !== user.id && p.status === 'active' && !p.group_id
    )
  );
  const flaggedPacks = $derived(studio.packs.filter((p) => p.status === 'flagged'));

  // Group sections source packs from `studio.packs` (not `g.packs`) so inline
  // renames — which only touch studio.packs — reflect in the navigator
  // without re-sync. `studio.groups` still supplies the label + admin status.
  const groupSections = $derived(
    studio.groups
      .map((g) => ({ g, packs: studio.packs.filter((p) => p.group_id === g.id) }))
      .filter((s) => s.packs.length > 0)
  );

  async function selectPack(packId: string) {
    studio.selectPack(packId);
    studio.items = await listItems(packId).catch(() => []);
  }

  async function submitNewPack() {
    if (!newPackName.trim()) return;
    creating = true;
    try {
      const pack = await createPack({ name: newPackName.trim(), description: newPackDesc.trim() || undefined });
      // Pack kind isn't a backend column — items carry payload_version. Stash
      // the user's intent so the right-pane editor opens in the correct mode
      // before any items exist (lost on reload, then re-inferred from items).
      studio.rememberKind(pack.id, newPackKind);
      studio.packs = [...studio.packs, pack];
      await selectPack(pack.id);
      showNewPackForm = false;
      newPackName = '';
      newPackDesc = '';
      newPackKind = 'image';
    } catch {
      toast.show(m.studio_toast_pack_create_failed_alt(), 'error');
    } finally {
      creating = false;
    }
  }

  async function banPack(pack: Pack) {
    try {
      await updatePack(pack.id, { status: 'banned' });
      studio.packs = studio.packs.map((p) => p.id === pack.id ? { ...p, status: 'banned' } : p);
    } catch {
      toast.show(m.studio_toast_pack_ban_failed(), 'error');
    }
  }

  async function clearFlag(pack: Pack) {
    try {
      await updatePack(pack.id, { status: 'active' });
      studio.packs = studio.packs.map((p) => p.id === pack.id ? { ...p, status: 'active' } : p);
    } catch {
      toast.show(m.studio_toast_pack_clear_failed(), 'error');
    }
  }

  async function handleDelete(pack: Pack, e: MouseEvent) {
    e.stopPropagation();
    if (!confirm(m.studio_confirm_delete_pack({ name: pack.name }))) return;
    try {
      // Group packs need the group-scoped delete so audit trail + per-group
      // quotas stay in sync. Personal packs take the existing /api/packs path.
      if (pack.group_id) {
        await groupsApi.deletePack(pack.group_id, pack.id);
      } else {
        await deletePack(pack.id);
      }
      studio.packs = studio.packs.filter((p) => p.id !== pack.id);
      studio.forgetKind(pack.id);
      if (studio.selectedPackId === pack.id) {
        studio.selectedPackId = null;
        studio.items = [];
      }
      toast.show(m.studio_toast_pack_deleted(), 'success');
    } catch {
      toast.show(m.studio_toast_pack_delete_failed(), 'error');
    }
  }

  // A group pack is "manageable" (renamable, deletable) when the caller is
  // platform admin OR admin of that specific group — mirrors the spec and
  // the access rules in the backend's canAdminPack helper.
  function canManageGroupPack(pack: Pack): boolean {
    if (!pack.group_id) return false;
    if (user.role === 'admin') return true;
    const g = studio.groups.find((g) => g.id === pack.group_id);
    return g?.member_role === 'admin';
  }
</script>

<div class="flex flex-col gap-2 p-3">
  {#snippet packGroup(label: string, packs: Pack[], deletable: boolean)}
    {#if packs.length > 0}
      <div class="flex flex-col gap-0.5">
        <p class="text-xs font-semibold uppercase text-brand-text-muted tracking-wider px-2 py-1">{label}</p>
        {#each packs as pack}
          <div
            class="group relative flex items-center gap-1 pr-1 rounded-md transition-colors
              before:content-[''] before:absolute before:left-0 before:top-1.5 before:bottom-1.5 before:w-[3px] before:rounded-r-full before:transition-colors
              {studio.selectedPackId === pack.id
                ? 'bg-black/5 before:bg-primary'
                : 'hover:bg-black/5 before:bg-transparent hover:before:bg-primary/40'}"
          >
            {#if editingPackId === pack.id}
              <input
                bind:value={editName}
                bind:this={editInput}
                onkeydown={onEditKey}
                onblur={() => void commitRename()}
                type="text"
                class="flex-1 min-w-0 mx-1 my-1 h-7 px-2 rounded border border-brand-border-heavy bg-brand-white text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label={m.studio_nav_rename_aria()}
              />
            {:else}
              <button
                type="button"
                onclick={() => selectPack(pack.id)}
                ondblclick={deletable ? (e) => startRename(pack, e) : undefined}
                class="flex-1 text-left px-2 py-1.5 text-sm min-w-0
                  {studio.selectedPackId === pack.id ? 'text-primary font-medium' : 'text-brand-text'}"
              >
                <span class="flex items-center gap-1.5 truncate">
                  <span class="truncate {pack.status === 'banned' ? 'line-through text-brand-text-muted' : ''}">{pack.name}</span>
                  {#if pack.is_system}
                    <span class="shrink-0 text-[9px] font-bold uppercase tracking-wider text-brand-text-muted border border-brand-border px-1 py-[1px] rounded"
                          title={m.studio_nav_badge_system_title()}>
                      {m.studio_nav_badge_system()}
                    </span>
                  {/if}
                  {#if pack.status === 'banned'}
                    <span class="shrink-0 text-[9px] font-bold uppercase tracking-wider text-red-600 border border-red-600 px-1 py-[1px] rounded"
                          title={m.studio_nav_badge_banned_title()}>
                      {m.studio_nav_badge_banned()}
                    </span>
                  {:else if pack.status === 'flagged'}
                    <span class="shrink-0 text-[9px] font-bold uppercase tracking-wider text-yellow-700 border border-yellow-600 px-1 py-[1px] rounded"
                          title={m.studio_nav_badge_flagged_title()}>
                      {m.studio_nav_badge_flagged()}
                    </span>
                  {/if}
                </span>
              </button>
              {#if deletable}
                <button
                  type="button"
                  onclick={(e) => startRename(pack, e)}
                  class="opacity-0 group-hover:opacity-100 text-brand-text-muted hover:text-brand-text transition-all p-1 rounded-full"
                  aria-label={m.studio_nav_rename_aria()}
                  title={m.studio_nav_rename_title()}
                >
                  <Edit2 size={12} strokeWidth={2.5} />
                </button>
                <button
                  type="button"
                  onclick={(e) => handleDelete(pack, e)}
                  class="opacity-0 group-hover:opacity-100 text-brand-text-muted hover:text-red-600 transition-all p-1 rounded-full"
                  aria-label={m.studio_nav_delete_aria()}
                  title={m.studio_nav_delete_title()}
                >
                  <Trash2 size={12} strokeWidth={2.5} />
                </button>
              {/if}
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  {/snippet}

  {@render packGroup(m.studio_nav_group_official(), officialPacks, false)}
  {@render packGroup(m.studio_nav_group_public(), publicPacks, false)}
  {@render packGroup(m.studio_nav_group_mine(), myPacks, true)}

  <!-- Phase 3 — one section per group the user is in. Any group pack is
       deletable if the caller can admin it (platform admin or group admin);
       otherwise it's a read-and-edit-items surface, not a manage surface. -->
  {#each groupSections as s (s.g.id)}
    {@render packGroup(m.studio_nav_group_group({ name: s.g.name }), s.packs, canManageGroupPack(s.packs[0]))}
  {/each}

  <!-- Admin moderation section -->
  {#if user.role === 'admin' && flaggedPacks.length > 0}
    <div class="mt-2 border-t border-brand-border pt-2">
      <p class="text-xs font-semibold uppercase text-brand-text-muted tracking-wider px-2 py-1">
        {m.studio_nav_moderation_heading({ count: flaggedPacks.length })}
      </p>
      {#each flaggedPacks as pack}
        <div class="px-2 py-1.5 rounded-md text-sm">
          <span class="block truncate text-yellow-700">{pack.name}</span>
          <div class="flex gap-1 mt-0.5">
            <button type="button" onclick={() => banPack(pack)}
              class="text-xs text-red-600 underline hover:text-red-800">{m.studio_nav_moderation_ban()}</button>
            <button type="button" onclick={() => clearFlag(pack)}
              class="text-xs text-brand-text-muted underline hover:text-brand-text">{m.studio_nav_moderation_clear()}</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- New Pack form -->
  <div class={['mt-2', studio.packs.length > 0 && 'border-t border-brand-border pt-2']}>
    {#if showNewPackForm}
      <div class="flex flex-col gap-2 px-1">
        <input
          bind:value={newPackName}
          bind:this={newPackNameInput}
          type="text"
          placeholder={m.studio_nav_new_pack_placeholder_name()}
          class="h-8 rounded border border-brand-border-heavy bg-brand-white px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <input
          bind:value={newPackDesc}
          type="text"
          placeholder={m.studio_nav_new_pack_placeholder_desc()}
          class="h-8 rounded border border-brand-border-heavy bg-brand-white px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <fieldset class="grid grid-cols-2 gap-1" aria-label={m.studio_nav_new_pack_kind_aria()}>
          <label
            class="h-8 rounded border text-xs cursor-pointer inline-flex items-center justify-center gap-1
              {newPackKind === 'image'
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-brand-white border-brand-border text-brand-text-muted hover:text-brand-text'}"
          >
            <input type="radio" bind:group={newPackKind} value="image" class="sr-only" />
            <ImageIcon size={12} strokeWidth={2.5} />
            {m.studio_nav_new_pack_kind_image()}
          </label>
          <label
            class="h-8 rounded border text-xs cursor-pointer inline-flex items-center justify-center gap-1
              {newPackKind === 'text'
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-brand-white border-brand-border text-brand-text-muted hover:text-brand-text'}"
          >
            <input type="radio" bind:group={newPackKind} value="text" class="sr-only" />
            <Type size={12} strokeWidth={2.5} />
            {m.studio_nav_new_pack_kind_text()}
          </label>
          <label
            class="h-8 rounded border text-xs cursor-pointer inline-flex items-center justify-center gap-1
              {newPackKind === 'prompt'
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-brand-white border-brand-border text-brand-text-muted hover:text-brand-text'}"
          >
            <input type="radio" bind:group={newPackKind} value="prompt" class="sr-only" />
            <Type size={12} strokeWidth={2.5} />
            {m.studio_nav_new_pack_kind_prompt()}
          </label>
          <label
            class="h-8 rounded border text-xs cursor-pointer inline-flex items-center justify-center gap-1
              {newPackKind === 'filler'
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-brand-white border-brand-border text-brand-text-muted hover:text-brand-text'}"
          >
            <input type="radio" bind:group={newPackKind} value="filler" class="sr-only" />
            <Type size={12} strokeWidth={2.5} />
            {m.studio_nav_new_pack_kind_filler()}
          </label>
        </fieldset>
        <div class="flex gap-1">
          <button
            type="button"
            onclick={submitNewPack}
            disabled={creating || !newPackName.trim()}
            use:pressPhysics={'dark'}
            class="flex-1 h-8 rounded bg-primary text-primary-foreground text-xs font-medium disabled:opacity-50 inline-flex items-center justify-center gap-1">
            <Plus size={12} strokeWidth={2.5} />
            {creating ? m.studio_nav_new_pack_creating() : m.studio_nav_new_pack_create()}
          </button>
          <button
            type="button"
            onclick={() => showNewPackForm = false}
            use:pressPhysics={'ghost'}
            class="h-8 px-3 rounded border border-brand-border text-xs inline-flex items-center gap-1">
            <XCircle size={12} strokeWidth={2.5} />
            {m.studio_nav_new_pack_cancel()}
          </button>
        </div>
      </div>
    {:else}
      <button
        type="button"
        onclick={() => showNewPackForm = true}
        use:pressPhysics={'ghost'}
        class="w-full text-left px-2 py-1.5 rounded-md border border-brand-border text-sm text-brand-text-muted hover:text-brand-text hover:bg-muted inline-flex items-center gap-1.5">
        <Plus size={12} strokeWidth={2.5} />
        {m.studio_nav_new_pack_trigger()}
      </button>
    {/if}
  </div>
</div>
