import type { PageServerLoad } from './$types';
import { apiFetch } from '$lib/server/backend';

interface AdminStats {
  active_rooms: number;
  total_users: number;
  total_packs: number;
  pending_invites: number;
}

export const load: PageServerLoad = async ({ fetch }) => {
  const notifData = await apiFetch<{ total: number }>(
    fetch,
    '/api/admin/notifications?unread=true&limit=1'
  );
  return {
    unreadCount: notifData.total ?? 0,
    stats: null as AdminStats | null,
  };
};
