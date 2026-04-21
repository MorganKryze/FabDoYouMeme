// frontend/src/lib/content/toneSelect.ts

import { getLocale, type Locale } from '$lib/paraglide/runtime';
import { tonePools, type SlotId, type ToneLevel, type TonePair } from './tonePools';

/**
 * Pick one tone pair for a given slot, level, and locale.
 *
 * - Locale defaults to the current paraglide locale.
 * - If `lastSeen` is provided and the random pick matches it exactly,
 *   re-roll once. Only once — two consecutive identical picks across
 *   two visits with a re-roll in between is an acceptable rare outcome,
 *   and a loop-until-different approach risks infinite loops when a
 *   bucket collapses to a single entry.
 * - If the requested bucket is empty (should not happen with authored
 *   content), fall back to the Playful bucket (level 2) of the same locale.
 */
export function pickForSlot(
  slot: SlotId,
  level: ToneLevel,
  lastSeen?: TonePair | null,
  locale: Locale = getLocale(),
): TonePair {
  const bucket = tonePools[locale][slot][level];
  if (bucket.length === 0) {
    const fallback = tonePools[locale][slot][2];
    return fallback[Math.floor(Math.random() * fallback.length)];
  }
  if (bucket.length === 1) return bucket[0];

  let pick = bucket[Math.floor(Math.random() * bucket.length)];
  if (lastSeen && pick.h1 === lastSeen.h1 && pick.subline === lastSeen.subline) {
    pick = bucket[Math.floor(Math.random() * bucket.length)];
  }
  return pick;
}
