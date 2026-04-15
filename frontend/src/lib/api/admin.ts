import { api } from './client';
import type {
  User,
  Invite,
  PaginatedResponse,
  DeepHealthResponse,
  AdminStats,
  AdminStorageStats,
  AuditEntry,
  DangerReport
} from './types';

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
  deleteInvite: (id: string) => api.delete<void>(`/api/admin/invites/${id}`),

  getHealth: () => api.get<DeepHealthResponse>('/api/health/deep'),
  getStats: () => api.get<AdminStats>('/api/admin/stats'),
  getStorageStats: () =>
    api.get<AdminStorageStats>('/api/admin/storage'),
  listAudit: (limit = 10) =>
    api.get<{ data: AuditEntry[] }>(`/api/admin/audit?limit=${limit}`),

  // Destructive admin actions ("danger zone"). The confirmation phrase is
  // hardcoded here to match the server-side expected value — both sides
  // must agree or the backend responds 400 invalid_confirmation. Keep
  // these strings in lockstep with backend/internal/api/danger.go.
  danger: {
    wipeGameHistory: () =>
      api.post<DangerReport>('/api/admin/danger/wipe-game-history', {
        confirmation: 'wipe game history'
      }),
    wipePacksAndMedia: () =>
      api.post<DangerReport>('/api/admin/danger/wipe-packs-and-media', {
        confirmation: 'wipe packs and media'
      }),
    wipeInvites: () =>
      api.post<DangerReport>('/api/admin/danger/wipe-invites', {
        confirmation: 'wipe invites'
      }),
    wipeSessions: () =>
      api.post<DangerReport>('/api/admin/danger/wipe-sessions', {
        confirmation: 'force logout everyone'
      }),
    fullReset: () =>
      api.post<DangerReport>('/api/admin/danger/full-reset', {
        confirmation: 'RESET TO FIRST BOOT'
      })
  }
};
