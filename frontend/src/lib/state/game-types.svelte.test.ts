import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  GameTypesRegistry,
  computeMinItems
} from './game-types.svelte';
import type { GameType } from '$lib/api/types';

function gt(overrides: Partial<GameType> & Pick<GameType, 'slug'>): GameType {
  return {
    id: `id-${overrides.slug}`,
    slug: overrides.slug,
    name: overrides.slug,
    description: null,
    version: '1.0.0',
    supports_solo: false,
    config: {
      min_round_duration_seconds: 15,
      max_round_duration_seconds: 300,
      default_round_duration_seconds: 45,
      min_voting_duration_seconds: 10,
      max_voting_duration_seconds: 120,
      default_voting_duration_seconds: 30,
      min_players: 2,
      max_players: 12,
      min_round_count: 1,
      max_round_count: 30,
      default_round_count: 8,
      ...(overrides.config ?? {})
    },
    required_packs: overrides.required_packs ?? []
  };
}

const memeFreestyle = gt({
  slug: 'meme-freestyle',
  required_packs: [{ role: 'image', payload_versions: [1] }]
});

const memeShowdown = gt({
  slug: 'meme-showdown',
  config: {
    min_round_duration_seconds: 15,
    max_round_duration_seconds: 300,
    default_round_duration_seconds: 45,
    min_voting_duration_seconds: 10,
    max_voting_duration_seconds: 120,
    default_voting_duration_seconds: 30,
    min_players: 2,
    max_players: 12,
    min_round_count: 1,
    max_round_count: 30,
    default_round_count: 8,
    min_hand_size: 3,
    max_hand_size: 7,
    default_hand_size: 4
  },
  required_packs: [
    { role: 'image', payload_versions: [1] },
    { role: 'text', payload_versions: [2] }
  ]
});

const promptFreestyle = gt({
  slug: 'prompt-freestyle',
  required_packs: [{ role: 'prompt', payload_versions: [4] }]
});

const promptShowdown = gt({
  slug: 'prompt-showdown',
  config: {
    min_round_duration_seconds: 15,
    max_round_duration_seconds: 300,
    default_round_duration_seconds: 45,
    min_voting_duration_seconds: 10,
    max_voting_duration_seconds: 120,
    default_voting_duration_seconds: 30,
    min_players: 2,
    max_players: 12,
    min_round_count: 1,
    max_round_count: 30,
    default_round_count: 8,
    min_hand_size: 3,
    max_hand_size: 7,
    default_hand_size: 4
  },
  required_packs: [
    { role: 'prompt', payload_versions: [4] },
    { role: 'filler', payload_versions: [3] }
  ]
});

const ALL = [memeFreestyle, memeShowdown, promptFreestyle, promptShowdown];

describe('GameTypesRegistry', () => {
  let r: GameTypesRegistry;

  beforeEach(() => {
    r = new GameTypesRegistry();
    r.setTypes(ALL);
  });

  it('compatibleGameTypeSlugs(image) returns both meme variants', () => {
    expect(r.compatibleGameTypeSlugs('image').sort()).toEqual([
      'meme-freestyle',
      'meme-showdown'
    ]);
  });

  it('compatibleGameTypeSlugs(text) returns only meme-showdown', () => {
    expect(r.compatibleGameTypeSlugs('text')).toEqual(['meme-showdown']);
  });

  it('compatibleGameTypeSlugs(filler) returns only prompt-showdown', () => {
    expect(r.compatibleGameTypeSlugs('filler')).toEqual(['prompt-showdown']);
  });

  it('compatibleGameTypeSlugs(prompt) returns both prompt variants', () => {
    expect(r.compatibleGameTypeSlugs('prompt').sort()).toEqual([
      'prompt-freestyle',
      'prompt-showdown'
    ]);
  });

  it('worstCaseItemsNeeded(image) = max default round count across consumers', () => {
    // Both meme-freestyle and meme-showdown default to 8 rounds.
    expect(r.worstCaseItemsNeeded('image')).toBe(8);
  });

  it('worstCaseItemsNeeded(text) = hand_size×max + (rounds-1)×max', () => {
    // meme-showdown: 4×12 + (8-1)×12 = 48 + 84 = 132
    expect(r.worstCaseItemsNeeded('text')).toBe(132);
  });

  it('worstCaseItemsNeeded(filler) = same shape as text', () => {
    // prompt-showdown: 4×12 + 7×12 = 132
    expect(r.worstCaseItemsNeeded('filler')).toBe(132);
  });

  it('worstCaseItemsNeeded(prompt) = round count', () => {
    expect(r.worstCaseItemsNeeded('prompt')).toBe(8);
  });

  it('rolesFor returns the declared roles in order', () => {
    expect(r.rolesFor('meme-showdown').map(p => p.role)).toEqual([
      'image',
      'text'
    ]);
    expect(r.rolesFor('prompt-showdown').map(p => p.role)).toEqual([
      'prompt',
      'filler'
    ]);
  });

  it('rolesFor returns [] for an unknown slug', () => {
    expect(r.rolesFor('does-not-exist')).toEqual([]);
  });
});

describe('computeMinItems', () => {
  it('primary role uses round_count', () => {
    expect(computeMinItems(memeShowdown, 0)).toBe(8);
  });

  it('secondary role uses hand × max + (rounds-1) × max', () => {
    expect(computeMinItems(memeShowdown, 1)).toBe(4 * 12 + 7 * 12);
  });

  it('round_count=1 produces no refills', () => {
    const single = gt({
      slug: 'one-round',
      config: {
        min_round_duration_seconds: 15,
        max_round_duration_seconds: 300,
        default_round_duration_seconds: 45,
        min_voting_duration_seconds: 10,
        max_voting_duration_seconds: 120,
        default_voting_duration_seconds: 30,
        min_players: 2,
        max_players: 12,
        min_round_count: 1,
        max_round_count: 30,
        default_round_count: 1,
        min_hand_size: 3,
        max_hand_size: 7,
        default_hand_size: 4
      },
      required_packs: [
        { role: 'image', payload_versions: [1] },
        { role: 'text', payload_versions: [2] }
      ]
    });
    expect(computeMinItems(single, 1)).toBe(4 * 12);
  });
});

describe('GameTypesRegistry.ensureLoaded', () => {
  it('coalesces concurrent calls to a single fetch', async () => {
    const r = new GameTypesRegistry();
    const refresh = vi.spyOn(r, 'refresh').mockImplementation(async () => {
      r.setTypes(ALL);
    });
    await Promise.all([r.ensureLoaded(), r.ensureLoaded(), r.ensureLoaded()]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(r.types).toHaveLength(ALL.length);
  });

  it('does not re-fetch once loaded', async () => {
    const r = new GameTypesRegistry();
    r.setTypes(ALL);
    const refresh = vi.spyOn(r, 'refresh');
    await r.ensureLoaded();
    expect(refresh).not.toHaveBeenCalled();
  });
});
