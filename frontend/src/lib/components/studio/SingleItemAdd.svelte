<!-- frontend/src/lib/components/studio/SingleItemAdd.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { uploadImageItem, validateImageFile } from '$lib/api/studio';
  import { pressPhysics } from '$lib/actions/pressPhysics';
  import { Upload } from '$lib/icons';

  let file = $state<File | null>(null);
  let name = $state('');
  let previewUrl = $state<string | null>(null);
  let submitting = $state(false);
  let inlineError = $state<string | null>(null);

  function pickFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const f = input.files?.[0] ?? null;
    setFile(f);
    input.value = '';
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
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
      inlineError = `${f.name}: ${err}`;
      return;
    }
    file = f;
    name = f.name.replace(/\.[^.]+$/, '');
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(f);
  }

  async function submit() {
    if (!file || !name.trim()) return;
    submitting = true;
    inlineError = null;
    const result = await uploadImageItem(studio.selectedPackId!, name.trim(), file);
    submitting = false;
    if (result.ok) {
      studio.items = [...studio.items, result.item];
      studio.selectItem(result.item.id);
      if (previewUrl) URL.revokeObjectURL(previewUrl);
      file = null;
      name = '';
      previewUrl = null;
      toast.show('Item added.', 'success');
    } else {
      inlineError = result.error;
    }
  }
</script>

<div
  class="flex flex-col gap-3 p-4"
  ondragover={(e) => e.preventDefault()}
  ondrop={onDrop}
  role="region"
  aria-label="Add a single item — drag an image here"
>
  <h3 class="text-sm font-semibold">Add an item</h3>

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
        Drop or pick an image
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
    <label for="single-add-name" class="text-xs font-medium">Name</label>
    <input
      id="single-add-name"
      type="text"
      bind:value={name}
      class="h-9 rounded border border-brand-border-heavy bg-brand-white px-3 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
    />
  </div>

  <p class="text-[11px] text-brand-text-muted">JPEG / PNG / WebP · max 10 MB</p>

  {#if inlineError}
    <p class="text-xs text-red-600">{inlineError}</p>
  {/if}

  <button
    type="button"
    disabled={!file || !name.trim() || submitting}
    onclick={submit}
    use:pressPhysics={'dark'}
    class="h-9 px-4 rounded-lg bg-primary text-primary-foreground text-sm font-medium inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
  >
    <Upload size={14} strokeWidth={2.5} />
    {submitting ? 'Adding…' : 'Add to pack'}
  </button>
</div>
