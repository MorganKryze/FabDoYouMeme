import type { PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch, url }) => {
  const cursor = url.searchParams.get('cursor') ?? '';
  const q = cursor ? `?after=${encodeURIComponent(cursor)}` : '';
  const res = await fetch(`${API_BASE}/api/packs${q}`);
  const body = res.ok ? await res.json() : { data: [], next_cursor: null };
  return {
    packs: body.data ?? [],
    nextCursor: body.next_cursor ?? null,
  };
};
