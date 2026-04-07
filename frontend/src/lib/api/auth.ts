import { api } from './client';
import type { User } from './types';

export const authApi = {
  register: (body: {
    invite_token: string;
    username: string;
    email: string;
    consent: true;
    age_affirmation: true;
  }) =>
    api.post<{ user_id: string; warning?: string }>('/api/auth/register', body),

  magicLink: (email: string) =>
    api.post<void>('/api/auth/magic-link', { email }),

  verify: (token: string) =>
    api.post<{ user_id: string }>('/api/auth/verify', { token }),

  logout: () => api.post<void>('/api/auth/logout'),

  me: () => api.get<User>('/api/auth/me'),

  patchMe: (body: { username?: string; email?: string }) =>
    api.patch<User | { message: string }>('/api/users/me', body),

  getHistory: (cursor?: string) =>
    api.get<{ rooms: unknown[]; next_cursor: string | null }>(
      `/api/users/me/history${cursor ? `?after=${cursor}` : ''}`
    ),

  getExport: () => api.get<Blob>('/api/users/me/export')
};
