import { error, type NumericRange } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import type { GameType, Room } from '$lib/api/types';
import { API_BASE, apiFetch } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ params, fetch }) => {
  const res = await fetch(`${API_BASE}/api/rooms/${params.code}`);
  if (!res.ok) {
    // Preserve the real backend status so debugging isn't misled. A 401/403
    // flattened to 404 "not found" hides auth misconfiguration — we got burned
    // by that in the guest-join flow where the backend was returning 401 but
    // the layout reported "Room not found" to the user.
    const msg =
      res.status === 404
        ? `Room ${params.code} not found`
        : `Could not load room ${params.code} (${res.status})`;
    throw error(res.status as NumericRange<400, 599>, msg);
  }
  const roomData = (await res.json()) as Room;

  // The /api/rooms/:code endpoint returns only game_type_slug. Resolve the
  // full game type object here so room.init() gets the shape it expects —
  // without this, room.gameType stays null and UI reads as "unrecognized".
  let gameType: GameType | null = null;
  try {
    gameType = await apiFetch<GameType>(
      fetch,
      `/api/game-types/${roomData.game_type_slug}`
    );
  } catch (e) {
    console.error(
      `[rooms/${params.code}] failed to resolve game_type ${roomData.game_type_slug}`,
      e
    );
  }

  return { room: { ...roomData, game_type: gameType } };
};
