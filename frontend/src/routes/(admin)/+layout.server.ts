import { redirect, error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { apiFetch } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ locals, fetch }) => {
  if (!locals.user) {
    throw redirect(303, '/');
  }
  if (locals.user.role !== 'admin') {
    throw error(403, 'Admin access required');
  }
  const notifData = await apiFetch<{ total: number }>(
    fetch,
    '/api/admin/notifications?unread=true&limit=1'
  );
  return {
    user: locals.user,
    unreadNotifications: notifData.total ?? 0,
  };
};
