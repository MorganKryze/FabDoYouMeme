/**
 * Theme preference state.
 *
 * Reconciles three inputs into one output:
 *   - `preference`: user's explicit choice (auto/morning/evening/night)
 *   - `timeOfDay`:  clock-driven band (morning/evening/night)
 *   - `active`:     derived — preference if not 'auto', otherwise timeOfDay
 *
 * `preference` is persisted to localStorage under 'fdym:theme'.
 * `tickTimeOfDay()` should be called on mount and periodically (e.g.
 * every 5 minutes) from the root layout.
 */

export type Band = 'morning' | 'evening' | 'night';
export type ThemePref = 'auto' | Band;

const STORAGE_KEY = 'fdym:theme';
const VALID_PREFS: readonly ThemePref[] = ['auto', 'morning', 'evening', 'night'] as const;

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
  timeOfDay = $state<Band>('morning');

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
    if (h >= 6 && h < 17) {
      this.timeOfDay = 'morning';
    } else if (h >= 17 && h < 21) {
      this.timeOfDay = 'evening';
    } else {
      this.timeOfDay = 'night';
    }
  }
}

export const theme = new ThemeState();
