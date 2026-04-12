<!-- frontend/src/lib/components/studio/ItemEditor.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { getUploadUrl, putToRustFS, confirmUpload, listVersions } from '$lib/api/studio';
  import { reveal } from '$lib/actions/reveal';
  import ImageEditor from './ImageEditor.svelte';
  import TextEditor from './TextEditor.svelte';
  import VersionHistory from './VersionHistory.svelte';

  const item = $derived(studio.items.find((i) => i.id === studio.selectedItemId) ?? null);
  const activeVersion = $derived(
    studio.versions.find((v) => v.id === item?.current_version_id) ?? null
  );

  async function handleImageSave(blob: Blob) {
    if (!studio.selectedPackId || !studio.selectedItemId || !item) return;

    const file = new File([blob], `${item.name}.png`, { type: 'image/png' });

    const previewSlice = file.slice(0, 512);
    const previewBuffer = await previewSlice.arrayBuffer();
    const previewBytes = btoa(String.fromCharCode(...new Uint8Array(previewBuffer)));

    try {
      const { upload_url, media_key } = await getUploadUrl({
        mime_type: 'image/png',
        filename: file.name,
        size_bytes: file.size,
        preview_bytes: previewBytes,
      });
      await putToRustFS(upload_url, blob, 'image/png');
      const updated = await confirmUpload(studio.selectedPackId, studio.selectedItemId, media_key);
      studio.items = studio.items.map((i) => i.id === updated.id ? updated : i);
      const versions = await listVersions(studio.selectedPackId, studio.selectedItemId);
      studio.versions = versions;
      toast.show('New version saved.', 'success');
    } catch {
      toast.show('Failed to save version.', 'error');
    }
  }

  async function handleTextSave(text: string) {
    if (!studio.selectedPackId || !studio.selectedItemId) return;
    try {
      const res = await fetch(`/api/packs/${studio.selectedPackId}/items/${studio.selectedItemId}/versions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ content: text }),
      });
      if (!res.ok) throw new Error('Failed to save');
      const versions = await listVersions(studio.selectedPackId, studio.selectedItemId);
      studio.versions = versions;
      toast.show('New version saved.', 'success');
    } catch {
      toast.show('Failed to save version.', 'error');
    }
  }
</script>

<div class="flex flex-col h-full" use:reveal>
  {#if item}
    <div class="px-4 py-3 border-b border-brand-border shrink-0">
      <p class="text-sm font-semibold truncate">{item.name}</p>
      <p class="text-xs text-brand-text-muted">{item.type} · {studio.versions.length} version(s)</p>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#key studio.selectedItemId}
        {#if item.type === 'image'}
          <ImageEditor
            src={activeVersion?.media_url ?? null}
            onSave={handleImageSave}
          />
        {:else}
          <TextEditor
            initialValue={activeVersion?.content ?? ''}
            onSave={handleTextSave}
          />
        {/if}
      {/key}
    </div>

    <VersionHistory />
  {/if}
</div>
