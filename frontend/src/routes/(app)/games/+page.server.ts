import type { PageServerLoad } from './$types';
import { apiFetch } from '$lib/server/backend';
import type { HistoryRoom } from '../home/+page.server';

type HistoryResponse = {
  rooms: HistoryRoom[];
  next_cursor?: string | null;
};

export const load: PageServerLoad = async ({ fetch, url }) => {
  const after = url.searchParams.get('after') ?? '';
  const limit = 20;
  const qs = new URLSearchParams({ limit: String(limit) });
  if (after) qs.set('after', after);
  const res = await apiFetch<HistoryResponse>(fetch, `/api/users/me/history?${qs}`);
  return { rooms: res.rooms, nextCursor: res.next_cursor ?? null };
};
