import type { Pack } from '$lib/api/types';

interface Item {
  id: string;
  position: number;
  payload_version: number;
  current_version_id: string | null;
  media_key?: string | null;
  payload?: unknown;
}

interface ItemVersion {
  id: string;
  item_id: string;
  version_number: number;
  media_key: string | null;
  payload: unknown;
  created_at: string;
  deleted_at: string | null;
}

class StudioState {
  selectedPackId = $state<string | null>(null);
  selectedItemId = $state<string | null>(null);
  /** Up to 2 version IDs for side-by-side comparison */
  selectedVersionIds = $state<string[]>([]);

  packs = $state<Pack[]>([]);
  items = $state<Item[]>([]);
  versions = $state<ItemVersion[]>([]);

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

  reset() {
    this.selectedPackId = null;
    this.selectedItemId = null;
    this.selectedVersionIds = [];
    this.packs = [];
    this.items = [];
    this.versions = [];
  }
}

export const studio = new StudioState();
