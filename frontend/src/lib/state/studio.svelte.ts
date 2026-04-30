import type { Pack, GameItem, ItemVersion } from '$lib/api/types';

export interface StudioGroup {
  id: string;
  name: string;
  classification: 'sfw' | 'nsfw';
  language: 'en' | 'fr' | 'multi';
  // Caller's role in the group — drives rename / delete visibility in the
  // navigator. Mirrors GroupListItem.member_role from /api/groups.
  member_role: 'admin' | 'member';
}

// PackKind mirrors a pack's expected item-payload version:
//   image  → payload_version 1
//   text   → payload_version 2 (meme captions)
//   filler → payload_version 3 (sentence-blank fillers)
//   prompt → payload_version 4 (sentence with blank, { prefix, suffix })
export type PackKind = 'image' | 'text' | 'filler' | 'prompt';

export function payloadVersionForKind(kind: PackKind): number {
  switch (kind) {
    case 'image': return 1;
    case 'text': return 2;
    case 'filler': return 3;
    case 'prompt': return 4;
  }
}

export function kindFromPayloadVersion(version: number): PackKind {
  switch (version) {
    case 4: return 'prompt';
    case 3: return 'filler';
    case 2: return 'text';
    default: return 'image';
  }
}

// Persisted in localStorage so the user's choice on the new-pack form survives
// reloads. Without persistence, refreshing while a freshly-created text pack
// is empty drops the intent and the studio falls back to 'image' — exactly
// the bug the user hit. Cross-device persistence would need a backend column;
// for a single browser this is enough.
const STORAGE_KEY = 'fdym:studio:intendedKind';

function loadIntendedKind(): Record<string, PackKind> {
  if (typeof localStorage === 'undefined') return {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object') return {};
    const out: Record<string, PackKind> = {};
    for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (v === 'image' || v === 'text' || v === 'filler' || v === 'prompt') {
        out[k] = v;
      }
    }
    return out;
  } catch {
    return {};
  }
}

function saveIntendedKind(map: Record<string, PackKind>) {
  if (typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
  } catch {
    /* quota / disabled — best-effort */
  }
}

class StudioState {
  selectedPackId = $state<string | null>(null);
  selectedItemId = $state<string | null>(null);
  /** Up to 2 version IDs for side-by-side comparison */
  selectedVersionIds = $state<string[]>([]);

  packs = $state<Pack[]>([]);
  items = $state<GameItem[]>([]);
  versions = $state<ItemVersion[]>([]);
  // Groups the user is a member of, each with its duplicated packs. Rendered
  // as distinct sections in PackNavigator so group packs don't leak into the
  // "Official" / "Mine" buckets.
  groups = $state<StudioGroup[]>([]);

  // Intent for newly created packs that don't yet have items. Once a pack
  // has items, kindFor() infers kind from items[0].payload_version and
  // ignores this map. Persisted to localStorage so reloads don't lose it.
  intendedKind = $state<Record<string, PackKind>>(loadIntendedKind());

  selectPack(packId: string) {
    this.selectedPackId = packId;
    this.selectedItemId = null;
    this.selectedVersionIds = [];
    this.items = [];
    this.versions = [];
  }

  selectItem(itemId: string) {
    this.selectedItemId = itemId;
    this.selectedVersionIds = [];
    this.versions = [];
  }

  toggleVersionSelection(versionId: string) {
    if (this.selectedVersionIds.includes(versionId)) {
      this.selectedVersionIds = this.selectedVersionIds.filter(
        id => id !== versionId
      );
    } else if (this.selectedVersionIds.length < 2) {
      this.selectedVersionIds = [...this.selectedVersionIds, versionId];
    }
  }

  rememberKind(packId: string, kind: PackKind) {
    this.intendedKind = { ...this.intendedKind, [packId]: kind };
    saveIntendedKind(this.intendedKind);
  }

  forgetKind(packId: string) {
    if (!(packId in this.intendedKind)) return;
    const { [packId]: _, ...rest } = this.intendedKind;
    this.intendedKind = rest;
    saveIntendedKind(this.intendedKind);
  }

  kindFor(packId: string | null): PackKind {
    if (!packId) return 'image';
    if (this.selectedPackId === packId && this.items.length > 0) {
      return kindFromPayloadVersion(this.items[0].payload_version);
    }
    return this.intendedKind[packId] ?? 'image';
  }

  reset() {
    this.selectedPackId = null;
    this.selectedItemId = null;
    this.selectedVersionIds = [];
    this.packs = [];
    this.items = [];
    this.versions = [];
    this.groups = [];
    this.intendedKind = {};
    saveIntendedKind(this.intendedKind);
  }
}

export const studio = new StudioState();
