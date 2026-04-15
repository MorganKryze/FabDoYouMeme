<!-- frontend/src/lib/components/studio/ItemEditor.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import {
    uploadFileToBackend,
    createItemVersion,
    promoteVersion,
    listVersions
  } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import ImageEditor from './ImageEditor.svelte';
  import VersionHistory from './VersionHistory.svelte';

  const item = $derived(studio.items.find((i) => i.id === studio.selectedItemId) ?? null);
  const activeVersion = $derived(
    studio.versions.find((v) => v.id === item?.current_version_id) ?? null
  );

  async function handleImageSave(blob: Blob) {
    if (!studio.selectedPackId || !studio.selectedItemId || !item) return;

    const filename = `${item.name}.jpg`;

    try {
      const nextVersionNumber = studio.versions.length + 1;
      const { media_key } = await uploadFileToBackend(
        studio.selectedPackId,
        studio.selectedItemId,
        nextVersionNumber,
        blob,
        filename
      );
      const version = await createItemVersion(
        studio.selectedPackId,
        studio.selectedItemId,
        media_key
      );
      const updated = await promoteVersion(
        studio.selectedPackId,
        studio.selectedItemId,
        version.id
      );
      // promoteVersion returns the raw DB row — no thumbnail_url and no
      // version_number (both are server-enriched only on the list endpoint).
      // Graft them back on from data we already have so the table row updates
      // without a refetch.
      const thumbnail_url = `/api/assets/media?key=${encodeURIComponent(media_key)}`;
      const enriched = {
        ...updated,
        media_key,
        thumbnail_url,
        version_number: version.version_number
      };
      studio.items = studio.items.map((i) => i.id === enriched.id ? enriched : i);
      const versions = await listVersions(studio.selectedPackId, studio.selectedItemId);
      studio.versions = versions;
      toast.show('New version saved.', 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'unknown error';
      console.error('[studio] save version failed:', err);
      toast.show(`Failed to save version: ${message}`, 'error');
    }
  }
</script>

<div class="flex flex-col h-full" use:reveal>
  {#if item}
    <div class="px-4 py-3 border-b border-brand-border shrink-0">
      <p class="text-sm font-semibold truncate">{item.name}</p>
      <p class="text-xs text-brand-text-muted">image · {studio.versions.length} version(s)</p>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#key studio.selectedItemId}
        <ImageEditor
          src={activeVersion?.media_url ?? null}
          onSave={handleImageSave}
        />
      {/key}
    </div>

    <VersionHistory />
  {/if}
</div>
