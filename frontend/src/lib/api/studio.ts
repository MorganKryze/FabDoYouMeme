// frontend/src/lib/api/studio.ts
import { api } from './client';
import type { Pack, GameItem, ItemVersion } from './types';

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
  return api.get<GameItem[]>(`/api/packs/${packId}/items`);
}

export async function createItem(
  packId: string,
  body: { name: string; type: 'image' | 'text' }
): Promise<GameItem> {
  return api.post<GameItem>(`/api/packs/${packId}/items`, body);
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
  return api.post<UploadUrlResponse>('/api/assets/upload-url', body);
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
  return api.patch<GameItem>(`/api/packs/${packId}/items/${itemId}`, { media_key: mediaKey });
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
  return api.get<ItemVersion[]>(
    `/api/packs/${packId}/items/${itemId}/versions`
  );
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
