// Per-tab guest identity store. Guests are ephemeral room participants — no
// account, no cross-room token. Keyed by room code so each room has its own
// guest session record that survives reloads within the same browser tab.
//
// Persistence lives in sessionStorage:
//   - scoped to the tab (a new tab cannot inherit another tab's guest)
//   - survives page reloads (so the WS can re-authenticate on reconnect)
//   - cleared when the tab closes (the backend guest_players row is cleaned
//     up on room reaper, not on tab close)

export interface GuestRecord {
  player_id: string;
  display_name: string;
  token: string;
}

const STORAGE_PREFIX = 'fdym:guest:';

function keyFor(code: string): string {
  return `${STORAGE_PREFIX}${code.toUpperCase()}`;
}

class GuestStateClass {
  get(code: string): GuestRecord | null {
    if (typeof window === 'undefined') return null;
    const raw = sessionStorage.getItem(keyFor(code));
    if (!raw) return null;
    try {
      return JSON.parse(raw) as GuestRecord;
    } catch {
      return null;
    }
  }

  set(code: string, rec: GuestRecord): void {
    if (typeof window === 'undefined') return;
    sessionStorage.setItem(keyFor(code), JSON.stringify(rec));
  }

  clear(code: string): void {
    if (typeof window === 'undefined') return;
    sessionStorage.removeItem(keyFor(code));
  }

  token(code: string): string | null {
    return this.get(code)?.token ?? null;
  }

  playerId(code: string): string | null {
    return this.get(code)?.player_id ?? null;
  }
}

export const guest = new GuestStateClass();
