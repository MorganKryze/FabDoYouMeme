import { api } from './client';
import type { Pack, PaginatedResponse } from './types';

export const packsApi = {
  list: (params?: {
    game_type_id?: string;
    after?: string;
    limit?: number;
  }) => {
    const q = new URLSearchParams();
    if (params?.game_type_id) q.set('game_type_id', params.game_type_id);
    if (params?.after) q.set('after', params.after);
    if (params?.limit) q.set('limit', String(params.limit));
    return api.get<PaginatedResponse<Pack>>(`/api/packs?${q}`);
  },
  create: (body: { name: string; description?: string; visibility?: string }) =>
    api.post<Pack>('/api/packs', body),
  get: (id: string) => api.get<Pack>(`/api/packs/${id}`),
  update: (id: string, body: Partial<Pack>) =>
    api.patch<Pack>(`/api/packs/${id}`, body),
  delete: (id: string) => api.delete<void>(`/api/packs/${id}`),
  listItems: (packId: string, params?: { after?: string }) => {
    const q = new URLSearchParams();
    if (params?.after) q.set('after', params.after);
    return api.get<{ data: unknown[]; next_cursor: string | null }>(
      `/api/packs/${packId}/items?${q}`
    );
  }
};
