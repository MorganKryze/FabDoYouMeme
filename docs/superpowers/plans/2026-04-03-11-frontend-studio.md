# Frontend Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Studio page (`/studio`) — a three-panel content-creation tool for managing packs, items, and versions. All authenticated users can access; content and actions depend on pack ownership and role.

**Architecture:** Single SvelteKit page with three co-located Svelte components driven by `StudioState` (Phase 8). Upload flow uses presigned PUT URLs from RustFS. Image editing uses the Canvas API directly (no external library). Version history is a collapsible timeline per item.

**Tech Stack:** SvelteKit 2, Svelte 5 runes, Tailwind CSS v4, Canvas API, typed `$lib/api/studio.ts`, `StudioState` from `$lib/state/studio.svelte.ts`

---

## Files

| File                                                       | Role                                                              |
| ---------------------------------------------------------- | ----------------------------------------------------------------- |
| `frontend/src/routes/(app)/studio/+layout.server.ts`       | Session guard (inherits from (app) layout — no extra auth needed) |
| `frontend/src/routes/(app)/studio/+page.svelte`            | Three-panel layout orchestrator                                   |
| `frontend/src/routes/(app)/studio/+page.server.ts`         | Load initial pack list                                            |
| `frontend/src/lib/api/studio.ts`                           | Typed fetch wrappers for all studio endpoints                     |
| `frontend/src/lib/components/studio/PackNavigator.svelte`  | Left panel: pack list grouped by category                         |
| `frontend/src/lib/components/studio/ItemTable.svelte`      | Center panel: item list with drag reorder + bulk import           |
| `frontend/src/lib/components/studio/ItemEditor.svelte`     | Right panel: image/text editor + version history                  |
| `frontend/src/lib/components/studio/ImageEditor.svelte`    | Canvas-based crop + freehand draw                                 |
| `frontend/src/lib/components/studio/TextEditor.svelte`     | Extensible textarea with character count                          |
| `frontend/src/lib/components/studio/VersionHistory.svelte` | Collapsible timeline with restore/bin/compare actions             |

---

## Task 1: Studio API Module

**Files:**

- Create: `frontend/src/lib/api/studio.ts`

- [ ] **Step 1: Write all studio API wrappers**

```ts
// frontend/src/lib/api/studio.ts
import { apiFetch } from './client';
import type { Pack, GameItem, ItemVersion } from './types';

// ── Packs ─────────────────────────────────────────────────────────────────

export async function listPacks(params?: {
  game_type_id?: string;
}): Promise<Pack[]> {
  const qs = params?.game_type_id ? `?game_type_id=${params.game_type_id}` : '';
  return apiFetch<Pack[]>(`/api/packs${qs}`);
}

export async function createPack(body: {
  name: string;
  description?: string;
}): Promise<Pack> {
  return apiFetch<Pack>('/api/packs', {
    method: 'POST',
    body: JSON.stringify(body)
  });
}

export async function updatePack(
  id: string,
  body: Partial<Pick<Pack, 'name' | 'description' | 'status'>>
): Promise<Pack> {
  return apiFetch<Pack>(`/api/packs/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(body)
  });
}

export async function deletePack(id: string): Promise<void> {
  return apiFetch<void>(`/api/packs/${id}`, { method: 'DELETE' });
}

// ── Items ─────────────────────────────────────────────────────────────────

export async function listItems(packId: string): Promise<GameItem[]> {
  return apiFetch<GameItem[]>(`/api/packs/${packId}/items`);
}

export async function createItem(
  packId: string,
  body: { name: string; type: 'image' | 'text' }
): Promise<GameItem> {
  return apiFetch<GameItem>(`/api/packs/${packId}/items`, {
    method: 'POST',
    body: JSON.stringify(body)
  });
}

export async function updateItem(
  packId: string,
  itemId: string,
  body: Partial<{ name: string; current_version_id: string }>
): Promise<GameItem> {
  return apiFetch<GameItem>(`/api/packs/${packId}/items/${itemId}`, {
    method: 'PATCH',
    body: JSON.stringify(body)
  });
}

export async function deleteItem(
  packId: string,
  itemId: string
): Promise<void> {
  return apiFetch<void>(`/api/packs/${packId}/items/${itemId}`, {
    method: 'DELETE'
  });
}

