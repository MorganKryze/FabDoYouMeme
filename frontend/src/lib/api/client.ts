// Browser-side API calls travel through the SvelteKit server as a relative
// URL (see the /api/* proxy in `hooks.server.ts`). The session cookie rides
// along on the same origin and the handle hook forwards it to the backend.
// There is no SSR consumer of this module — server loads use `apiFetch`
// from `$lib/server/backend.ts` directly.
const BASE = '';

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
