import type { PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch }) => {
  const res = await fetch('/api/packs');
  const packs: Pack[] = res.ok ? await res.json() : [];
  return { packs };
};
