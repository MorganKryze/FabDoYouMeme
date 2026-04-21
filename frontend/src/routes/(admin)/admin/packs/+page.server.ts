import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch, url }) => {
  const raw = url.searchParams.get('language');
  const language = raw === 'en' || raw === 'fr' || raw === 'multi' ? raw : null;
  const query = language ? `?language=${language}` : '';
  const body = await apiFetch<{ data: Pack[]; next_cursor: string | null }>(
    fetch,
    `/api/packs${query}`
  );
  return { packs: body.data ?? [], language };
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
