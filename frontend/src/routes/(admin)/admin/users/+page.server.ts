import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { User } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

type AdminUser = User & {
  is_active: boolean;
  created_at: string;
  is_protected: boolean;
  games_played: number;
  // null when the user has no live session (i.e. fully logged out). Backend
  // derives it from MAX(sessions.created_at); see ListUsers in users.sql.
  last_login_at: string | null;
};

export const load: PageServerLoad = async ({ fetch, url }) => {
  const q = url.searchParams.get('q') ?? '';
  const cursor = url.searchParams.get('cursor') ?? '';
  const qs = new URLSearchParams({ limit: '50' });
  if (q) qs.set('q', q);
  // Backend's parsePagination() reads `after`, not `cursor`. The URL-bar
  // param stays `cursor` (the Svelte page hardcodes it), but we translate
  // on the way out so pagination actually advances.
  if (cursor) qs.set('after', cursor);

  const data = await apiFetch<{
    data: AdminUser[];
    next_cursor: string | null;
    total: number;
  }>(fetch, `/api/admin/users?${qs}`);

  return { users: data.data ?? [], nextCursor: data.next_cursor ?? null, q };
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
  },

  sendMagicLink: async ({ request, fetch }) => {
    const data = await request.formData();
    const userId = data.get('user_id') as string;

    const res = await fetch(`/api/admin/users/${userId}/magic-link`, {
      method: 'POST'
    });

    if (res.status === 204) {
      return { link_sent: userId };
    }

    let code = 'error';
    let retryAfter = 0;
    try {
      const b = await res.json();
      code = b.code ?? code;
      if (typeof b.retry_after === 'number') retryAfter = b.retry_after;
    } catch {
      /**/
    }

    let error = 'Failed to send magic link.';
    if (code === 'cooldown_active') error = `Please wait ${retryAfter}s before resending.`;
    else if (code === 'user_inactive') error = 'Cannot send to a deactivated account.';
    else if (code === 'user_not_found') error = 'User not found.';

    return fail(res.status, { error });
  }
};
