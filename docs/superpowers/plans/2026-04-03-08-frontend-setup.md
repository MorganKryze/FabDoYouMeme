# Frontend — Project Setup + State Layer + API Client — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the SvelteKit project (from Phase 1 scaffold), implement all four global state classes using Svelte 5 runes, build the typed API client layer, and configure `src/hooks.server.ts` with CSP nonces and session loading.

**Architecture:** Global state lives in `src/lib/state/` as Svelte 5 reactive classes (no stores). API calls live in `src/lib/api/` as typed fetch wrappers. The WS connection is owned by `WsState` with exponential backoff + jitter.

**Tech Stack:** SvelteKit (`adapter-node`), Svelte 5, Tailwind CSS v4, TypeScript, `PUBLIC_API_URL` env var.

**Prerequisite:** Phase 1 complete (SvelteKit scaffold exists with Tailwind v4, adapter-node, shadcn-svelte).

---

### Task 1: SvelteKit hooks + CSP nonce

**Files:**

- Create: `frontend/src/hooks.server.ts`
- Create: `frontend/src/app.d.ts`

- [ ] **Step 1: Write `app.d.ts` — type augmentation**

```ts
// frontend/src/app.d.ts
declare global {
  namespace App {
    interface Locals {
      user: {
        id: string;
        username: string;
        email: string;
        role: 'player' | 'admin';
      } | null;
      nonce: string;
    }
  }
}
export {};
```

- [ ] **Step 2: Write `hooks.server.ts`**

```ts
// frontend/src/hooks.server.ts
import type { Handle } from '@sveltejs/kit';
import { randomBytes } from 'node:crypto';
import { env } from '$env/dynamic/private';

const API_URL = env.PUBLIC_API_URL || 'http://localhost:8080';

export const handle: Handle = async ({ event, resolve }) => {
  // Generate per-request CSP nonce
  const nonce = randomBytes(16).toString('base64');
  event.locals.nonce = nonce;

  // Load session from backend (session cookie is HttpOnly — forwarded automatically)
  try {
    const res = await fetch(`${API_URL}/api/auth/me`, {
      headers: { cookie: event.request.headers.get('cookie') ?? '' }
    });
    if (res.ok) {
      event.locals.user = await res.json();
    } else {
      event.locals.user = null;
    }
  } catch {
    event.locals.user = null;
  }

  const response = await resolve(event, {
    transformPageChunk: ({ html }) => html.replace('%sveltekit.nonce%', nonce)
  });

  return response;
};
```

- [ ] **Step 3: Update `svelte.config.js` CSP to use nonce from hook**

Ensure `frontend/svelte.config.js` has (already done in Phase 1):

```js
csp: {
  mode: 'nonce',
  directives: {
    'default-src': ['self'],
    'script-src': ['self'],
    'style-src': ['self'],
    'font-src': ['self'],
    'img-src': ['self', 'data:', 'blob:'],
    'connect-src': ['self', 'wss:', 'ws:'],
    'frame-ancestors': ['none']
  }
}
```

- [ ] **Step 4: Verify type-check**

```bash
cd frontend && npm run check
```

Expected: 0 errors.

---

### Task 2: API type definitions

**Files:**

- Create: `frontend/src/lib/api/types.ts`

- [ ] **Step 1: Write `types.ts`**

```ts
// frontend/src/lib/api/types.ts

export interface User {
  id: string;
  username: string;
  email: string;
  role: 'player' | 'admin';
}

export interface GameType {
  id: string;
  slug: string;
  name: string;
  description: string | null;
  version: string;
  supports_solo: boolean;
  config: GameTypeConfig;
  supported_payload_versions: number[];
}

export interface GameTypeConfig {
  min_round_duration_seconds: number;
  max_round_duration_seconds: number;
  default_round_duration_seconds: number;
  min_voting_duration_seconds: number;
  max_voting_duration_seconds: number;
  default_voting_duration_seconds: number;
  min_players: number;
  max_players: number | null;
  min_round_count: number;
  max_round_count: number;
  default_round_count: number;
}

export interface Pack {
  id: string;
  name: string;
  description: string | null;
  owner_id: string | null;
  is_official: boolean;
  visibility: 'private' | 'public';
  status: 'active' | 'flagged' | 'banned';
  created_at: string;
}

export interface Room {
  id: string;
  code: string;
  game_type_id: string;
  game_type_slug: string;
  pack_id: string;
  host_id: string;
  mode: 'multiplayer' | 'solo';
  state: 'lobby' | 'playing' | 'finished';
  config: RoomConfig;
  created_at: string;
  finished_at: string | null;
}

export interface RoomConfig {
  round_duration_seconds: number;
  voting_duration_seconds: number;
  round_count: number;
}

export interface Invite {
  id: string;
  token: string;
  label: string | null;
  restricted_email: string | null;
  max_uses: number;
  uses_count: number;
  expires_at: string | null;
  created_at: string;
}

export interface ApiError {
  error: string;
  code: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  next_cursor: string | null;
  total?: number;
}

// WebSocket message types
export type WsMessageType =
  | 'pong'
  | 'player_joined'
  | 'player_left'
  | 'player_kicked'
  | 'reconnecting'
  | 'game_started'
  | 'round_started'
  | 'submissions_closed'
  | 'vote_results'
  | 'game_ended'
  | 'room_state'
  | 'error'
  | `meme-caption:submissions_shown`
  | `meme-caption:vote_results`;

export interface WsMessage {
  type: WsMessageType | string;
  data?: unknown;
}

export interface Player {
  user_id: string;
  username: string;
}

export interface LeaderboardEntry {
  user_id: string;
  username: string;
  total_score: number;
  rank: number;
}
```

