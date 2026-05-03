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

// Walks the cursor until the server signals "no more pages". The backend
// caps a single page at 100 rows (parsePagination in backend/internal/api/
// packs.go) and used to silently truncate the studio view to the default
// page size; this loop ensures packs with hundreds of items render in full.
// The hard cap of 50 iterations defends against a runaway server that
// keeps returning a non-empty cursor — at 100 rows per page that's 5,000
// items, far above any realistic pack size.
export async function listItems(packId: string): Promise<GameItem[]> {
  const all: GameItem[] = [];
  let cursor: string | null = null;
  for (let i = 0; i < 50; i++) {
    const qs: string = `?limit=100${cursor ? `&after=${encodeURIComponent(cursor)}` : ''}`;
    const body: { data: GameItem[]; next_cursor?: string | null } = await api.get(
      `/api/packs/${packId}/items${qs}`
    );
    if (body.data?.length) all.push(...body.data);
    cursor = body.next_cursor ?? null;
    if (!cursor) return all;
  }
  return all;
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
): Promise<{ media_key: string; orientation?: string }> {
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

// Creates a new version row. For image items, pass `media_key` (the asset key
// returned by uploadFileToBackend). For text items, pass `payload` (the JSON
// blob the game handler will read at round time, e.g. `{ text: "..." }`). The
// backend stores whichever is set; the other defaults to NULL/`{}`.
export async function createItemVersion(
  packId: string,
  itemId: string,
  body: { media_key?: string; payload?: unknown }
): Promise<ItemVersion> {
  return api.post<ItemVersion>(
    `/api/packs/${packId}/items/${itemId}/versions`,
    body
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

    const { media_key, orientation } = await uploadFileToBackend(
      packId,
      item.id,
      1,
      file,
      file.name
    );

    // Persist orientation in the version payload so every consumer (round
    // start, replay, studio) reads the bucket from one source instead of
    // sniffing the file again. A missing orientation (detection failed
    // server-side) is left out of the payload entirely; renderers fall back
    // to a default bucket.
    const payload = orientation ? { orientation } : undefined;
    const version = await createItemVersion(packId, item.id, { media_key, payload });
    const promoted = await promoteVersion(packId, item.id, version.id);
    // The backend /items list endpoint injects `thumbnail_url` server-side,
    // but the single-item endpoints (createItem / promoteVersion) don't.
    // Construct it client-side so freshly uploaded items render previews
    // without needing a list refetch.
    const thumbnail_url = `/api/assets/media?key=${encodeURIComponent(media_key)}`;
    return {
      ok: true,
      item: { ...promoted, media_key, thumbnail_url, payload }
    };
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

// Text counterpart to uploadImageItem. Steps: create item (payload_version 2)
// → create version with `{ text }` payload → PATCH item to point at the new
// version. No asset upload, so one fewer network round-trip than the image
// flow.
const MAX_TEXT_LENGTH = 500;

export function validateItemText(text: string): string | null {
  const trimmed = text.trim();
  if (!trimmed) return 'text is required';
  if (trimmed.length > MAX_TEXT_LENGTH) return `exceeds ${MAX_TEXT_LENGTH} characters`;
  return null;
}

export async function uploadTextItem(
  packId: string,
  name: string,
  text: string
): Promise<UploadOutcome> {
  return uploadTextlikeItem(packId, name, text, 2);
}

// Filler items share the `{ text }` payload shape with captions; only the
// payload_version differs to declare role intent (filler pack vs caption pack).
// One pack of fillers feeds prompt-showdown rooms.
export async function uploadFillerItem(
  packId: string,
  name: string,
  text: string
): Promise<UploadOutcome> {
  return uploadTextlikeItem(packId, name, text, 3);
}

async function uploadTextlikeItem(
  packId: string,
  name: string,
  text: string,
  payloadVersion: number,
): Promise<UploadOutcome> {
  const invalid = validateItemText(text);
  if (invalid) return { ok: false, error: invalid, filename: name };

  let itemId: string | null = null;
  try {
    const item = await createItem(packId, { name, payload_version: payloadVersion });
    itemId = item.id;

    const payload = { text: text.trim() };
    const version = await createItemVersion(packId, item.id, { payload });
    const promoted = await promoteVersion(packId, item.id, version.id);
    return {
      ok: true,
      item: {
        ...promoted,
        payload,
        version_number: version.version_number
      }
    };
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
      filename: name
    };
  }
}

// Prompt items are sentences with a blank, payload_version 4. The blank is
// implicit between prefix and suffix; at least one of the two must be
// non-empty (otherwise the "sentence" is just a blank slot).
export interface PromptPayload { prefix: string; suffix: string; }

export function validatePromptPayload({ prefix, suffix }: PromptPayload): string | null {
  const p = prefix.trim();
  const s = suffix.trim();
  if (!p && !s) return 'prefix or suffix is required';
  if (p.length > MAX_TEXT_LENGTH) return `prefix exceeds ${MAX_TEXT_LENGTH} characters`;
  if (s.length > MAX_TEXT_LENGTH) return `suffix exceeds ${MAX_TEXT_LENGTH} characters`;
  return null;
}

export async function uploadPromptItem(
  packId: string,
  name: string,
  payload: PromptPayload,
): Promise<UploadOutcome> {
  const invalid = validatePromptPayload(payload);
  if (invalid) return { ok: false, error: invalid, filename: name };

  let itemId: string | null = null;
  try {
    const item = await createItem(packId, { name, payload_version: 4 });
    itemId = item.id;

    const versionPayload = {
      prefix: payload.prefix.trim(),
      suffix: payload.suffix.trim(),
    };
    const version = await createItemVersion(packId, item.id, { payload: versionPayload });
    const promoted = await promoteVersion(packId, item.id, version.id);
    return {
      ok: true,
      item: {
        ...promoted,
        payload: versionPayload,
        version_number: version.version_number,
      },
    };
  } catch (err) {
    if (itemId) {
      try {
        await deleteItem(packId, itemId);
      } catch {
        /* non-fatal */
      }
    }
    return {
      ok: false,
      error: err instanceof Error ? err.message : String(err),
      filename: name,
    };
  }
}

// Must match the backend MaxBulkUploadFiles cap in
// backend/internal/api/items_bulk.go. The server rejects requests above this
// with 413 too_many_files; the chunker keeps each request inside the cap.
const BULK_UPLOAD_CHUNK_SIZE = 25;

// Server response shape from POST /api/packs/{id}/items/bulk.
interface BulkServerResult {
  ok: boolean;
  filename: string;
  item?: GameItem;
  reason?: string;
  code?: string;
}

// uploadOneBulkChunk POSTs N files in a single multipart request, with a
// bounded exponential-backoff retry on 429 responses so a transient
// rate-limit hit doesn't lose the whole batch. The endpoint is idempotent
// per file (each file lives in its own DB transaction), so a retried
// request that previously partially succeeded would create duplicates —
// we therefore only retry when the *whole* request was rejected (429 from
// the rate limiter, before per-file processing began).
async function uploadOneBulkChunk(
  packId: string,
  files: File[]
): Promise<{ results: BulkServerResult[] } | { error: string }> {
  const form = new FormData();
  for (const f of files) {
    form.append('file', f, f.name);
    form.append('name', f.name.replace(/\.[^.]+$/, ''));
  }

  // Backoff: 1s, 2s, 4s — well under the per-user upload bucket refill
  // window. 4 attempts max so a chronically-throttled user fails fast.
  const delays = [1000, 2000, 4000];
  for (let attempt = 0; attempt <= delays.length; attempt++) {
    let res: Response;
    try {
      res = await fetch(`/api/packs/${packId}/items/bulk`, {
        method: 'POST',
        credentials: 'include',
        body: form
      });
    } catch (err) {
      return { error: err instanceof Error ? err.message : String(err) };
    }
    if (res.status === 429 && attempt < delays.length) {
      await new Promise((resolve) => setTimeout(resolve, delays[attempt]));
      continue;
    }
    if (!res.ok) {
      // Read the response as text first, then try to parse as JSON. The
      // SvelteKit adapter-node 413 returns a plain-text body
      // ("Content-length of … exceeds limit of … bytes") that earlier
      // versions of this code threw away by going through res.json()
      // directly, leaving the toast with only "Payload Too Large".
      const raw = await res.text().catch(() => '');
      let detail = raw;
      try {
        const parsed = JSON.parse(raw);
        if (parsed?.error) detail = parsed.error;
      } catch {}
      return { error: `${res.status} ${res.statusText}${detail ? ` — ${detail}` : ''}` };
    }
    return (await res.json()) as { results: BulkServerResult[] };
  }
  return { error: 'rate limited (429) — retried 4 times' };
}

// bulkUploadImageItems chunks an arbitrarily large set of files into
// MaxBulkUploadFiles-sized batches and uploads each batch in a single
// HTTP request. Pre-fix this function ran four sequential round-trips per
// file; for an 83-image batch that meant 332 requests, which saturated
// the per-user global rate limiter (default 100/min) within seconds and
// silently dropped the rest. The bulk endpoint collapses each file to one
// rate-limit token, so a 4-chunk batch costs four tokens total.
//
// onProgress is called once per completed chunk with the total number of
// files attempted so far — matching the legacy progress-bar contract that
// callers wired against (done/total counts, last-touched filename).
export async function bulkUploadImageItems(
  packId: string,
  files: File[],
  onProgress?: (done: number, total: number, currentName: string) => void
): Promise<BulkUploadOutcome> {
  const outcome: BulkUploadOutcome = { succeeded: [], failed: [] };
  for (let i = 0; i < files.length; i += BULK_UPLOAD_CHUNK_SIZE) {
    const chunk = files.slice(i, i + BULK_UPLOAD_CHUNK_SIZE);
    onProgress?.(i, files.length, chunk[0]?.name ?? '');
    const res = await uploadOneBulkChunk(packId, chunk);
    if ('error' in res) {
      // Whole-chunk failure (auth, network, rate-limit exhaustion, parse
      // error). Mark every file in the chunk as failed with the same
      // reason so the caller's summary toast reflects reality.
      for (const f of chunk) {
        outcome.failed.push({ filename: f.name, reason: res.error });
      }
      continue;
    }
    for (const r of res.results) {
      if (r.ok && r.item) outcome.succeeded.push(r.item);
      else outcome.failed.push({ filename: r.filename, reason: r.reason ?? r.code ?? 'upload failed' });
    }
    onProgress?.(Math.min(i + chunk.length, files.length), files.length, chunk[chunk.length - 1]?.name ?? '');
  }
  return outcome;
}

// ── Bulk text import via JSON ─────────────────────────────────────────────
// Accepted shape: a top-level array of `{ name, text }` objects. Anything else
// (object wrapper, missing fields, wrong types) is rejected with a precise
// reason so the caller can show one toast per row, matching the image flow.
export interface ParsedTextItem {
  name: string;
  text: string;
}

export type ParsedTextItems =
  | { ok: true; items: ParsedTextItem[] }
  | { ok: false; error: string };

export function parseTextItemsJson(raw: string): ParsedTextItems {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (err) {
    return { ok: false, error: `invalid JSON: ${err instanceof Error ? err.message : String(err)}` };
  }
  if (!Array.isArray(parsed)) {
    return { ok: false, error: 'expected a top-level JSON array of { name, text } objects' };
  }
  const items: ParsedTextItem[] = [];
  for (let i = 0; i < parsed.length; i++) {
    const row = parsed[i];
    if (!row || typeof row !== 'object') {
      return { ok: false, error: `entry ${i + 1}: expected an object with name and text` };
    }
    const name = (row as { name?: unknown }).name;
    const text = (row as { text?: unknown }).text;
    if (typeof name !== 'string' || !name.trim()) {
      return { ok: false, error: `entry ${i + 1}: name is required` };
    }
    if (typeof text !== 'string') {
      return { ok: false, error: `entry ${i + 1} ("${name}"): text must be a string` };
    }
    items.push({ name: name.trim(), text });
  }
  return { ok: true, items };
}

// Must match the backend MaxBulkTextItems cap in
// backend/internal/api/items_bulk_text.go. The server rejects requests above
// this with 413 too_many_items; the chunker keeps each request inside the cap.
const BULK_TEXT_CHUNK_SIZE = 100;

interface BulkTextServerResult {
  ok: boolean;
  filename: string; // server reuses the bulk shape — for text items this is the row name
  item?: GameItem;
  reason?: string;
  code?: string;
}

// uploadOneTextChunk POSTs a JSON array of {name, text} rows in one request,
// with bounded exponential-backoff retry on 429 — mirroring the image-bulk
// chunk uploader. The endpoint is per-item-transactional, so a partial
// per-item failure cannot leak orphan rows; retries therefore only fire
// when the whole request was rate-limited before any work began.
async function uploadOneTextChunk(
  packId: string,
  items: ParsedTextItem[]
): Promise<{ results: BulkTextServerResult[] } | { error: string }> {
  const delays = [1000, 2000, 4000];
  for (let attempt = 0; attempt <= delays.length; attempt++) {
    let res: Response;
    try {
      res = await fetch(`/api/packs/${packId}/items/bulk-text`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items })
      });
    } catch (err) {
      return { error: err instanceof Error ? err.message : String(err) };
    }
    if (res.status === 429 && attempt < delays.length) {
      await new Promise((resolve) => setTimeout(resolve, delays[attempt]));
      continue;
    }
    if (!res.ok) {
      const raw = await res.text().catch(() => '');
      let detail = raw;
      try {
        const parsed = JSON.parse(raw);
        if (parsed?.error) detail = parsed.error;
      } catch {}
      return { error: `${res.status} ${res.statusText}${detail ? ` — ${detail}` : ''}` };
    }
    return (await res.json()) as { results: BulkTextServerResult[] };
  }
  return { error: 'rate limited (429) — retried 4 times' };
}

