// frontend/src/lib/api/studio.ts
import { api } from './client';
import type {
  Pack,
  GameItem,
  ItemVersion,
  UploadOutcome,
  BulkUploadOutcome
} from './types';

// ── Packs ─────────────────────────────────────────────────────────────────

export async function listPacks(params?: {
  game_type_id?: string;
}): Promise<Pack[]> {
  const q = new URLSearchParams();
  if (params?.game_type_id) q.set('game_type_id', params.game_type_id);
  const qs = q.toString() ? `?${q}` : '';
  return api.get<Pack[]>(`/api/packs${qs}`);
}

export async function createPack(body: {
  name: string;
  description?: string;
}): Promise<Pack> {
  return api.post<Pack>('/api/packs', body);
}

export async function updatePack(
  id: string,
  body: Partial<Pick<Pack, 'name' | 'description' | 'status'>>
): Promise<Pack> {
  return api.patch<Pack>(`/api/packs/${id}`, body);
}

export async function deletePack(id: string): Promise<void> {
  return api.delete<void>(`/api/packs/${id}`);
}

// ── Items ─────────────────────────────────────────────────────────────────

export async function listItems(packId: string): Promise<GameItem[]> {
  const body = await api.get<{ data: GameItem[] }>(
    `/api/packs/${packId}/items`
  );
  return body.data ?? [];
}

export async function createItem(
  packId: string,
  body: { name: string; payload_version?: number }
): Promise<GameItem> {
  return api.post<GameItem>(`/api/packs/${packId}/items`, {
    name: body.name,
    payload_version: body.payload_version ?? 1
  });
}

export async function updateItem(
  packId: string,
  itemId: string,
  body: Partial<Pick<GameItem, 'name' | 'current_version_id'>>
): Promise<GameItem> {
  return api.patch<GameItem>(`/api/packs/${packId}/items/${itemId}`, body);
}

export async function deleteItem(
  packId: string,
  itemId: string
): Promise<void> {
  return api.delete<void>(`/api/packs/${packId}/items/${itemId}`);
}

export async function reorderItems(
  packId: string,
  positions: { id: string; position: number }[]
): Promise<void> {
  return api.patch<void>(`/api/packs/${packId}/items/reorder`, { positions });
}

// ── Upload flow ────────────────────────────────────────────────────────────

// uploadFileToBackend POSTs a file to /api/assets/upload as multipart/form-data.
// The backend validates MIME + size + ownership, then streams the bytes to
// RustFS. This replaces the old pre-signed direct-PUT path, which can't work
// from the browser unless the bucket has CORS configured.
//
// Don't go through `api.post` — it forces Content-Type: application/json, which
// would break the multipart boundary. The browser fills in the correct header
// on its own when `body` is a FormData instance.
export async function uploadFileToBackend(
  packId: string,
  itemId: string,
  versionNumber: number,
  file: Blob,
  filename: string
): Promise<{ media_key: string }> {
  const form = new FormData();
  form.append('pack_id', packId);
  form.append('item_id', itemId);
  form.append('version_number', String(versionNumber));
  form.append('file', file, filename);

  const res = await fetch('/api/assets/upload', {
    method: 'POST',
    credentials: 'include',
    body: form
  });

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      message = body.error ?? message;
    } catch {}
    throw new Error(`Upload failed: ${message}`);
  }
  return res.json();
}

// Creates a new version row with media_key attached. This replaces the old
// confirmUpload that tried to PATCH the item directly — the backend's item
// PATCH only accepts current_version_id, not media_key.
export async function createItemVersion(
  packId: string,
  itemId: string,
  mediaKey: string
): Promise<ItemVersion> {
  return api.post<ItemVersion>(
    `/api/packs/${packId}/items/${itemId}/versions`,
    { media_key: mediaKey }
  );
}

// Promotes the given version to current on the item. Returns the updated item.
export async function promoteVersion(
  packId: string,
  itemId: string,
  versionId: string
): Promise<GameItem> {
  return api.patch<GameItem>(`/api/packs/${packId}/items/${itemId}`, {
    current_version_id: versionId
  });
}

// ── Client-side file validation (shared by bulk + single flows) ──────────
const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MB
const ALLOWED_MIME = ['image/jpeg', 'image/png', 'image/webp'];

export function validateImageFile(file: File): string | null {
  if (!ALLOWED_MIME.includes(file.type)) return 'unsupported file type';
  if (file.size > MAX_FILE_SIZE) return 'exceeds 10 MB limit';
  return null;
}

// Orchestrates the full upload chain. Returns a discriminated result instead
// of throwing, and best-effort cleans up orphan DB rows if a later step fails.
//
// Steps: create item → POST file to /api/assets/upload (proxied to RustFS) →
// create version with media_key → PATCH item to point current_version_id at
// the new version.
export async function uploadImageItem(
  packId: string,
  name: string,
  file: File
): Promise<UploadOutcome> {
  const invalid = validateImageFile(file);
  if (invalid) return { ok: false, error: invalid, filename: file.name };

  let itemId: string | null = null;
  try {
    const item = await createItem(packId, { name });
    itemId = item.id;

    const { media_key } = await uploadFileToBackend(
      packId,
      item.id,
      1,
      file,
      file.name
    );

    const version = await createItemVersion(packId, item.id, media_key);
    const promoted = await promoteVersion(packId, item.id, version.id);
    // The backend /items list endpoint injects `thumbnail_url` server-side,
    // but the single-item endpoints (createItem / promoteVersion) don't.
    // Construct it client-side so freshly uploaded items render previews
    // without needing a list refetch.
    const thumbnail_url = `/api/assets/media?key=${encodeURIComponent(media_key)}`;
    return { ok: true, item: { ...promoted, media_key, thumbnail_url } };
  } catch (err) {
    if (itemId) {
      try {
        await deleteItem(packId, itemId);
      } catch {
        /* non-fatal: admin can hand-clean from /admin/packs/[id] */
      }
    }
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
      filename: file.name
    };
  }
}

export async function bulkUploadImageItems(
  packId: string,
  files: File[],
  onProgress?: (done: number, total: number, currentName: string) => void
): Promise<BulkUploadOutcome> {
  const outcome: BulkUploadOutcome = { succeeded: [], failed: [] };
  for (let i = 0; i < files.length; i++) {
    const file = files[i];
    onProgress?.(i, files.length, file.name);
    const result = await uploadImageItem(
      packId,
      file.name.replace(/\.[^.]+$/, ''),
      file
    );
    if (result.ok) outcome.succeeded.push(result.item);
    else outcome.failed.push({ filename: result.filename, reason: result.error });
  }
  return outcome;
}

// ── Versions ──────────────────────────────────────────────────────────────

export async function listVersions(
  packId: string,
  itemId: string
): Promise<ItemVersion[]> {
  const body = await api.get<{ data: ItemVersion[] }>(
    `/api/packs/${packId}/items/${itemId}/versions`
  );
  return body.data ?? [];
}

export async function restoreVersion(
  packId: string,
  itemId: string,
  versionId: string
): Promise<GameItem> {
  return api.patch<GameItem>(`/api/packs/${packId}/items/${itemId}`, {
    current_version_id: versionId
  });
}

export async function softDeleteVersion(
  packId: string,
  itemId: string,
  versionId: string
): Promise<void> {
  return api.delete<void>(
    `/api/packs/${packId}/items/${itemId}/versions/${versionId}`
  );
}
