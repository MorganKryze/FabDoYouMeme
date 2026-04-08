import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
  const q = url.searchParams.get('q') ?? '';
  const cursor = url.searchParams.get('cursor') ?? '';
  const qs = new URLSearchParams({ limit: '50' });
  if (q) qs.set('q', q);
  if (cursor) qs.set('cursor', cursor);

  const res = await fetch(`/api/admin/users?${qs}`);
  const data = res.ok ? await res.json() : { users: [], next_cursor: null };

  return { users: data.users ?? [], nextCursor: data.next_cursor ?? null, q };
};

export const actions: Actions = {
  updateUser: async ({ request, fetch }) => {
    const data = await request.formData();
    const userId = data.get('user_id') as string;
    const patch: Record<string, unknown> = {};
    if (data.has('role')) patch.role = data.get('role');
    if (data.has('is_active'))
      patch.is_active = data.get('is_active') === 'true';
    if (data.has('username')) patch.username = data.get('username');
    if (data.has('email')) patch.email = data.get('email');

    const res = await fetch(`/api/admin/users/${userId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch)
    });

    if (!res.ok) {
      let code = 'error';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      return fail(res.status, {
        error:
          code === 'username_taken'
            ? 'Username already taken.'
            : 'Update failed.'
      });
    }
    return { success: true };
  },

  deleteUser: async ({ request, fetch }) => {
    const data = await request.formData();
    const userId = data.get('user_id') as string;

    const res = await fetch(`/api/admin/users/${userId}`, { method: 'DELETE' });
    if (!res.ok) return fail(res.status, { error: 'Failed to delete user.' });
    return { deleted: userId };
  }
};