// bulkUploadTextItems chunks an arbitrarily large parsed JSON list into
// MaxBulkTextItems-sized batches and uploads each batch in a single HTTP
// request. Pre-fix this function ran three sequential round-trips per item;
// for a 131-item bulk import that meant 393 requests, which saturated the
// per-user global rate limiter (default 100/min) within seconds and
// silently dropped the rest. The bulk-text endpoint collapses each item to
// one rate-limit token, so a 2-chunk batch costs two tokens total.
export async function bulkUploadTextItems(
  packId: string,
  items: ParsedTextItem[],
  onProgress?: (done: number, total: number, currentName: string) => void
): Promise<BulkUploadOutcome> {
  const outcome: BulkUploadOutcome = { succeeded: [], failed: [] };
  for (let i = 0; i < items.length; i += BULK_TEXT_CHUNK_SIZE) {
    const chunk = items.slice(i, i + BULK_TEXT_CHUNK_SIZE);
    onProgress?.(i, items.length, chunk[0]?.name ?? '');
    const res = await uploadOneTextChunk(packId, chunk);
    if ('error' in res) {
      for (const it of chunk) {
        outcome.failed.push({ filename: it.name, reason: res.error });
      }
      continue;
    }
    for (const r of res.results) {
      if (r.ok && r.item) outcome.succeeded.push(r.item);
      else outcome.failed.push({ filename: r.filename, reason: r.reason ?? r.code ?? 'import failed' });
    }
    onProgress?.(Math.min(i + chunk.length, items.length), items.length, chunk[chunk.length - 1]?.name ?? '');
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
