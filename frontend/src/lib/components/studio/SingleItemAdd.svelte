<!-- frontend/src/lib/components/studio/SingleItemAdd.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import {
    uploadImageItem,
    uploadTextItem,
    uploadFillerItem,
    uploadPromptItem,
    validateImageFile,
    validateItemText,
    validatePromptPayload,
  } from '$lib/api/studio';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Upload, Save } from '$lib/icons';
  import SentenceWithBlank from '$lib/games/_shared/SentenceWithBlank.svelte';
  import * as m from '$lib/paraglide/messages';

  const kind = $derived(studio.kindFor(studio.selectedPackId));

  // ── Image flow ──────────────────────────────────────────────────────────
  let file = $state<File | null>(null);
  let imageName = $state('');
  let previewUrl = $state<string | null>(null);

  // ── Text / filler flow ─────────────────────────────────────────────────
  // The single-text composer serves both "text" (caption pack) and "filler"
  // (fill-in-the-blank cards) — same { text } payload shape, just different
  // payload_versions selected at upload time.
  const TEXT_MAX = 500;
  let text = $state('');
  let textName = $state('');

  // ── Prompt flow ────────────────────────────────────────────────────────
  let promptName = $state('');
  let promptPrefix = $state('');
  let promptSuffix = $state('');

  // ── Shared ──────────────────────────────────────────────────────────────
  let submitting = $state(false);
  let inlineError = $state<string | null>(null);

  // Reset all forms whenever the user switches packs so half-typed input
  // from one pack doesn't leak into the next.
  $effect(() => {
    studio.selectedPackId;
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    file = null;
    imageName = '';
    previewUrl = null;
    text = '';
    textName = '';
    promptName = '';
    promptPrefix = '';
    promptSuffix = '';
    inlineError = null;
  });

  function pickFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    setFile(f);
    input.value = '';
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    if (kind !== 'image') return;
    const f = e.dataTransfer?.files?.[0] ?? null;
    setFile(f);
  }

  function setFile(f: File | null) {
    inlineError = null;
    if (!f) {
      file = null;
      previewUrl = null;
      return;
    }
    const err = validateImageFile(f);
    if (err) {
      inlineError = m.studio_add_inline_error_named({ name: f.name, reason: err });
      return;
    }
    file = f;
    imageName = f.name.replace(/\.[^.]+$/, '');
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(f);
  }

  async function submitImage() {
    if (!file || !imageName.trim()) return;
    submitting = true;
    inlineError = null;
    const result = await uploadImageItem(studio.selectedPackId!, imageName.trim(), file);
    submitting = false;
    if (result.ok) {
      studio.items = [...studio.items, result.item];
      studio.selectItem(result.item.id);
      if (previewUrl) URL.revokeObjectURL(previewUrl);
      file = null;
      imageName = '';
      previewUrl = null;
      toast.show(m.studio_toast_item_added(), 'success');
    } else {
      inlineError = result.error;
    }
  }

  async function submitText() {
    const trimmedName = textName.trim();
    const trimmedText = text.trim();
    if (!trimmedName) return;
    const textErr = validateItemText(trimmedText);
    if (textErr) {
      inlineError = textErr;
      return;
    }
    submitting = true;
    inlineError = null;
    const upload = kind === 'filler' ? uploadFillerItem : uploadTextItem;
    const result = await upload(studio.selectedPackId!, trimmedName, trimmedText);
    submitting = false;
    if (result.ok) {
      studio.items = [...studio.items, result.item];
      studio.selectItem(result.item.id);
      text = '';
      textName = '';
      toast.show(m.studio_toast_item_added(), 'success');
    } else {
      inlineError = result.error;
    }
  }

  async function submitPrompt() {
    const trimmedName = promptName.trim();
    if (!trimmedName) return;
    const err = validatePromptPayload({ prefix: promptPrefix, suffix: promptSuffix });
    if (err) {
      inlineError = err;
      return;
    }
    submitting = true;
    inlineError = null;
    const result = await uploadPromptItem(studio.selectedPackId!, trimmedName, {
      prefix: promptPrefix,
      suffix: promptSuffix,
    });
    submitting = false;
    if (result.ok) {
      studio.items = [...studio.items, result.item];
      studio.selectItem(result.item.id);
      promptName = '';
      promptPrefix = '';
      promptSuffix = '';
      toast.show(m.studio_toast_item_added(), 'success');
    } else {
      inlineError = result.error;
    }
  }

  const textRemaining = $derived(TEXT_MAX - text.length);
</script>

<div
  class="flex flex-col gap-3 p-4"
  ondragover={(e) => e.preventDefault()}
  ondrop={onDrop}
  role="region"
  aria-label={m.studio_add_region_aria()}
