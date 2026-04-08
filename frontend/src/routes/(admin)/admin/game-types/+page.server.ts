import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/game-types');
  const gameTypes = res.ok ? await res.json() : [];
  return { gameTypes };
};
