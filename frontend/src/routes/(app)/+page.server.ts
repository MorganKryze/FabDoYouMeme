import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { GameType, Pack } from '$lib/api/types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch(`${API_BASE}/api/game-types`);
  const gameTypes: GameType[] = res.ok ? await res.json() : [];
  return { gameTypes };
};

export const actions: Actions = {
  createRoom: async ({ request, fetch }) => {
    const data = await request.formData();
    const game_type_id = data.get('game_type_id') as string;
    const pack_id = data.get('pack_id') as string;
    const is_solo = data.get('is_solo') === 'true';
    const round_count = Number(data.get('round_count') ?? 5);
    const round_duration_seconds = Number(
      data.get('round_duration_seconds') ?? 60
    );
    const voting_duration_seconds = Number(
      data.get('voting_duration_seconds') ?? 30
    );

    const res = await fetch(`${API_BASE}/api/rooms`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        game_type_id,
        pack_id,
        is_solo,
        config: { round_count, round_duration_seconds, voting_duration_seconds }
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
        invalid_game_type: 'Invalid game type selected.'
      };
      return fail(400, {
        error: messages[code] ?? 'Could not create room. Try again.'
      });
    }

    const body = await res.json();
    throw redirect(303, `/rooms/${body.code}`);
  },

  joinRoom: async ({ request }) => {
    const data = await request.formData();
    const code = ((data.get('code') as string) ?? '').trim().toUpperCase();
    if (code.length !== 4)
      return fail(400, { joinError: 'Enter a 4-character room code.' });
    throw redirect(303, `/rooms/${code}`);
  }
};
