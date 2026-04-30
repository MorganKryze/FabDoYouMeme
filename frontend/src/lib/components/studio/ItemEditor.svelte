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
  import TextEditor from './TextEditor.svelte';
  import PromptEditor from './PromptEditor.svelte';
  import VersionHistory from './VersionHistory.svelte';
  import * as m from '$lib/paraglide/messages';

  const item = $derived(studio.items.find((i) => i.id === studio.selectedItemId) ?? null);
  const activeVersion = $derived(
    studio.versions.find((v) => v.id === item?.current_version_id) ?? null
  );
  const isSystem = $derived(
    studio.packs.find((p) => p.id === studio.selectedPackId)?.is_system ?? false
  );
  // Per-payload-version editor routing: image (v1), text/filler (v2/v3 share
  // the {text} shape so the same TextEditor fits both), prompt (v4 has its
  // own prefix/suffix shape).
  const payloadVersion = $derived(item?.payload_version ?? 1);
  const isText = $derived(payloadVersion === 2 || payloadVersion === 3);
  const isPrompt = $derived(payloadVersion === 4);

  const initialText = $derived(
    extractText(activeVersion?.payload) ??
    extractText(item?.payload) ??
    ''
  );

  const initialPromptPayload = $derived.by(() => {
    const src = activeVersion?.payload ?? item?.payload;
    if (src && typeof src === 'object') {
      const prefix = (src as { prefix?: unknown }).prefix;
      const suffix = (src as { suffix?: unknown }).suffix;
      return {
        prefix: typeof prefix === 'string' ? prefix : '',
        suffix: typeof suffix === 'string' ? suffix : '',
      };
    }
    return { prefix: '', suffix: '' };
  });

  function extractText(payload: unknown): string | null {
    if (payload && typeof payload === 'object' && 'text' in payload) {
      const t = (payload as { text: unknown }).text;
      return typeof t === 'string' ? t : null;
    }
    return null;
  }

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
        { media_key }
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
      toast.show(m.studio_toast_version_saved(), 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'unknown error';
      console.error('[studio] save version failed:', err);
      toast.show(m.studio_toast_version_save_failed({ reason: message }), 'error');
    }
  }

  async function handleTextSave(text: string) {
    if (!studio.selectedPackId || !studio.selectedItemId || !item) return;
    const trimmed = text.trim();
    if (!trimmed) return;
    await persistVersion({ text: trimmed });
  }

  async function handlePromptSave(payload: { prefix: string; suffix: string }) {
    if (!studio.selectedPackId || !studio.selectedItemId || !item) return;
    const prefix = payload.prefix.trim();
    const suffix = payload.suffix.trim();
    if (!prefix && !suffix) return;
    await persistVersion({ prefix, suffix });
  }

  async function persistVersion(payload: Record<string, unknown>) {
    if (!studio.selectedPackId || !studio.selectedItemId) return;
    try {
      const version = await createItemVersion(
        studio.selectedPackId,
        studio.selectedItemId,
        { payload }
      );
      const updated = await promoteVersion(
        studio.selectedPackId,
        studio.selectedItemId,
        version.id
      );
      const enriched = {
        ...updated,
        payload,
        version_number: version.version_number
      };
      studio.items = studio.items.map((i) => i.id === enriched.id ? enriched : i);
      const versions = await listVersions(studio.selectedPackId, studio.selectedItemId);
      studio.versions = versions;
      toast.show(m.studio_toast_version_saved(), 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'unknown error';
      console.error('[studio] save version failed:', err);
      toast.show(m.studio_toast_version_save_failed({ reason: message }), 'error');
    }
  }
</script>

<div class="flex flex-col h-full" use:reveal>
  {#if item}
    <div class="px-4 py-3 border-b border-brand-border shrink-0">
      <p class="text-sm font-semibold truncate">{item.name}</p>
      <p class="text-xs text-brand-text-muted">{(isText || isPrompt) ? m.studio_editor_version_count_text({ count: studio.versions.length }) : m.studio_editor_version_count_image({ count: studio.versions.length })}</p>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#key studio.selectedItemId}
        {#if isPrompt}
          {#if isSystem}
            <p class="whitespace-pre-wrap text-sm">
              {initialPromptPayload.prefix}
              <span class="px-1 underline decoration-2 underline-offset-2">___</span>
              {initialPromptPayload.suffix}
            </p>
          {:else}
            <PromptEditor
              initialPrefix={initialPromptPayload.prefix}
              initialSuffix={initialPromptPayload.suffix}
              onSave={handlePromptSave}
            />
          {/if}
        {:else if isText}
          {#if isSystem}
            <p class="whitespace-pre-wrap text-sm">{initialText}</p>
          {:else}
            <TextEditor initialValue={initialText} onSave={handleTextSave} />
          {/if}
        {:else}
          <ImageEditor
            src={activeVersion?.media_url ?? null}
            onSave={handleImageSave}
            readOnly={isSystem}
          />
        {/if}
      {/key}
    </div>

    <VersionHistory />
  {/if}
</div>
