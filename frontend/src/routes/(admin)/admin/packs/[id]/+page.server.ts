import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Pack, GameItem } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ params, fetch }) => {
  const [pack, itemsBody] = await Promise.all([
    apiFetch<Pack>(fetch, `/api/packs/${params.id}`),
    apiFetch<{ data: GameItem[]; next_cursor: string | null }>(
      fetch,
      `/api/packs/${params.id}/items`
    )
  ]);
  return { pack, items: itemsBody.data ?? [] };
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
  },
  setStatus: async ({ request, fetch, params }) => {
    const data = await request.formData();
    const status = data.get('status') as string;
    if (status !== 'active' && status !== 'flagged' && status !== 'banned') {
      return fail(400, { statusError: 'Invalid status.' });
    }
    const res = await fetch(`/api/packs/${params.id}/status`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status })
    });
    if (!res.ok) return fail(res.status, { statusError: 'Failed to update status.' });
    return { statusUpdated: status };
  }
};
