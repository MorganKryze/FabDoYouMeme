import type { GameType, RequiredPack } from '$lib/api/types';
import { gameTypesApi } from '$lib/api/game-types';
import {
  payloadVersionForKind,
  type PackKind
} from './studio.svelte';

// GameTypesRegistry caches the /api/game-types response and exposes the
// lookups the studio (and host page) need: which game types accept a given
// pack kind, and the worst-case items required for a pack of that kind.
//
// Loaded once per session via `ensureLoaded()`; refresh() force-reloads.
export class GameTypesRegistry {
  types = $state<GameType[]>([]);
  loaded = $state(false);
  loading = $state(false);

  private inflight: Promise<void> | null = null;

  async ensureLoaded(): Promise<void> {
    if (this.loaded) return;
    if (this.inflight) return this.inflight;
    this.inflight = this.refresh();
    try {
      await this.inflight;
    } finally {
      this.inflight = null;
    }
  }

  async refresh(): Promise<void> {
    this.loading = true;
    try {
      this.types = await gameTypesApi.list();
      this.loaded = true;
    } finally {
      this.loading = false;
    }
  }

  /** Replace the cache (test seam). */
  setTypes(types: GameType[]) {
    this.types = types;
    this.loaded = true;
  }

  /** Game types that accept items of this kind in *any* of their roles. */
  compatibleGameTypes(kind: PackKind): GameType[] {
    const version = payloadVersionForKind(kind);
    return this.types.filter(t =>
      t.required_packs.some(p => p.payload_versions.includes(version))
    );
  }

  /** Slugs only — convenience for badges. */
  compatibleGameTypeSlugs(kind: PackKind): string[] {
    return this.compatibleGameTypes(kind).map(t => t.slug);
  }

  /**
   * Worst-case items required for a pack of `kind` to satisfy the largest
   * room of any compatible game type. Mirrors the backend `MinItemsFn` math:
   *
   *   primary role (image/prompt): round_count
   *   secondary role (text/filler): hand_size × max_players
   *                                 + (round_count − 1) × max_players
   *
   * Returns 0 when no game type consumes this kind (defensive).
   */
  worstCaseItemsNeeded(kind: PackKind): number {
    const version = payloadVersionForKind(kind);
    let max = 0;
    for (const gt of this.types) {
      const roleIndex = gt.required_packs.findIndex(p =>
        p.payload_versions.includes(version)
      );
      if (roleIndex < 0) continue;
      const need = computeMinItems(gt, roleIndex);
      if (need > max) max = need;
    }
    return max;
  }

  /** All required-pack roles a game type consumes, in declaration order. */
  rolesFor(slug: string): RequiredPack[] {
    return this.types.find(t => t.slug === slug)?.required_packs ?? [];
  }
}

// Standalone so tests can call it without instantiating the registry.
export function computeMinItems(gt: GameType, roleIndex: number): number {
  const cfg = gt.config;
  const roundCount = cfg.default_round_count;
  const maxPlayers = cfg.max_players ?? 12;
  const handSize = cfg.default_hand_size ?? 0;

  // Primary role (index 0) consumes one item per round — the prompt/image
  // shown to everyone. Secondary roles (index >= 1) are dealt as a hand.
  if (roleIndex === 0) {
    return roundCount;
  }
  // No hand → secondary role isn't actually deck-based; fall back to
  // round_count to stay defensive (handler would in practice not declare
  // a secondary role without hand-size bounds).
  if (handSize === 0) return roundCount;

  const refills = roundCount > 1 ? (roundCount - 1) * maxPlayers : 0;
  return handSize * maxPlayers + refills;
}

export const gameTypes = new GameTypesRegistry();
