import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => {
  const [packRes, itemsRes] = await Promise.all([
    fetch(`/api/packs/${params.id}`),
    fetch(`/api/packs/${params.id}/items`)
  ]);

  if (!packRes.ok) throw error(404, 'Pack not found');
  return {
    pack: await packRes.json(),
    items: itemsRes.ok ? await itemsRes.json() : []
  };
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
