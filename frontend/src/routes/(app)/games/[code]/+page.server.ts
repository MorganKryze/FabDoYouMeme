import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { apiFetch } from '$lib/server/backend';
import type { ReplayPayload } from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch, params }) => {
  const code = params.code.toUpperCase();
  try {
    const replay = await apiFetch<ReplayPayload>(fetch, `/api/rooms/${code}/replay`);
    return { replay };
  } catch (e) {
    const status = (e as { status?: number }).status ?? 500;
    if (status === 404) throw error(404, "That game's gone");
    if (status === 403) throw error(403, "You weren't in this room");
    throw error(status as never, 'Failed to load replay');
  }
};