---

### Task 3: API client layer

**Files:**

- Create: `frontend/src/lib/api/client.ts`
- Create: `frontend/src/lib/api/auth.ts`
- Create: `frontend/src/lib/api/rooms.ts`
- Create: `frontend/src/lib/api/packs.ts`
- Create: `frontend/src/lib/api/admin.ts`
- Create: `frontend/src/lib/api/index.ts`

- [ ] **Step 1: Write `client.ts` — base fetch wrapper**

```ts
// frontend/src/lib/api/client.ts
import { browser } from '$app/environment';
import { env } from '$env/dynamic/public';

const BASE =
  (browser ? env.PUBLIC_API_URL : process.env.PUBLIC_API_URL) ||
  'http://localhost:8080';

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'include', // send session cookie
    headers: { 'Content-Type': 'application/json', ...init.headers },
    ...init
  });

  if (!res.ok) {
    let code = 'internal_error';
    let message = res.statusText;
    try {
      const body = await res.json();
      code = body.code ?? code;
      message = body.error ?? message;
    } catch {}
    throw new ApiError(res.status, code, message);
  }

  // 204 No Content
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string, init?: RequestInit) =>
    request<T>(path, { method: 'GET', ...init }),
  post: <T>(path: string, body?: unknown, init?: RequestInit) =>
    request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
      ...init
    }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined
    }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' })
};
```

- [ ] **Step 2: Write `auth.ts`**

```ts
// frontend/src/lib/api/auth.ts
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
```

- [ ] **Step 3: Write `rooms.ts`**

```ts
// frontend/src/lib/api/rooms.ts
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
```

- [ ] **Step 4: Write `packs.ts`**

```ts
// frontend/src/lib/api/packs.ts
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
```

- [ ] **Step 5: Write `admin.ts`**

```ts
// frontend/src/lib/api/admin.ts
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
  updateUser: (id: string, body: { role?: string; is_active?: boolean }) =>
    api.patch<User>(`/api/admin/users/${id}`, body),
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
```

- [ ] **Step 6: Write `index.ts` barrel export**

```ts
// frontend/src/lib/api/index.ts
export { api, ApiError } from './client';
export { authApi } from './auth';
export { roomsApi } from './rooms';
export { packsApi } from './packs';
export { adminApi } from './admin';
export type * from './types';
```

- [ ] **Step 7: Type-check**

```bash
cd frontend && npm run check
```

Expected: 0 errors.

---

### Task 4: Global state — user + WsState + RoomState + StudioState

**Files:**

- Create: `frontend/src/lib/state/user.svelte.ts`
- Create: `frontend/src/lib/state/ws.svelte.ts`
- Create: `frontend/src/lib/state/room.svelte.ts`
- Create: `frontend/src/lib/state/studio.svelte.ts`
- Create: `frontend/src/lib/state/index.ts`

- [ ] **Step 1: Write `user.svelte.ts`**

```ts
// frontend/src/lib/state/user.svelte.ts

class UserState {
  id = $state<string | null>(null);
  username = $state<string | null>(null);
  email = $state<string | null>(null);
  role = $state<'player' | 'admin' | null>(null);

  isAuthenticated = $derived(this.id !== null);
  isAdmin = $derived(this.role === 'admin');

  setFrom(u: {
    id: string;
    username: string;
    email: string;
    role: 'player' | 'admin';
  }) {
    this.id = u.id;
    this.username = u.username;
    this.email = u.email;
    this.role = u.role;
  }

  clear() {
    this.id = null;
    this.username = null;
    this.email = null;
    this.role = null;
  }
}

export const user = new UserState();
```