>
  <h3 class="text-sm font-semibold">{m.studio_add_heading()}</h3>

  {#if kind === 'image'}
    <label class="block cursor-pointer">
      {#if previewUrl}
        <img
          src={previewUrl}
          alt=""
          class="w-full h-36 object-cover rounded-md border border-brand-border"
        />
      {:else}
        <div
          class="w-full h-36 rounded-md border-2 border-dashed border-brand-border flex items-center justify-center text-xs text-brand-text-muted"
        >
          {m.studio_add_drop_or_pick()}
        </div>
      {/if}
      <input
        type="file"
        accept="image/jpeg,image/png,image/webp"
        class="sr-only"
        onchange={pickFile}
      />
    </label>

    <div class="flex flex-col gap-1">
      <label for="single-add-name" class="text-xs font-medium">{m.studio_add_label_name()}</label>
      <input
        id="single-add-name"
        type="text"
        bind:value={imageName}
        class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
      />
    </div>

    <p class="text-[11px] text-brand-text-muted">{m.studio_add_image_hint()}</p>

    {#if inlineError}
      <p class="text-xs text-red-600">{inlineError}</p>
    {/if}

    <button
      type="button"
      disabled={!file || !imageName.trim() || submitting}
      onclick={submitImage}
      use:pressPhysics={'dark'}
      class="h-9 px-4 rounded-lg border border-brand-border bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
    >
      <Upload size={14} strokeWidth={2.5} />
      {submitting ? m.studio_add_submitting() : m.studio_add_submit()}
    </button>
  {:else if kind === 'prompt'}
    <div class="flex flex-col gap-1">
      <label for="single-add-prompt-name" class="text-xs font-medium">{m.studio_add_label_name()}</label>
      <input
        id="single-add-prompt-name"
        type="text"
        bind:value={promptName}
        placeholder={m.studio_add_placeholder_name()}
        class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
      />
    </div>

    <div class="flex flex-col gap-1">
      <label for="single-add-prompt-prefix" class="text-xs font-medium">
        {m.studio_prompt_prefix_label()}
      </label>
      <textarea
        id="single-add-prompt-prefix"
        bind:value={promptPrefix}
        rows={2}
        maxlength={TEXT_MAX}
        placeholder={m.studio_prompt_prefix_placeholder()}
        class="w-full rounded border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring"
      ></textarea>
    </div>

    <div class="flex flex-col gap-1">
      <label for="single-add-prompt-suffix" class="text-xs font-medium">
        {m.studio_prompt_suffix_label()}
      </label>
      <textarea
        id="single-add-prompt-suffix"
        bind:value={promptSuffix}
        rows={2}
        maxlength={TEXT_MAX}
        placeholder={m.studio_prompt_suffix_placeholder()}
        class="w-full rounded border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring"
      ></textarea>
    </div>

    <div class="flex flex-col gap-1">
      <span class="text-xs font-medium">{m.studio_prompt_preview_label()}</span>
      <div class="rounded border border-brand-border bg-brand-surface px-3 py-2 text-sm text-brand-text leading-snug">
        <SentenceWithBlank
          prefix={promptPrefix}
          suffix={promptSuffix}
          placeholder={m.studio_prompt_preview_placeholder()}
          size="sm"
        />
      </div>
    </div>

    {#if inlineError}
      <p class="text-xs text-red-600">{inlineError}</p>
    {/if}

    <button
      type="button"
      disabled={!promptName.trim() || (!promptPrefix.trim() && !promptSuffix.trim()) || submitting}
      onclick={submitPrompt}
      use:pressPhysics={'dark'}
      class="h-9 px-4 rounded-lg border border-brand-border bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
    >
      <Save size={14} strokeWidth={2.5} />
      {submitting ? m.studio_add_submitting() : m.studio_add_submit()}
    </button>
  {:else}
    <div class="flex flex-col gap-1">
      <label for="single-add-text-name" class="text-xs font-medium">{m.studio_add_label_name()}</label>
      <input
        id="single-add-text-name"
        type="text"
        bind:value={textName}
        placeholder={m.studio_add_placeholder_name()}
        class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
      />
    </div>

    <div class="flex flex-col gap-1">
      <label for="single-add-text" class="text-xs font-medium">{m.studio_add_label_text()}</label>
      <div class="relative">
        <textarea
          id="single-add-text"
          bind:value={text}
          rows={5}
          maxlength={TEXT_MAX}
          placeholder={m.studio_add_placeholder_text()}
          class="w-full rounded border border-brand-border-heavy bg-brand-white p-3 text-sm resize-none focus:outline-none focus:ring-1 focus:ring-ring"
        ></textarea>
        <span class="absolute bottom-2 right-3 text-[11px] text-brand-text-muted">{textRemaining}</span>
      </div>
    </div>

    <p class="text-[11px] text-brand-text-muted">{m.studio_add_text_hint({ max: TEXT_MAX })}</p>

    {#if inlineError}
      <p class="text-xs text-red-600">{inlineError}</p>
    {/if}

    <button
      type="button"
      disabled={!textName.trim() || !text.trim() || submitting}
      onclick={submitText}
      use:pressPhysics={'dark'}
      class="h-9 px-4 rounded-lg border border-brand-border bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
    >
      <Save size={14} strokeWidth={2.5} />
      {submitting ? m.studio_add_submitting() : m.studio_add_submit()}
    </button>
  {/if}
</div>
