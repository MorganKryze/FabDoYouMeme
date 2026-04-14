import { describe, it, expect } from 'vitest';
import { computeMedals, formatMakerSince } from './medals';
import type { HistoryRoom } from '../routes/(app)/home/+page.server';

const baseUser = { created_at: '2026-01-15T09:30:00Z' };

function makeRoom(rank: number): HistoryRoom {
  return {
    code: 'ABCD',
    game_type_slug: 'meme-caption',
    pack_name: 'Pack',
    started_at: '2026-02-01T00:00:00Z',
    score: 0,
    rank,
    player_count: 4,
  };
}

describe('computeMedals', () => {
  it('always grants Welcomed, even with no history', () => {
    const medals = computeMedals(baseUser, []);
    const welcomed = medals.find((m) => m.id === 'welcomed');
    expect(welcomed?.earned).toBe(true);
  });

  it('returns exactly four medals in a fixed order', () => {
    const medals = computeMedals(baseUser, []);
    expect(medals.map((m) => m.id)).toEqual([
      'welcomed',
      'first-game',
      'first-win',
      'veteran',
    ]);
  });

  it('locks First Game when no history exists', () => {
    const medals = computeMedals(baseUser, []);
    expect(medals.find((m) => m.id === 'first-game')?.earned).toBe(false);
  });

  it('earns First Game after at least one room', () => {
    const medals = computeMedals(baseUser, [makeRoom(3)]);
    expect(medals.find((m) => m.id === 'first-game')?.earned).toBe(true);
  });

  it('locks First Win when no room was won', () => {
    const medals = computeMedals(baseUser, [makeRoom(2), makeRoom(3)]);
    expect(medals.find((m) => m.id === 'first-win')?.earned).toBe(false);
  });

  it('earns First Win as soon as any room has rank 1', () => {
    const medals = computeMedals(baseUser, [makeRoom(2), makeRoom(1)]);
    expect(medals.find((m) => m.id === 'first-win')?.earned).toBe(true);
  });

  it('locks Veteran below ten rooms', () => {
    const history = Array.from({ length: 9 }, () => makeRoom(2));
    const medals = computeMedals(baseUser, history);
    expect(medals.find((m) => m.id === 'veteran')?.earned).toBe(false);
  });

  it('earns Veteran at exactly ten rooms', () => {
    const history = Array.from({ length: 10 }, () => makeRoom(2));
    const medals = computeMedals(baseUser, history);
    expect(medals.find((m) => m.id === 'veteran')?.earned).toBe(true);
  });
});

describe('formatMakerSince', () => {
  it('returns a short-month year string', () => {
    const out = formatMakerSince('2026-04-14T00:00:00Z');
    expect(out).toMatch(/\w{3,}\s+2026/);
  });

  it('does not crash on an invalid date string', () => {
    expect(() => formatMakerSince('not-a-date')).not.toThrow();
  });
});