export async function reorderItems(
  packId: string,
  orderedIds: string[]
): Promise<void> {
  return apiFetch<void>(`/api/packs/${packId}/items/reorder`, {
    method: 'PATCH',
    body: JSON.stringify({ ordered_ids: orderedIds })
  });
}

// ── Upload flow ────────────────────────────────────────────────────────────

interface UploadUrlRequest {
  mime_type: string;
  filename: string;
  size_bytes: number;
  preview_bytes?: string; // base64-encoded first ~512 bytes for server-side MIME validation
}

interface UploadUrlResponse {
  upload_url: string;
  media_key: string;
}

export async function getUploadUrl(
  body: UploadUrlRequest
): Promise<UploadUrlResponse> {
  return apiFetch<UploadUrlResponse>('/api/assets/upload-url', {
    method: 'POST',
    body: JSON.stringify(body)
  });
}

export async function putToRustFS(
  uploadUrl: string,
  file: Blob,
  mimeType: string
): Promise<void> {
  const res = await fetch(uploadUrl, {
    method: 'PUT',
    headers: { 'Content-Type': mimeType },
    body: file
  });
  if (!res.ok) throw new Error(`RustFS upload failed: ${res.status}`);
}

export async function confirmUpload(
  packId: string,
  itemId: string,
  mediaKey: string
): Promise<GameItem> {
  return apiFetch<GameItem>(`/api/packs/${packId}/items/${itemId}`, {
    method: 'PATCH',
    body: JSON.stringify({ media_key: mediaKey })
  });
}

// Orchestrates all 4 steps: create item → get URL → PUT → confirm
export async function uploadImageItem(
  packId: string,
  name: string,
  file: File
): Promise<GameItem> {
  // Step 1: create item record
  const item = await createItem(packId, { name, type: 'image' });

  // Step 2: read first 512 bytes for magic byte validation
  const previewSlice = file.slice(0, 512);
  const previewBuffer = await previewSlice.arrayBuffer();
  const previewBytes = btoa(
    String.fromCharCode(...new Uint8Array(previewBuffer))
  );

  const { upload_url, media_key } = await getUploadUrl({
    mime_type: file.type,
    filename: file.name,
    size_bytes: file.size,
    preview_bytes: previewBytes
  });

  // Step 3: PUT to RustFS
  await putToRustFS(upload_url, file, file.type);

  // Step 4: confirm upload
  return confirmUpload(packId, item.id, media_key);
}

// ── Versions ──────────────────────────────────────────────────────────────

export async function listVersions(
  packId: string,
  itemId: string
): Promise<ItemVersion[]> {
  return apiFetch<ItemVersion[]>(
    `/api/packs/${packId}/items/${itemId}/versions`
  );
}

export async function restoreVersion(
  packId: string,
  itemId: string,
  versionId: string
): Promise<GameItem> {
  return apiFetch<GameItem>(`/api/packs/${packId}/items/${itemId}`, {
    method: 'PATCH',
    body: JSON.stringify({ current_version_id: versionId })
  });
}

