import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { GameType } from '$lib/api/types';
import { API_BASE, apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  const gameTypes = await apiFetch<GameType[]>(fetch, '/api/game-types');
  return { gameTypes };
};

export const actions: Actions = {
  joinRoom: async ({ request, fetch }) => {
    const data = await request.formData();
    const code = ((data.get('code') as string) ?? '').trim().toUpperCase();
    if (code.length !== 4)
      return fail(400, { joinError: 'Enter a 4-character room code.' });

    const res = await fetch(`${API_BASE}/api/rooms/${code}`);
    if (!res.ok) {
      return fail(404, { joinError: 'Room not found. Check the code and try again.' });
    }

    throw redirect(303, `/rooms/${code}`);
  }
};
