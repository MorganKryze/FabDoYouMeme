// frontend/src/lib/state/tone.svelte.ts

/**
 * Tone preference state.
 *
 * Mirrors `theme.svelte.ts` structurally — a class with a `$state` field,
 * exported as a singleton. Hydrates from localStorage synchronously in the
 * constructor so the first-paint read of `tone.level` is already correct,
 * preventing greeting flash on /home.
 */

import { DEFAULT_TONE, type ToneLevel } from '$lib/content/tonePools';

const STORAGE_KEY = 'fdym:tone';
const VALID_LEVELS: readonly ToneLevel[] = [0, 1, 2, 3, 4] as const;

function isValidLevel(v: unknown): v is ToneLevel {
  return (
    typeof v === 'number' &&
    (VALID_LEVELS as readonly number[]).includes(v)
  );
}

function safeStorage(): Storage | null {
  try {
    if (typeof localStorage === 'undefined') return null;
    if (typeof localStorage.getItem !== 'function') return null;
    return localStorage;
  } catch {
    return null;
  }
}

export class ToneState {
  level = $state<ToneLevel>(DEFAULT_TONE);

  constructor() {
    const storage = safeStorage();
    if (storage) {
      const raw = storage.getItem(STORAGE_KEY);
      const parsed = raw === null ? NaN : Number(raw);
      if (isValidLevel(parsed)) this.level = parsed;
    }
  }

  setLevel(next: ToneLevel): void {
    this.level = next;
    const storage = safeStorage();
    if (storage) {
      storage.setItem(STORAGE_KEY, String(next));
    }
  }
}

export const tone = new ToneState();
