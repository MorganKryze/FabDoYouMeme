/**
 * Theme preference state.
 *
 * Reconciles three inputs into one output:
 *   - `preference`: user's explicit choice (auto/light/dark)
 *   - `timeOfDay`:  clock-driven band (light/dark)
 *   - `active`:     derived — preference if not 'auto', otherwise timeOfDay
 *
 * `preference` is persisted to localStorage under 'fdym:theme'.
 * `tickTimeOfDay()` should be called on mount and periodically (e.g.
 * every 5 minutes) from the root layout.
 */

export type Band = 'light' | 'dark';
export type ThemePref = 'auto' | Band;

const STORAGE_KEY = 'fdym:theme';
const VALID_PREFS: readonly ThemePref[] = ['auto', 'light', 'dark'] as const;

function isValidPref(v: unknown): v is ThemePref {
  return typeof v === 'string' && (VALID_PREFS as readonly string[]).includes(v);
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

export class ThemeState {
  preference = $state<ThemePref>('auto');
  timeOfDay = $state<Band>('light');

  active = $derived<Band>(
    this.preference === 'auto' ? this.timeOfDay : this.preference
  );

  constructor() {
    const storage = safeStorage();
    if (storage) {
      const saved = storage.getItem(STORAGE_KEY);
      if (isValidPref(saved)) this.preference = saved;
    }
    this.tickTimeOfDay();
  }

  setPreference(p: ThemePref): void {
    this.preference = p;
    const storage = safeStorage();
    if (storage) {
      storage.setItem(STORAGE_KEY, p);
    }
  }

  tickTimeOfDay(): void {
    const h = new Date().getHours();
    this.timeOfDay = h >= 7 && h < 19 ? 'light' : 'dark';
  }
}

export const theme = new ThemeState();
