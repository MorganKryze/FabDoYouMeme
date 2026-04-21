<!-- frontend/src/lib/components/studio/VersionHistory.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { restoreVersion, softDeleteVersion } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { ChevronLeft, ChevronRight } from '$lib/icons';
  import type { ItemVersion } from '$lib/api/types';
  import * as m from '$lib/paraglide/messages';

  let open = $state(false);

  const activeVersionId = $derived(
    studio.items.find((i) => i.id === studio.selectedItemId)?.current_version_id ?? null
  );

  async function restore(version: ItemVersion) {
    try {
      const updated = await restoreVersion(studio.selectedPackId!, studio.selectedItemId!, version.id);
      studio.items = studio.items.map((i) => i.id === updated.id ? updated : i);
      toast.show(m.studio_toast_version_restored(), 'success');
    } catch {
      toast.show(m.studio_toast_version_restore_failed(), 'error');
    }
  }

  async function moveToBin(version: ItemVersion) {
    if (!confirm(m.studio_confirm_move_version_to_bin())) return;
    try {
      await softDeleteVersion(studio.selectedPackId!, studio.selectedItemId!, version.id);
      studio.versions = studio.versions.map((v) =>
        v.id === version.id ? { ...v, deleted_at: new Date().toISOString() } : v
      );
    } catch {
      toast.show(m.studio_toast_version_bin_failed(), 'error');
    }
  }
</script>

<div class="border-t border-brand-border">
  <button
    type="button"
    onclick={() => open = !open}
    class="w-full flex items-center justify-between px-4 py-2 text-xs font-semibold uppercase text-brand-text-muted tracking-wider hover:bg-muted/40 transition-colors"
  >
    {m.studio_versions_heading({ count: studio.versions.length })}
    <span>{open ? '▲' : '▼'}</span>
  </button>

  {#if open}
    <div class="flex flex-col divide-y divide-border/50">
      {#each studio.versions as version, i}
        {@const isActive = version.id === activeVersionId}
        {@const isBinned = !!version.deleted_at}
        {@const isSelected = studio.selectedVersionIds.includes(version.id)}

        <div
          use:reveal={{ delay: i }}
          class="px-4 py-2 flex flex-col gap-1 {isBinned ? 'opacity-50' : ''} {isSelected ? 'bg-primary/5' : ''}"
        >
          <div class="flex items-center gap-2 text-xs">
            <span class="font-medium">v{version.version_number}</span>
            <span class="text-brand-text-muted">{new Date(version.created_at).toLocaleDateString()}</span>
            {#if isActive}
              <span class="ml-auto px-1.5 py-0.5 rounded-full bg-primary/10 text-primary text-[10px] font-medium">{m.studio_versions_active()}</span>
            {/if}
          </div>

          {#if !isBinned}
            <div class="flex gap-2 text-xs">
              {#if !isActive}
                <button type="button" onclick={() => restore(version)}
                  class="text-brand-text-muted underline hover:text-brand-text">
                  {m.studio_versions_restore()}
                </button>
              {/if}
              <button type="button" onclick={() => moveToBin(version)}
                class="text-brand-text-muted underline hover:text-red-600">
                {m.studio_versions_move_to_bin()}
              </button>
              <button type="button" onclick={() => studio.toggleVersionSelection(version.id)}
                class="ml-auto {isSelected ? 'text-primary' : 'text-brand-text-muted'} underline hover:text-brand-text">
                {isSelected ? m.studio_versions_deselect() : m.studio_versions_compare()}
              </button>
            </div>
          {:else}
            <p class="text-xs text-brand-text-muted">{m.studio_versions_binned()}</p>
          {/if}
        </div>
      {/each}
    </div>

    {#if studio.selectedVersionIds.length === 2}
      <div class="px-4 py-2 border-t border-brand-border">
        <button
          type="button"
          disabled
          title={m.studio_versions_compare_soon_title()}
          use:pressPhysics={'ghost'}
          class="w-full h-8 rounded-md bg-muted text-sm font-medium opacity-50 cursor-not-allowed inline-flex items-center justify-center gap-1"
        >
          <ChevronLeft size={12} strokeWidth={2.5} />
          {m.studio_versions_compare_pair({ a: studio.versions.find((v) => v.id === studio.selectedVersionIds[0])?.version_number ?? '', b: studio.versions.find((v) => v.id === studio.selectedVersionIds[1])?.version_number ?? '' })}
          <ChevronRight size={12} strokeWidth={2.5} />
        </button>
      </div>
    {/if}
  {/if}
</div>
