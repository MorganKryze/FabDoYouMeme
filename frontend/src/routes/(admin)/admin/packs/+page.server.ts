import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  const packs = await apiFetch<Pack[]>(fetch, '/api/packs?include_all=true');
  return { packs };
};

export const actions: Actions = {
  createPack: async ({ request, fetch }) => {
    const data = await request.formData();
    const name = (data.get('name') as string | null) ?? '';
    const description = (data.get('description') as string | null) ?? '';

    const res = await fetch('/api/packs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, description: description || undefined })
    });
    if (!res.ok)
      return fail(res.status, { createError: 'Failed to create pack.' });
    return { created: await res.json() };
  },

  deletePack: async ({ request, fetch }) => {
    const data = await request.formData();
    const packId = data.get('pack_id') as string;
    const res = await fetch(`/api/packs/${packId}`, { method: 'DELETE' });
    if (!res.ok)
      return fail(res.status, { deleteError: 'Failed to delete pack.' });
    return { deleted: packId };
  }
};
