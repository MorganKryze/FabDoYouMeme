import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Pack, GameItem } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ params, fetch }) => {
  const [pack, items] = await Promise.all([
    apiFetch<Pack>(fetch, `/api/packs/${params.id}`),
    apiFetch<GameItem[]>(fetch, `/api/packs/${params.id}/items`)
  ]);
  return { pack, items };
};

export const actions: Actions = {
  deleteItem: async ({ request, fetch, params }) => {
    const data = await request.formData();
    const itemId = data.get('item_id') as string;
    const res = await fetch(`/api/packs/${params.id}/items/${itemId}`, {
      method: 'DELETE'
    });
    if (!res.ok)
      return fail(res.status, { deleteError: 'Failed to delete item.' });
    return { deleted: itemId };
  }
};
