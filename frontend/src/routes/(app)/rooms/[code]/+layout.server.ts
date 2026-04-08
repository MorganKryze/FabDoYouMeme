import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: LayoutServerLoad = async ({ params, fetch }) => {
  const res = await fetch(`${API_BASE}/api/rooms/${params.code}`);
  if (!res.ok) throw error(404, `Room ${params.code} not found`);
  const roomData = await res.json();

  return { room: roomData };
};
