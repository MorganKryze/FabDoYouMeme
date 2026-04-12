import type { PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch, url }) => {
  const cursor = url.searchParams.get('cursor') ?? '';
  const q = cursor ? `?after=${encodeURIComponent(cursor)}` : '';
  const body = await apiFetch<{ data: Pack[]; next_cursor: string | null }>(
    fetch,
    `/api/packs${q}`
  );
  return {
    packs: body.data ?? [],
    nextCursor: body.next_cursor ?? null,
  };
};