- [ ] **Step 2: Write `ws.svelte.ts`**

```ts
// frontend/src/lib/state/ws.svelte.ts
import type { WsMessage } from '$lib/api/types';
import { env } from '$env/dynamic/public';

type WsStatus = 'connected' | 'reconnecting' | 'error' | 'closed';

class WsState {
  status = $state<WsStatus>('closed');
  retryCount = $state(0);

  #ws: WebSocket | null = null;
  #handlers = new Map<string, ((data: unknown) => void)[]>();
  #roomCode: string | null = null;
  #retryTimer: ReturnType<typeof setTimeout> | null = null;
  #pingTimer: ReturnType<typeof setInterval> | null = null;
  #pongTimeout: ReturnType<typeof setTimeout> | null = null;

  /** Connect to a room's WebSocket. */
  connect(roomCode: string) {
    this.#roomCode = roomCode;
    this.retryCount = 0;
    this.#connect();
  }

  #connect() {
    if (this.#ws) {
      this.#ws.close();
    }
    const base = (env.PUBLIC_API_URL || 'http://localhost:8080').replace(
      /^http/,
      'ws'
    );
    this.#ws = new WebSocket(`${base}/api/ws/rooms/${this.#roomCode}`);

    this.#ws.addEventListener('open', () => {
      this.status = 'connected';
      this.retryCount = 0;
      this.#startPing();
    });

    this.#ws.addEventListener('message', ev => {
      try {
        const msg: WsMessage = JSON.parse(ev.data as string);
        this.#dispatch(msg);
      } catch {}
    });

    this.#ws.addEventListener('close', () => {
      this.#stopPing();
      if (this.retryCount < 10) {
        this.status = 'reconnecting';
        const delay =
          Math.min(30, Math.pow(2, this.retryCount - 1)) + Math.random();
        this.#retryTimer = setTimeout(() => {
          this.retryCount++;
          this.#connect();
        }, delay * 1000);
      } else {
        this.status = 'error';
      }
    });

    this.#ws.addEventListener('error', () => {
      // close event follows; handled there
    });
  }

  /** Send a typed message to the server. */
  send(type: string, data?: unknown) {
    if (this.#ws?.readyState === WebSocket.OPEN) {
      this.#ws.send(JSON.stringify({ type, data }));
    }
  }

  /** Register a handler for a specific message type. Returns unsubscribe fn. */
  onMessage(type: string, handler: (data: unknown) => void): () => void {
    if (!this.#handlers.has(type)) this.#handlers.set(type, []);
    this.#handlers.get(type)!.push(handler);
    return () => {
      const list = this.#handlers.get(type);
      if (list) {
        const i = list.indexOf(handler);
        if (i >= 0) list.splice(i, 1);
      }
    };
  }

  disconnect() {
    this.#roomCode = null;
    if (this.#retryTimer) clearTimeout(this.#retryTimer);
    this.#stopPing();
    this.#ws?.close();
    this.#ws = null;
    this.status = 'closed';
    this.retryCount = 0;
  }

  #dispatch(msg: WsMessage) {
    const handlers = this.#handlers.get(msg.type) ?? [];
    for (const h of handlers) h(msg.data);
    // Also dispatch to '*' catch-all handlers
    const all = this.#handlers.get('*') ?? [];
    for (const h of all) h(msg);
  }

  #startPing() {
    this.#pingTimer = setInterval(() => {
      this.send('ping');
      this.#pongTimeout = setTimeout(() => {
        // No pong within 10s — server dead; force reconnect
        this.#ws?.close();
      }, 10_000);
    }, 25_000);

    this.onMessage('pong', () => {
      if (this.#pongTimeout) clearTimeout(this.#pongTimeout);
    });
  }

  #stopPing() {
    if (this.#pingTimer) clearInterval(this.#pingTimer);
    if (this.#pongTimeout) clearTimeout(this.#pongTimeout);
  }
}

export const ws = new WsState();
```

- [ ] **Step 3: Write `room.svelte.ts`**

```ts
// frontend/src/lib/state/room.svelte.ts
import type { Player, LeaderboardEntry, WsMessage } from '$lib/api/types';

type RoomPhase = 'idle' | 'countdown' | 'submitting' | 'voting' | 'results';
type RoomStatus = 'lobby' | 'playing' | 'finished';

interface Round {
  roundNumber: number;
  item: { payload: unknown; media_url?: string };
  durationSeconds: number;
  endsAt: string; // ISO8601
}

