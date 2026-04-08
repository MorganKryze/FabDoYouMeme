<!-- frontend/src/lib/components/studio/PackNavigator.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { user } from '$lib/state/user.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { createPack, listItems, updatePack } from '$lib/api/studio';
  import type { Pack } from '$lib/api/types';

  let showNewPackForm = $state(false);
  let newPackName = $state('');
  let newPackDesc = $state('');
  let creating = $state(false);

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
</script>

<div class="flex flex-col gap-2 p-3">
  {#snippet packGroup(label: string, packs: Pack[])}
    {#if packs.length > 0}
      <div class="flex flex-col gap-0.5">
        <p class="text-xs font-semibold uppercase text-muted-foreground tracking-wider px-2 py-1">{label}</p>
        {#each packs as pack}
          <button
            type="button"
            onclick={() => selectPack(pack.id)}
            class="w-full text-left px-2 py-1.5 rounded-md text-sm transition-colors
              {studio.selectedPackId === pack.id ? 'bg-primary/10 text-primary font-medium' : 'hover:bg-muted text-foreground'}"
          >
            <span class="block truncate">{pack.name}</span>
          </button>
        {/each}
      </div>
    {/if}
  {/snippet}

  {@render packGroup('Official', officialPacks)}
  {@render packGroup('Public', publicPacks)}
  {@render packGroup('My Packs', myPacks)}

  <!-- Admin moderation section -->
  {#if user.role === 'admin' && flaggedPacks.length > 0}
    <div class="mt-2 border-t border-border pt-2">
      <p class="text-xs font-semibold uppercase text-muted-foreground tracking-wider px-2 py-1">
        Moderation ({flaggedPacks.length})
      </p>
      {#each flaggedPacks as pack}
        <div class="px-2 py-1.5 rounded-md text-sm">
          <span class="block truncate text-yellow-700">{pack.name}</span>
          <div class="flex gap-1 mt-0.5">
            <button type="button" onclick={() => banPack(pack)}
              class="text-xs text-red-600 underline hover:text-red-800">Ban</button>
            <button type="button" onclick={() => clearFlag(pack)}
              class="text-xs text-muted-foreground underline hover:text-foreground">Clear</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}

  <!-- New Pack form -->
  <div class="mt-2 border-t border-border pt-2">
    {#if showNewPackForm}
      <div class="flex flex-col gap-2 px-1">
        <input
          bind:value={newPackName}
          type="text"
          placeholder="Pack name"
          autofocus
          class="h-8 rounded border border-input bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <input
          bind:value={newPackDesc}
          type="text"
          placeholder="Description (optional)"
          class="h-8 rounded border border-input bg-background px-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <div class="flex gap-1">
          <button type="button" onclick={submitNewPack} disabled={creating || !newPackName.trim()}
            class="flex-1 h-8 rounded bg-primary text-primary-foreground text-xs font-medium disabled:opacity-50">
            {creating ? 'Creating…' : 'Create'}
          </button>
          <button type="button" onclick={() => showNewPackForm = false}
            class="h-8 px-3 rounded border border-border text-xs hover:bg-muted">
            Cancel
          </button>
        </div>
      </div>
    {:else}
      <button type="button" onclick={() => showNewPackForm = true}
        class="w-full text-left px-2 py-1.5 rounded-md text-sm text-muted-foreground hover:text-foreground hover:bg-muted transition-colors">
        + New Pack
      </button>
    {/if}
  </div>
</div>
