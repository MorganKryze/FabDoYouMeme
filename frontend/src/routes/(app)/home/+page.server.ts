import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { GameType } from '$lib/api/types';
import { API_BASE, apiFetch } from '$lib/server/backend';

// Matches the shape returned by GET /api/users/me/history
// (see backend/internal/auth/profile.go:historyRoom).
export type HistoryRoom = {
  code: string;
  game_type_slug: string;
  pack_name: string;
  started_at: string;
  finished_at?: string;
  score: number;
  rank: number;
  player_count: number;
};

// Matches GET /api/users/me/active-room (backend/internal/auth/profile.go:activeRoom).
// `null` means the user is free to create or join a new room.
export type ActiveRoom = {
  code: string;
  state: 'lobby' | 'playing';
  game_type_slug: string;
  is_host: boolean;
};

type HistoryResponse = {
  rooms: HistoryRoom[];
  next_cursor?: string | null;
};

type ActiveRoomResponse = {
  room: ActiveRoom | null;
};

export const load: PageServerLoad = async ({ fetch }) => {
  // Parallel loads: history for "recent activity + derived stats", game types
  // for the host grid, and the single-room-enforcement check. Both require
  // auth and are served by the (app) layout's session gate, so anon visitors
  // never reach this load.
  const [historyRes, gameTypes, activeRoomRes] = await Promise.all([
    apiFetch<HistoryResponse>(fetch, '/api/users/me/history?limit=10'),
    apiFetch<GameType[]>(fetch, '/api/game-types'),
    apiFetch<ActiveRoomResponse>(fetch, '/api/users/me/active-room'),
  ]);
  return {
    history: historyRes.rooms,
    gameTypes,
    activeRoom: activeRoomRes.room,
  };
};

export const actions: Actions = {
  joinRoom: async ({ request, fetch, locals }) => {
    if (!locals.user) {
      return fail(401, { joinError: 'Sign in to join with your account.' });
    }
    const data = await request.formData();
    const code = ((data.get('code') as string) ?? '').trim().toUpperCase();
    if (code.length !== 4)
      return fail(400, { joinError: 'Enter a 4-character room code.' });

    // Pre-flight the active-room gate so we can surface the 409 cleanly
    // instead of letting the WebSocket upgrade fail mid-navigation.
    const activeRes = await fetch(`${API_BASE}/api/users/me/active-room`);
    if (activeRes.ok) {
      const body = (await activeRes.json()) as ActiveRoomResponse;
      if (body.room && body.room.code !== code) {
        return fail(409, {
          joinError: `You're already in room ${body.room.code}. Return to it first.`,
        });
      }
    }

    const res = await fetch(`${API_BASE}/api/rooms/${code}`);
    if (!res.ok) {
      return fail(404, { joinError: 'Room not found. Check the code and try again.' });
    }

    throw redirect(303, `/rooms/${code}`);
  },
};
