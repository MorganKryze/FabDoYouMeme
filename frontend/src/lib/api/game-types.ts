import { api } from './client';
import type { GameType } from './types';

export const gameTypesApi = {
  list: () => api.get<GameType[]>('/api/game-types'),
  get: (slug: string) => api.get<GameType>(`/api/game-types/${slug}`)
};
