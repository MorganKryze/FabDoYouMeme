import type { Handle, HandleFetch, RequestEvent } from '@sveltejs/kit';
import { randomBytes } from 'node:crypto';
import { API_BASE } from '$lib/server/backend';
import { paraglideMiddleware } from '$lib/paraglide/server';
import { setLocale, getLocale } from '$lib/paraglide/runtime';
import { isLocale } from '$lib/i18n/locale';

// RFC 7230 §6.1 hop-by-hop headers — a proxy must not forward them.
const HOP_BY_HOP = [
  'connection',
  'keep-alive',
  'proxy-authenticate',
  'proxy-authorization',
  'te',
  'trailer',
  'transfer-encoding',
  'upgrade',
  'host'
];

// Forwards /api/* to the backend. Mirrors the production reverse-proxy
// topology in dev, so browser code can call relative URLs like
// `/api/packs` even though the backend container port is never published
// to the host. WebSocket upgrades on /api/ws/* are not handled here —
// SvelteKit's `handle` hook only sees fully-parsed HTTP requests.
async function proxyToBackend(event: RequestEvent): Promise<Response> {
  const reqHeaders = new Headers(event.request.headers);
  for (const h of HOP_BY_HOP) reqHeaders.delete(h);

  const method = event.request.method;
  const body =
    method === 'GET' || method === 'HEAD' ? undefined : await event.request.arrayBuffer();

  const url = `${API_BASE}${event.url.pathname}${event.url.search}`;

  let res: Response;
  try {
    res = await fetch(url, { method, headers: reqHeaders, body, redirect: 'manual' });
  } catch (e) {
    console.error(`[api-proxy] ${method} ${event.url.pathname}`, e);
    return new Response('Backend unreachable', { status: 502 });
  }

  const resHeaders = new Headers(res.headers);
  for (const h of HOP_BY_HOP) resHeaders.delete(h);

  return new Response(res.body, {
    status: res.status,
    statusText: res.statusText,
    headers: resHeaders
  });
}

async function appHandle(event: RequestEvent, resolve: Parameters<Handle>[0]['resolve']) {
  // Generate per-request CSP nonce
  const nonce = randomBytes(16).toString('base64');
  event.locals.nonce = nonce;

  // Load session from backend (session cookie is HttpOnly — forwarded automatically).
  //
  // We distinguish failure modes carefully:
  //   - 200          → user is signed in, hydrate locals.user
  //   - 401          → user is definitively logged out (no / expired session)
  //   - 429 / 5xx    → introspection failed for transient reasons; we cannot
  //                    confirm the session, so we log loudly and treat as
  //                    logged out (we have no prior state to fall back on),
  //                    but operators need to see this so it can be fixed.
  //   - network fail → same as transient — log, treat as logged out.
  try {
    const res = await fetch(`${API_BASE}/api/auth/me`, {
      headers: { cookie: event.request.headers.get('cookie') ?? '' }
    });
    if (res.ok) {
      event.locals.user = await res.json();
    } else {
      if (res.status !== 401) {
        console.warn(
          `[auth] /api/auth/me returned ${res.status} for ${event.url.pathname} — session treated as anonymous`
        );
      }
      event.locals.user = null;
    }
  } catch (err) {
    console.warn(`[auth] /api/auth/me unreachable for ${event.url.pathname}:`, err);
    event.locals.user = null;
  }

  // Authenticated-user locale wins over Paraglide's strategy chain result.
  // Cookie → preferredLanguage → baseLocale already ran in the middleware
  // wrapper below; if the signed-in user has a stored preference, override.
  if (event.locals.user && isLocale(event.locals.user.locale)) {
    const current = getLocale();
    if (current !== event.locals.user.locale) {
      setLocale(event.locals.user.locale, { reload: false });
    }
  }

  return resolve(event, {
    transformPageChunk: ({ html }) => html.replace('%sveltekit.nonce%', nonce)
  });
}

export const handle: Handle = async ({ event, resolve }) => {
  if (event.url.pathname.startsWith('/api/')) {
    return proxyToBackend(event);
  }

  return paraglideMiddleware(event.request, ({ request }) => {
    event.request = request;
    return appHandle(event, resolve);
  });
};

// Forward the browser's session cookie on every server-side `event.fetch`
// call to the backend. SvelteKit's default only forwards cookies to the app's
// own origin or subdomains of it; our Dockerised backend (`http://backend:8080`)
// is neither, so without this hook every authenticated server load silently
// becomes a 401.
export const handleFetch: HandleFetch = async ({ event, request, fetch }) => {
  if (request.url.startsWith(API_BASE)) {
    const cookie = event.request.headers.get('cookie');
    if (cookie) request.headers.set('cookie', cookie);
  }
  return fetch(request);
};
