<!-- frontend/src/lib/components/studio/PackNavigator.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { user } from '$lib/state/user.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { createPack, deletePack, listItems, updatePack } from '$lib/api/studio';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Plus, Trash2, XCircle, Edit2 } from '$lib/icons';
  import type { Pack } from '$lib/api/types';

  let showNewPackForm = $state(false);
  let newPackName = $state('');
  let newPackDesc = $state('');
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
      toast.show('Failed to rename pack.', 'error');
    }
  }

  function onEditKey(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); void commitRename(); }
    else if (e.key === 'Escape') { e.preventDefault(); cancelRename(); }
  }

  const officialPacks = $derived(studio.packs.filter((p) => p.owner_id === null));
  const myPacks = $derived(studio.packs.filter((p) => p.owner_id === user.id));
  const publicPacks = $derived(
    studio.packs.filter((p) => p.owner_id !== null && p.owner_id !== user.id && p.status === 'active')
  );
  const flaggedPacks = $derived(studio.packs.filter((p) => p.status === 'flagged'));

  async function selectPack(packId: string) {
    studio.selectPack(packId);
    studio.items = await listItems(packId).catch(() => []);
  }

  async function submitNewPack() {
    if (!newPackName.trim()) return;
    creating = true;
    try {
      const pack = await createPack({ name: newPackName.trim(), description: newPackDesc.trim() || undefined });
      studio.packs = [...studio.packs, pack];
      await selectPack(pack.id);
      showNewPackForm = false;
      newPackName = '';
      newPackDesc = '';
    } catch {
      toast.show('Failed to create pack.', 'error');
    } finally {
      creating = false;
    }
  }

  async function banPack(pack: Pack) {
    try {
      await updatePack(pack.id, { status: 'banned' });
      studio.packs = studio.packs.map((p) => p.id === pack.id ? { ...p, status: 'banned' } : p);
    } catch {
      toast.show('Failed to ban pack.', 'error');
    }
  }

  async function clearFlag(pack: Pack) {
    try {
      await updatePack(pack.id, { status: 'active' });
      studio.packs = studio.packs.map((p) => p.id === pack.id ? { ...p, status: 'active' } : p);
    } catch {
      toast.show('Failed to clear flag.', 'error');
    }
  }

  async function handleDelete(pack: Pack, e: MouseEvent) {
    e.stopPropagation();
    if (!confirm(`Delete "${pack.name}"? This cannot be undone.`)) return;
    try {
      await deletePack(pack.id);
      studio.packs = studio.packs.filter((p) => p.id !== pack.id);
      if (studio.selectedPackId === pack.id) {
        studio.selectedPackId = null;
        studio.items = [];
      }
      toast.show('Pack deleted.', 'success');
    } catch {
      toast.show('Failed to delete pack.', 'error');
    }
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
                aria-label="Rename pack"
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
                          title="This pack is bundled with the server and is read-only">
                      System
                    </span>
                  {/if}
                  {#if pack.status === 'banned'}
                    <span class="shrink-0 text-[9px] font-bold uppercase tracking-wider text-red-600 border border-red-600 px-1 py-[1px] rounded"
                          title="This pack has been banned by a moderator and cannot be used in games">
                      Banned
                    </span>
                  {:else if pack.status === 'flagged'}
                    <span class="shrink-0 text-[9px] font-bold uppercase tracking-wider text-yellow-700 border border-yellow-600 px-1 py-[1px] rounded"
                          title="This pack has been reported and is awaiting moderator review">
                      Flagged
                    </span>
                  {/if}
                </span>
              </button>
              {#if deletable}
                <button
                  type="button"
                  onclick={(e) => startRename(pack, e)}
                  class="opacity-0 group-hover:opacity-100 text-brand-text-muted hover:text-brand-text transition-all p-1 rounded-full"
                  aria-label="Rename pack"
                  title="Rename pack (or double-click)"
                >
                  <Edit2 size={12} strokeWidth={2.5} />
                </button>
                <button
                  type="button"
                  onclick={(e) => handleDelete(pack, e)}
                  class="opacity-0 group-hover:opacity-100 text-brand-text-muted hover:text-red-600 transition-all p-1 rounded-full"
                  aria-label="Delete pack"
                  title="Delete pack"
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

  {@render packGroup('Official', officialPacks, false)}
  {@render packGroup('Public', publicPacks, false)}
  {@render packGroup('My Packs', myPacks, true)}

  <!-- Admin moderation section -->
  {#if user.role === 'admin' && flaggedPacks.length > 0}
    <div class="mt-2 border-t border-brand-border pt-2">
      <p class="text-xs font-semibold uppercase text-brand-text-muted tracking-wider px-2 py-1">
        Moderation ({flaggedPacks.length})
      </p>
      {#each flaggedPacks as pack}
        <div class="px-2 py-1.5 rounded-md text-sm">
          <span class="block truncate text-yellow-700">{pack.name}</span>
          <div class="flex gap-1 mt-0.5">
            <button type="button" onclick={() => banPack(pack)}
              class="text-xs text-red-600 underline hover:text-red-800">Ban</button>
            <button type="button" onclick={() => clearFlag(pack)}
              class="text-xs text-brand-text-muted underline hover:text-brand-text">Clear</button>
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
          placeholder="Pack name"
          class="h-8 rounded border border-brand-border-heavy bg-brand-white px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <input
          bind:value={newPackDesc}
          type="text"
          placeholder="Description (optional)"
          class="h-8 rounded border border-brand-border-heavy bg-brand-white px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <div class="flex gap-1">
          <button
            type="button"
            onclick={submitNewPack}
            disabled={creating || !newPackName.trim()}
            use:pressPhysics={'dark'}
            class="flex-1 h-8 rounded bg-primary text-primary-foreground text-xs font-medium disabled:opacity-50 inline-flex items-center justify-center gap-1">
            <Plus size={12} strokeWidth={2.5} />
            {creating ? 'Creating…' : 'Create'}
          </button>
          <button
            type="button"
            onclick={() => showNewPackForm = false}
            use:pressPhysics={'ghost'}
            class="h-8 px-3 rounded border border-brand-border text-xs inline-flex items-center gap-1">
            <XCircle size={12} strokeWidth={2.5} />
            Cancel
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
        New Pack
      </button>
    {/if}
  </div>
</div>
