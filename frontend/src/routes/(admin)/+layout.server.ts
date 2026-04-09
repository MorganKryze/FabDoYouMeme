import { redirect, error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ locals, fetch }) => {
  if (!locals.user) {
    throw redirect(303, '/');
  }
  if (locals.user.role !== 'admin') {
    throw error(403, 'Admin access required');
  }
  const res = await fetch(`${API_BASE}/api/admin/notifications?unread=true&limit=1`);
  const notifData = res.ok ? await res.json() : { total: 0 };
  return {
    user: locals.user,
    unreadNotifications: notifData.total ?? 0,
  };
};
