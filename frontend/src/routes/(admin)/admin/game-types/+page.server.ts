import type { PageServerLoad } from './$types';
import type { GameType } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  const gameTypes = await apiFetch<GameType[]>(fetch, '/api/game-types');
  return { gameTypes };
};
