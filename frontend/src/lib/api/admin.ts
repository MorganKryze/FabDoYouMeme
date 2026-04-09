import { api } from './client';
import type { User, Invite, PaginatedResponse } from './types';

export const adminApi = {
  listUsers: (params?: { q?: string; after?: string }) => {
    const q = new URLSearchParams();
    if (params?.q) q.set('q', params.q);
    if (params?.after) q.set('after', params.after);
    return api.get<PaginatedResponse<User> & { total: number }>(
      `/api/admin/users?${q}`
    );
  },
  updateUser: (
    id: string,
    body: {
      role?: 'player' | 'admin';
      is_active?: boolean;
      email?: string;
      username?: string;
    }
  ) => api.patch<User>(`/api/admin/users/${id}`, body),
  deleteUser: (id: string) => api.delete<void>(`/api/admin/users/${id}`),

  listInvites: (params?: { after?: string }) => {
    const q = new URLSearchParams();
    if (params?.after) q.set('after', params.after);
    return api.get<PaginatedResponse<Invite> & { total: number }>(
      `/api/admin/invites?${q}`
    );
  },
  createInvite: (body: {
    token: string;
    label?: string;
    restricted_email?: string;
    max_uses?: number;
  }) => api.post<Invite>('/api/admin/invites', body),
  deleteInvite: (id: string) => api.delete<void>(`/api/admin/invites/${id}`)
};
