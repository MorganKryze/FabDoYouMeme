import { describe, it, expect, vi, afterEach } from 'vitest';
import { pickForSlot } from './toneSelect';
import { tonePools, type TonePair, type ToneLevel } from './tonePools';

describe('pickForSlot', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns a pair from the requested bucket', () => {
    const pair = pickForSlot('home_greeting', 0);
    const bucket = tonePools.home_greeting[0];
    expect(bucket).toContainEqual(pair);
  });

  it('returns different buckets for different levels', () => {
    // Force Math.random() to index 0 so results are deterministic.
    vi.spyOn(Math, 'random').mockReturnValue(0);
    const cozy = pickForSlot('home_greeting', 0);
    const chaos = pickForSlot('home_greeting', 4);
    expect(cozy).toEqual(tonePools.home_greeting[0][0]);
    expect(chaos).toEqual(tonePools.home_greeting[4][0]);
    expect(cozy).not.toEqual(chaos);
  });

  it('re-rolls once when the picked pair equals lastSeen', () => {
    const bucket = tonePools.home_greeting[2];
    const first = bucket[0];
    const second = bucket[1];

    // First call to Math.random() returns 0 → picks index 0 (first).
    // Second call returns 0.2 → picks index floor(0.2 * 8) = 1 (second).
    const spy = vi.spyOn(Math, 'random');
    spy.mockReturnValueOnce(0);
    spy.mockReturnValueOnce(0.2);

    const pick = pickForSlot('home_greeting', 2, first);
    expect(pick).toEqual(second);
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('accepts the re-roll result even if it matches lastSeen again', () => {
    const bucket = tonePools.home_greeting[2];
    const first = bucket[0];

    // Both random calls return 0 → both picks index 0. The second one
    // is shipped anyway to prevent infinite loops.
    const spy = vi.spyOn(Math, 'random');
    spy.mockReturnValue(0);

    const pick = pickForSlot('home_greeting', 2, first);
    expect(pick).toEqual(first);
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('returns the only entry when the bucket has length 1', () => {
    // Temporarily patch a bucket to length 1 for this case.
    const original = tonePools.home_greeting[0];
    const sole: TonePair = { h1: 'only', subline: 'one' };
    (tonePools.home_greeting as Record<ToneLevel, TonePair[]>)[0] = [sole];
    try {
      const pick = pickForSlot('home_greeting', 0, sole);
      expect(pick).toEqual(sole);
    } finally {
      (tonePools.home_greeting as Record<ToneLevel, TonePair[]>)[0] = original;
    }
  });

  it('falls back to Playful bucket when requested bucket is empty', () => {
    const original = tonePools.home_greeting[4];
    (tonePools.home_greeting as Record<ToneLevel, TonePair[]>)[4] = [];
    try {
      vi.spyOn(Math, 'random').mockReturnValue(0);
      const pick = pickForSlot('home_greeting', 4);
      expect(pick).toEqual(tonePools.home_greeting[2][0]);
    } finally {
      (tonePools.home_greeting as Record<ToneLevel, TonePair[]>)[4] = original;
    }
  });
});
