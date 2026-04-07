import { api } from './client';
import type { Room, RoomConfig } from './types';

export const roomsApi = {
  create: (body: {
    game_type_id: string;
    pack_id: string;
    mode?: 'multiplayer' | 'solo';
    config?: Partial<RoomConfig>;
  }) => api.post<Room>('/api/rooms', body),

  get: (code: string) => api.get<Room>(`/api/rooms/${code}`),

  updateConfig: (code: string, config: Partial<RoomConfig>) =>
    api.patch<Room>(`/api/rooms/${code}/config`, config),

  leave: (code: string) => api.post<void>(`/api/rooms/${code}/leave`),

  kick: (code: string, userId: string) =>
    api.post<void>(`/api/rooms/${code}/kick`, { user_id: userId }),

  leaderboard: (code: string) =>
    api.get<{ leaderboard: unknown[] }>(`/api/rooms/${code}/leaderboard`)
};
