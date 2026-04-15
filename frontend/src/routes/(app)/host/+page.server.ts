import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { GameType } from '$lib/api/types';
import { API_BASE, apiFetch } from '$lib/server/backend';

type ActiveRoomResponse = {
  room: { code: string } | null;
};

export const load: PageServerLoad = async ({ fetch, url }) => {
  // Single-room enforcement: if the caller already has an active room, bounce
  // them back to /home which will display the "Return to your room" card.
  // Mirrors the gate in backend/internal/api/rooms.go RoomHandler.Create.
  const activeRes = await fetch(`${API_BASE}/api/users/me/active-room`);
  if (activeRes.ok) {
    const { room } = (await activeRes.json()) as ActiveRoomResponse;
    if (room) throw redirect(303, '/home');
  }
  const gameTypes = await apiFetch<GameType[]>(fetch, '/api/game-types');
  const preselectedSlug = url.searchParams.get('game_type') ?? '';
  const preselected = gameTypes.find((gt) => gt.slug === preselectedSlug) ?? null;
  return { gameTypes, preselectedSlug, preselectedId: preselected?.id ?? '' };
};

export const actions: Actions = {
  createRoom: async ({ request, fetch }) => {
    const data = await request.formData();
    const game_type_id = data.get('game_type_id') as string;
    const pack_id = data.get('pack_id') as string;
    const is_solo = data.get('is_solo') === 'true';

    // Defaults are placeholders — host tunes rounds/durations inside the
    // room's waiting stage (F3). Server still accepts a config because
    // /api/rooms expects one; these match the meme_caption defaults.
    const res = await fetch(`${API_BASE}/api/rooms`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        game_type_id,
        pack_id,
        is_solo,
        config: {
          round_count: 5,
          round_duration_seconds: 60,
          voting_duration_seconds: 30
        }
      })
    });

    if (!res.ok) {
      let code = 'unknown';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      const messages: Record<string, string> = {
        pack_no_supported_items:
          'This pack has no items compatible with the selected game type.',
        pack_insufficient_items:
          'This pack does not have enough items for the selected round count.',
        invalid_game_type: 'Invalid game type selected.',
        already_in_active_room:
          "You're already in a room — return to it or leave it first."
      };
      return fail(400, {
        error: messages[code] ?? 'Could not create room. Try again.'
      });
    }

    const body = await res.json();
    throw redirect(303, `/rooms/${body.code}`);
  }
};
