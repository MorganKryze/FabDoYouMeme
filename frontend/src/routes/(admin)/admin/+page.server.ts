import type { PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

interface AdminStats {
  active_rooms: number;
  total_users: number;
  total_packs: number;
  pending_invites: number;
}

export const load: PageServerLoad = async ({ fetch }) => {
  const notifRes = await fetch(`${API_BASE}/api/admin/notifications?unread=true&limit=1`);
  const notifData = notifRes.ok ? await notifRes.json() : { total: 0 };
  return {
    unreadCount: notifData.total ?? 0,
    stats: null as AdminStats | null,
  };
};