class RoomState {
  code = $state<string | null>(null);
  gameTypeSlug = $state<string | null>(null);
  status = $state<RoomStatus>('lobby');
  players = $state<Player[]>([]);
  currentRound = $state<Round | null>(null);
  phase = $state<RoomPhase>('idle');
  submissions = $state<unknown[]>([]);
  leaderboard = $state<LeaderboardEntry[]>([]);
  endReason = $state<string | null>(null);

  hasSubmitted = $state(false);
  hasVoted = $state(false);

  handleMessage(msg: WsMessage) {
    switch (msg.type) {
      case 'player_joined': {
        const d = msg.data as Player;
        if (!this.players.find(p => p.user_id === d.user_id)) {
          this.players = [...this.players, d];
        }
        break;
      }
      case 'player_left':
      case 'player_kicked': {
        const d = msg.data as Player;
        this.players = this.players.filter(p => p.user_id !== d.user_id);
        break;
      }
      case 'game_started':
        this.status = 'playing';
        this.phase = 'countdown';
        break;
      case 'round_started':
        this.currentRound = msg.data as Round;
        this.phase = 'submitting';
        this.submissions = [];
        this.hasSubmitted = false;
        this.hasVoted = false;
        break;
      case 'submissions_closed':
        this.phase = 'voting';
        break;
      case 'vote_results':
        this.phase = 'results';
        break;
      case 'game_ended': {
        const d = msg.data as {
          reason: string;
          leaderboard: LeaderboardEntry[];
        };
        this.status = 'finished';
        this.phase = 'idle';
        this.endReason = d.reason;
        this.leaderboard = d.leaderboard ?? [];
        break;
      }
      case 'room_state': {
        const d = msg.data as { state: RoomStatus; players: Player[] };
        this.status = d.state;
        this.players = d.players;
        break;
      }
    }
  }

  reset() {
    this.code = null;
    this.gameTypeSlug = null;
    this.status = 'lobby';
    this.players = [];
    this.currentRound = null;
    this.phase = 'idle';
    this.submissions = [];
    this.leaderboard = [];
    this.endReason = null;
    this.hasSubmitted = false;
    this.hasVoted = false;
  }
}

export const room = new RoomState();
```

- [ ] **Step 4: Write `studio.svelte.ts`**

```ts
// frontend/src/lib/state/studio.svelte.ts
import type { Pack } from '$lib/api/types';

interface Item {
  id: string;
  position: number;
  payload_version: number;
  current_version_id: string | null;
  media_key?: string | null;
  payload?: unknown;
}

interface ItemVersion {
  id: string;
  item_id: string;
  version_number: number;
  media_key: string | null;
  payload: unknown;
  created_at: string;
  deleted_at: string | null;
}

class StudioState {
  selectedPackId = $state<string | null>(null);
  selectedItemId = $state<string | null>(null);
  /** Up to 2 version IDs for side-by-side comparison */
  selectedVersionIds = $state<string[]>([]);

  packs = $state<Pack[]>([]);
  items = $state<Item[]>([]);
  versions = $state<ItemVersion[]>([]);

  selectPack(packId: string) {
    this.selectedPackId = packId;
    this.selectedItemId = null;
    this.selectedVersionIds = [];
    this.items = [];
    this.versions = [];
  }

  selectItem(itemId: string) {
    this.selectedItemId = itemId;
    this.selectedVersionIds = [];
    this.versions = [];
  }

  toggleVersionSelection(versionId: string) {
    if (this.selectedVersionIds.includes(versionId)) {
      this.selectedVersionIds = this.selectedVersionIds.filter(
        id => id !== versionId
      );
    } else if (this.selectedVersionIds.length < 2) {
      this.selectedVersionIds = [...this.selectedVersionIds, versionId];
    }
  }

  reset() {
    this.selectedPackId = null;
    this.selectedItemId = null;
    this.selectedVersionIds = [];
    this.packs = [];
    this.items = [];
    this.versions = [];
  }
}

export const studio = new StudioState();
```

- [ ] **Step 5: Write `index.ts`**

```ts
// frontend/src/lib/state/index.ts
export { user } from './user.svelte';
export { ws } from './ws.svelte';
export { room } from './room.svelte';
export { studio } from './studio.svelte';
```

- [ ] **Step 6: Final type-check**

```bash
cd frontend && npm run check
```

Expected: 0 errors.

- [ ] **Step 7: Build check**

```bash
cd frontend && PUBLIC_API_URL=http://localhost:8080 npm run build
```

Expected: `build/` directory created, 0 errors.

---

### Verification

```bash
cd frontend && npm run check
cd frontend && PUBLIC_API_URL=http://localhost:8080 npm run build
```

Mark phase 8 complete in `docs/implementation-status.md`.
