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

// adoptErrorBody normalises every error envelope we've seen in the wild
// into { code, message } strings. Three shapes are accepted:
//
//   1. Native backend shape — { error: "human msg", code: "machine_code" }.
//      This is what backend/internal/api/packs.go writeError produces and
//      what every other handler is expected to return.
//
//   2. Pangolin / Traefik proxy envelope — when the public ingress
//      intercepts a 4xx/5xx from the upstream and replaces the body with
//      its own error page. Observed shape:
//        { error: true, code: 409, message: "Conflict",
//          description: "...", details: { ...possibly the upstream body... } }
//      We prefer `details.error` when it's a string (the original message
//      survived), then fall back to `description` → `message`. Without
//      this branch every backend error read as the literal "true" in the
//      UI because `body.error` was a boolean.
//
//   3. Plain `{ message: "..." }` from frameworks that don't emit code.
//
// Returns a partial object — the caller decides what to do when fields
// are missing.
function adoptErrorBody(body: unknown): { code?: string; message?: string } {
  if (!body || typeof body !== 'object') return {};
  const b = body as Record<string, unknown>;

  // 1. Native shape — { error: string, code: string }
  if (typeof b.error === 'string' && b.error.length > 0) {
    return {
      code: typeof b.code === 'string' ? b.code : undefined,
      message: b.error
    };
  }

  // 2. Pangolin/Traefik wrapper — the upstream payload may live in `details`.
  if (b.details && typeof b.details === 'object') {
    const inner = adoptErrorBody(b.details);
    if (inner.message) return inner;
  }

  // 2b. Pangolin's outer envelope itself — fall back to description, then
  //     message, then a stringified numeric code.
  const description = typeof b.description === 'string' ? b.description : '';
  const messageField = typeof b.message === 'string' ? b.message : '';
  const codeField =
    typeof b.code === 'string' ? b.code
    : typeof b.code === 'number' ? String(b.code)
    : undefined;

  if (description || messageField) {
    return {
      code: codeField,
      message: [messageField, description].filter(Boolean).join(' — ')
    };
  }
  return { code: codeField };
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
    // Read once as text, then try to parse JSON — lets us recover the body
    // even when the upstream returned plain text (ingress 5xx pages, plain
    // 413s, etc.) and dump the raw payload to console for triage when the
    // shape is unexpected.
    const raw = await res.text().catch(() => '');
    try {
      const body = JSON.parse(raw);
      const adopted = adoptErrorBody(body);
      if (adopted.code) code = adopted.code;
      if (adopted.message) {
        message = adopted.message;
      } else {
        // eslint-disable-next-line no-console
        console.warn(
          `[api] ${path} → ${res.status} could not extract a string error from response:`,
          body
        );
        message = `${res.status} ${res.statusText}`;
      }
    } catch {
      // Not JSON — keep the text body around if it's short enough to be
      // a useful message; otherwise fall back to statusText.
      if (raw && raw.length <= 240) message = raw;
    }
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