export async function softDeleteVersion(
  packId: string,
  itemId: string,
  versionId: string
): Promise<void> {
  return apiFetch<void>(
    `/api/packs/${packId}/items/${itemId}/versions/${versionId}`,
    {
      method: 'DELETE'
    }
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api/studio.ts
git commit -m "feat(frontend/studio): add typed studio API wrappers with 4-step upload flow"
```

---

## Task 2: Studio Page Load

**Files:**

- Create: `frontend/src/routes/(app)/studio/+page.server.ts`
- Create: `frontend/src/routes/(app)/studio/+page.svelte`

- [ ] **Step 1: Write the server load**

```ts
// frontend/src/routes/(app)/studio/+page.server.ts
import type { PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/packs');
  const packs: Pack[] = res.ok ? await res.json() : [];
  return { packs };
};
```

- [ ] **Step 2: Write the three-panel layout page**

```svelte
<!-- frontend/src/routes/(app)/studio/+page.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import PackNavigator from '$lib/components/studio/PackNavigator.svelte';
  import ItemTable from '$lib/components/studio/ItemTable.svelte';
  import ItemEditor from '$lib/components/studio/ItemEditor.svelte';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  $effect(() => {
    studio.packs = data.packs;
  });
</script>

<svelte:head>
  <title>Studio — FabDoYouMeme</title>
</svelte:head>

<div class="flex-1 flex overflow-hidden h-[calc(100vh-3.5rem)]">
  <!-- Left: Pack Navigator (fixed width) -->
  <div class="w-52 shrink-0 border-r border-border overflow-y-auto">
    <PackNavigator />
  </div>

  <!-- Center: Item Table (flexible) -->
  <div class="flex-1 min-w-0 border-r border-border overflow-y-auto">
    {#if studio.selectedPackId}
      <ItemTable />
    {:else}
      <div class="flex items-center justify-center h-full text-muted-foreground text-sm">
        Select a pack to view items.
      </div>
    {/if}
  </div>

  <!-- Right: Item Editor (fixed width) -->
  <div class="w-80 shrink-0 overflow-y-auto">
    {#if studio.selectedItemId}
      <ItemEditor />
    {:else}
      <div class="flex items-center justify-center h-full text-muted-foreground text-sm p-4 text-center">
        Select an item to edit.
      </div>
    {/if}
  </div>
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors (component files don't exist yet — will be resolved in later tasks).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/\(app\)/studio/
git commit -m "feat(frontend/studio): add studio page with three-panel layout"
```

---

## Task 3: Pack Navigator Component

**Files:**

- Create: `frontend/src/lib/components/studio/PackNavigator.svelte`

- [ ] **Step 1: Write the PackNavigator**

```svelte
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
    studio.selectedPackId = packId;
    studio.selectedItemId = null;
    studio.selectedVersionIds = [];
    const items = await listItems(packId).catch(() => []);
    studio.items = items;
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
            <span class="text-xs text-muted-foreground">{pack.item_count ?? 0} items</span>
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
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/studio/PackNavigator.svelte
git commit -m "feat(frontend/studio): add PackNavigator with admin moderation section"
```

---

## Task 4: Item Table Component

**Files:**

- Create: `frontend/src/lib/components/studio/ItemTable.svelte`

- [ ] **Step 1: Write the ItemTable component**

```svelte
<!-- frontend/src/lib/components/studio/ItemTable.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { deleteItem, reorderItems, uploadImageItem, listVersions } from '$lib/api/studio';
  import type { GameItem } from '$lib/api/types';

  const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MB
  const ALLOWED_MIME = ['image/jpeg', 'image/png', 'image/webp'];

  let dragOverZone = $state(false);
  let uploading = $state(false);
  let uploadProgress = $state<{ name: string; done: number; total: number } | null>(null);

  async function selectItem(item: GameItem) {
    studio.selectedItemId = item.id;
    studio.selectedVersionIds = [];
    const versions = await listVersions(studio.selectedPackId!, item.id).catch(() => []);
    studio.versions = versions;
  }

  async function handleDelete(item: GameItem) {
    if (!confirm(`Delete "${item.name}"? This cannot be undone.`)) return;
    try {
      await deleteItem(studio.selectedPackId!, item.id);
      studio.items = studio.items.filter((i) => i.id !== item.id);
      if (studio.selectedItemId === item.id) studio.selectedItemId = null;
    } catch {
      toast.show('Failed to delete item.', 'error');
    }
  }

  async function bulkUpload(files: File[]) {
    const validFiles = files.filter((f) => {
      if (!ALLOWED_MIME.includes(f.type)) {
        toast.show(`${f.name}: unsupported file type.`, 'error');
        return false;
      }
      if (f.size > MAX_FILE_SIZE) {
        toast.show(`${f.name}: file exceeds 10 MB limit.`, 'error');
        return false;
      }
      return true;
    });

    if (validFiles.length === 0) return;

    uploading = true;
    for (let i = 0; i < validFiles.length; i++) {
      const file = validFiles[i];
      uploadProgress = { name: file.name, done: i, total: validFiles.length };
      try {
        const item = await uploadImageItem(studio.selectedPackId!, file.name.replace(/\.[^.]+$/, ''), file);
        studio.items = [...studio.items, item];
      } catch {
        toast.show(`Failed to upload ${file.name}.`, 'error');
      }
    }
    uploadProgress = null;
    uploading = false;
    toast.show(`${validFiles.length} item(s) uploaded.`, 'success');
  }

  function onDropZone(e: DragEvent) {
    e.preventDefault();
    dragOverZone = false;
    const files = Array.from(e.dataTransfer?.files ?? []);
    void bulkUpload(files);
  }

  function onFileInput(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    void bulkUpload(files);
    input.value = '';
  }
</script>

<div class="flex flex-col h-full">
  <!-- Header -->
  <div class="flex items-center gap-3 px-4 py-3 border-b border-border shrink-0">
    <h2 class="text-sm font-semibold flex-1">
      {studio.packs.find((p) => p.id === studio.selectedPackId)?.name ?? 'Items'}
      <span class="text-muted-foreground font-normal">({studio.items.length})</span>
    </h2>

    <label class="h-8 px-3 rounded-md border border-border text-xs font-medium cursor-pointer hover:bg-muted transition-colors flex items-center gap-1">
      <input type="file" accept="image/jpeg,image/png,image/webp" multiple class="sr-only" onchange={onFileInput} />
      Bulk Import
    </label>
  </div>

  <!-- Drop zone overlay -->
  <div
    class="flex-1 overflow-y-auto relative"
    ondragover={(e) => { e.preventDefault(); dragOverZone = true; }}
    ondragleave={() => dragOverZone = false}
    ondrop={onDropZone}
    role="region"
    aria-label="Item list — drag images here to import"
  >
    {#if dragOverZone}
      <div class="absolute inset-0 z-10 border-2 border-dashed border-primary bg-primary/5 flex items-center justify-center">
        <p class="text-primary font-medium">Drop images to import</p>
      </div>
    {/if}

    {#if uploading && uploadProgress}
      <div class="px-4 py-2 bg-muted/50 text-xs text-muted-foreground border-b border-border">
        Uploading {uploadProgress.name}… ({uploadProgress.done + 1}/{uploadProgress.total})
      </div>
    {/if}

    <!-- Item table -->
    {#if studio.items.length === 0}
      <div class="flex flex-col items-center justify-center h-48 text-muted-foreground text-sm gap-2">
        <p>No items yet.</p>
        <p class="text-xs">Drag images here or use Bulk Import.</p>
      </div>
    {:else}
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-border text-xs text-muted-foreground font-medium">
            <th class="text-left px-4 py-2">Name</th>
            <th class="text-left px-4 py-2">Type</th>
            <th class="text-right px-4 py-2">Ver.</th>
            <th class="px-4 py-2"></th>
          </tr>
        </thead>
        <tbody>
          {#each studio.items as item}
            <tr
              class="border-b border-border/50 hover:bg-muted/40 cursor-pointer transition-colors
                {studio.selectedItemId === item.id ? 'bg-primary/5' : ''}"
              onclick={() => selectItem(item)}
            >
              <td class="px-4 py-2">
                <div class="flex items-center gap-2">
                  {#if item.thumbnail_url}
                    <img src={item.thumbnail_url} alt="" class="h-8 w-8 rounded object-cover shrink-0" />
                  {:else}
                    <div class="h-8 w-8 rounded bg-muted shrink-0 flex items-center justify-center text-muted-foreground text-xs">
                      {item.type === 'image' ? '🖼' : 'T'}
                    </div>
                  {/if}
                  <span class="truncate max-w-[8rem]">{item.name}</span>
                </div>
              </td>
              <td class="px-4 py-2">
                <span class="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                  {item.type}
                </span>
              </td>
              <td class="px-4 py-2 text-right text-muted-foreground">v{item.version_number ?? 1}</td>
              <td class="px-4 py-2 text-right">
                <button
                  type="button"
                  onclick={(e) => { e.stopPropagation(); handleDelete(item); }}
                  class="text-muted-foreground hover:text-red-600 transition-colors text-lg leading-none"
                  aria-label="Delete item"
                >
                  ×
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/studio/ItemTable.svelte
git commit -m "feat(frontend/studio): add ItemTable with drag-and-drop bulk image import"
```

---

## Task 5: Image Editor Component

**Files:**

- Create: `frontend/src/lib/components/studio/ImageEditor.svelte`

- [ ] **Step 1: Write the Canvas-based image editor**

```svelte
<!-- frontend/src/lib/components/studio/ImageEditor.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';

  let { src, onSave }: { src: string | null; onSave: (blob: Blob) => void } = $props();

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let drawing = $state(false);
  let tool = $state<'draw' | 'crop'>('draw');
  let strokeColor = $state('#ff0000');
  let strokeWidth = $state(3);

  let lastX = 0;
  let lastY = 0;

  onMount(() => {
    ctx = canvas.getContext('2d');
    if (src) loadImage(src);
  });

  $effect(() => {
    if (src) loadImage(src);
  });

  function loadImage(url: string) {
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      if (!ctx) return;
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      ctx.drawImage(img, 0, 0);
    };
    img.src = url;
  }

  function getCanvasPos(e: MouseEvent | TouchEvent): { x: number; y: number } {
    const rect = canvas.getBoundingClientRect();
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
    const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
    return {
      x: (clientX - rect.left) * scaleX,
      y: (clientY - rect.top) * scaleY,
    };
  }

  function startDraw(e: MouseEvent | TouchEvent) {
    if (tool !== 'draw' || !ctx) return;
    drawing = true;
    const { x, y } = getCanvasPos(e);
    lastX = x; lastY = y;
  }

  function doDraw(e: MouseEvent | TouchEvent) {
    if (!drawing || tool !== 'draw' || !ctx) return;
    const { x, y } = getCanvasPos(e);
    ctx.beginPath();
    ctx.moveTo(lastX, lastY);
    ctx.lineTo(x, y);
    ctx.strokeStyle = strokeColor;
    ctx.lineWidth = strokeWidth;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.stroke();
    lastX = x; lastY = y;
  }

  function stopDraw() {
    drawing = false;
  }

  function save() {
    canvas.toBlob((blob) => {
      if (blob) onSave(blob);
    }, 'image/png');
  }
</script>

<div class="flex flex-col gap-3">
  <!-- Toolbar -->
  <div class="flex items-center gap-3 flex-wrap">
    <div class="flex gap-1 rounded-md border border-border overflow-hidden">
      <button type="button" onclick={() => tool = 'draw'}
        class="px-3 py-1 text-xs font-medium transition-colors {tool === 'draw' ? 'bg-primary text-primary-foreground' : 'hover:bg-muted'}">
        Draw
      </button>
    </div>
    <label class="flex items-center gap-1 text-xs text-muted-foreground">
      Color
      <input type="color" bind:value={strokeColor} class="h-6 w-8 rounded border-none cursor-pointer" />
    </label>
    <label class="flex items-center gap-1 text-xs text-muted-foreground">
      Size
      <input type="range" min={1} max={20} bind:value={strokeWidth} class="w-16 accent-primary" />
    </label>
  </div>

  <!-- Canvas -->
  <div class="relative overflow-hidden rounded-lg border border-border bg-muted/20">
    <canvas
      bind:this={canvas}
      class="w-full cursor-crosshair"
      onmousedown={startDraw}
      onmousemove={doDraw}
      onmouseup={stopDraw}
      onmouseleave={stopDraw}
      ontouchstart={(e) => { e.preventDefault(); startDraw(e); }}
      ontouchmove={(e) => { e.preventDefault(); doDraw(e); }}
      ontouchend={stopDraw}
    ></canvas>
  </div>

  <button type="button" onclick={save}
    class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors">
    Save as new version
  </button>
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/studio/ImageEditor.svelte
git commit -m "feat(frontend/studio): add Canvas-based image editor (draw + save version)"
```

---

## Task 6: Text Editor and Version History Components

**Files:**

- Create: `frontend/src/lib/components/studio/TextEditor.svelte`
- Create: `frontend/src/lib/components/studio/VersionHistory.svelte`

- [ ] **Step 1: Write TextEditor**

```svelte
<!-- frontend/src/lib/components/studio/TextEditor.svelte -->
<script lang="ts">
  const MAX_CHARS = 500;
  let { initialValue = '', onSave }: { initialValue?: string; onSave: (text: string) => void } = $props();

  let value = $state(initialValue);
  const remaining = $derived(MAX_CHARS - value.length);
</script>

<div class="flex flex-col gap-2">
  <div class="relative">
    <textarea
      bind:value
      rows={6}
      maxlength={MAX_CHARS}
      placeholder="Enter text content…"
      class="w-full rounded-lg border border-input bg-background p-3 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-ring"
    ></textarea>
    <span class="absolute bottom-2 right-3 text-xs text-muted-foreground">{remaining}</span>
  </div>

  <button
    type="button"
    onclick={() => onSave(value)}
    disabled={!value.trim()}
    class="h-10 rounded-lg bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50 hover:bg-primary/90 transition-colors"
  >
    Save as new version
  </button>
</div>
```

- [ ] **Step 2: Write VersionHistory**

```svelte
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

  function toggleCompare(versionId: string) {
    if (studio.selectedVersionIds.includes(versionId)) {
      studio.selectedVersionIds = studio.selectedVersionIds.filter((id) => id !== versionId);
    } else if (studio.selectedVersionIds.length < 2) {
      studio.selectedVersionIds = [...studio.selectedVersionIds, versionId];
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
              <button type="button" onclick={() => toggleCompare(version.id)}
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
          class="w-full h-8 rounded-md bg-muted text-sm font-medium hover:bg-muted/80 transition-colors">
          Compare v{studio.versions.find((v) => v.id === studio.selectedVersionIds[0])?.version_number}
          vs v{studio.versions.find((v) => v.id === studio.selectedVersionIds[1])?.version_number}
        </button>
      </div>
    {/if}
  {/if}
</div>
```

- [ ] **Step 3: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/components/studio/TextEditor.svelte frontend/src/lib/components/studio/VersionHistory.svelte
git commit -m "feat(frontend/studio): add TextEditor and VersionHistory components"
```

---

## Task 7: Item Editor Component

**Files:**

- Create: `frontend/src/lib/components/studio/ItemEditor.svelte`

- [ ] **Step 1: Write the ItemEditor**

```svelte
<!-- frontend/src/lib/components/studio/ItemEditor.svelte -->
<script lang="ts">
  import { studio } from '$lib/state/studio.svelte';
  import { toast } from '$lib/state/toast.svelte';
  import { getUploadUrl, putToRustFS, confirmUpload, listVersions } from '$lib/api/studio';
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

    // Read first 512 bytes for MIME validation
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

      // Refresh version list
      const versions = await listVersions(studio.selectedPackId, studio.selectedItemId);
      studio.versions = versions;

      toast.show('New version saved.', 'success');
    } catch {
      toast.show('Failed to save version.', 'error');
    }
  }

  async function handleTextSave(text: string) {
    if (!studio.selectedPackId || !studio.selectedItemId) return;
    // Text items store content in item payload, not RustFS
    // POST a new version via a dedicated endpoint (design: PATCH item with text payload)
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

<div class="flex flex-col h-full">
  {#if item}
    <div class="px-4 py-3 border-b border-border shrink-0">
      <p class="text-sm font-semibold truncate">{item.name}</p>
      <p class="text-xs text-muted-foreground">{item.type} · {studio.versions.length} version(s)</p>
    </div>

    <div class="flex-1 overflow-y-auto p-4">
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
    </div>

    <VersionHistory />
  {/if}
</div>
```

- [ ] **Step 2: Type-check**

```bash
cd frontend && npm run check
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/studio/ItemEditor.svelte
git commit -m "feat(frontend/studio): add ItemEditor with image/text mode dispatch and version save"
```

---

## Task 8: Integration Smoke Test

- [ ] **Step 1: Start the dev stack**

```bash
docker compose up --build
```

- [ ] **Step 2: Navigate to Studio**

As an authenticated user, go to `http://localhost:5173/studio`. Expected:

- Three-panel layout visible
- Left panel shows pack groups (Official, My Packs, etc.)

- [ ] **Step 3: Create a pack and upload an image**

1. Click "+ New Pack" → enter name → Create
2. Drag an image file into the center panel
3. Verify item appears in the table after upload

- [ ] **Step 4: Edit and version an image**

1. Click an image item
2. Draw on the canvas with the draw tool
3. Click "Save as new version"
4. Open Version History — verify v2 appears

- [ ] **Step 5: Text item**

1. (Requires a text-type pack/item — create via admin if needed)
2. Select text item → edit content → save
3. Version History shows new version

- [ ] **Step 6: Commit if fixes needed**

```bash
git commit -m "fix(frontend/studio): resolve studio smoke test issues"
```
