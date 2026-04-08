<!-- frontend/src/lib/components/studio/VersionHistory.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { restoreVersion, softDeleteVersion } from '$lib/api/studio';
  import type { ItemVersion } from '$lib/api/types';

  let open = $state(false);

  const activeVersionId = $derived(
    studio.items.find((i) => i.id === studio.selectedItemId)?.current_version_id ?? null
  );

  async function restore(version: ItemVersion) {
    try {
      const updated = await restoreVersion(studio.selectedPackId!, studio.selectedItemId!, version.id);
      studio.items = studio.items.map((i) => i.id === updated.id ? updated : i);
      toast.show('Version restored.', 'success');
    } catch {
      toast.show('Failed to restore version.', 'error');
    }
  }

  async function moveToBin(version: ItemVersion) {
    if (!confirm('Move this version to bin?')) return;
    try {
      await softDeleteVersion(studio.selectedPackId!, studio.selectedItemId!, version.id);
      studio.versions = studio.versions.map((v) =>
        v.id === version.id ? { ...v, deleted_at: new Date().toISOString() } : v
      );
    } catch {
      toast.show('Failed to move version to bin.', 'error');
    }
  }
</script>

<div class="border-t border-border">
  <button
    type="button"
    onclick={() => open = !open}
    class="w-full flex items-center justify-between px-4 py-2 text-xs font-semibold uppercase text-muted-foreground tracking-wider hover:bg-muted/40 transition-colors"
  >
    Version History ({studio.versions.length})
    <span>{open ? '▲' : '▼'}</span>
  </button>

  {#if open}
    <div class="flex flex-col divide-y divide-border/50">
      {#each studio.versions as version}
        {@const isActive = version.id === activeVersionId}
        {@const isBinned = !!version.deleted_at}
        {@const isSelected = studio.selectedVersionIds.includes(version.id)}

        <div class="px-4 py-2 flex flex-col gap-1 {isBinned ? 'opacity-50' : ''} {isSelected ? 'bg-primary/5' : ''}">
          <div class="flex items-center gap-2 text-xs">
            <span class="font-medium">v{version.version_number}</span>
            <span class="text-muted-foreground">{new Date(version.created_at).toLocaleDateString()}</span>
            {#if isActive}
              <span class="ml-auto px-1.5 py-0.5 rounded-full bg-primary/10 text-primary text-[10px] font-medium">active</span>
            {/if}
          </div>

          {#if !isBinned}
            <div class="flex gap-2 text-xs">
              {#if !isActive}
                <button type="button" onclick={() => restore(version)}
                  class="text-muted-foreground underline hover:text-foreground">
                  Restore
                </button>
              {/if}
              <button type="button" onclick={() => moveToBin(version)}
                class="text-muted-foreground underline hover:text-red-600">
                Move to Bin
              </button>
              <button type="button" onclick={() => studio.toggleVersionSelection(version.id)}
                class="ml-auto {isSelected ? 'text-primary' : 'text-muted-foreground'} underline hover:text-foreground">
                {isSelected ? 'Deselect' : 'Compare'}
              </button>
            </div>
          {:else}
            <p class="text-xs text-muted-foreground">In bin — cannot be restored.</p>
          {/if}
        </div>
      {/each}
    </div>

    {#if studio.selectedVersionIds.length === 2}
      <div class="px-4 py-2 border-t border-border">
        <button type="button"
          onclick={() => toast.show('Side-by-side comparison coming soon.', 'warning')}
          class="w-full h-8 rounded-md bg-muted text-sm font-medium hover:bg-muted/80 transition-colors">
          Compare v{studio.versions.find((v) => v.id === studio.selectedVersionIds[0])?.version_number}
          vs v{studio.versions.find((v) => v.id === studio.selectedVersionIds[1])?.version_number}
        </button>
      </div>
    {/if}
  {/if}
</div>
