import type { Handle, HandleFetch, RequestEvent } from '@sveltejs/kit';
import { randomBytes } from 'node:crypto';
import { AsyncLocalStorage } from 'node:async_hooks';
import { API_BASE } from '$lib/server/backend';
import {
  cookieName as paraglideCookieName,
  serverAsyncLocalStorage,
  overwriteServerAsyncLocalStorage
} from '$lib/paraglide/runtime';
import { isLocale, defaultLocale, type Locale } from '$lib/i18n/locale';

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
//
// We also stamp X-Forwarded-For with the real client IP. Without this the
// backend's ClientIP resolves to the SvelteKit container's docker IP, so
// every guest/anonymous caller shares one rate-limit bucket — the
// platform-wide ceiling that masquerades as "rate limit too low".
// Operators must list the SvelteKit container's network in the backend's
// TRUSTED_PROXIES for the header to be honoured (see docs/self-hosting.md).
async function proxyToBackend(event: RequestEvent): Promise<Response> {
  const reqHeaders = new Headers(event.request.headers);
  for (const h of HOP_BY_HOP) reqHeaders.delete(h);
  appendForwardedFor(reqHeaders, event);

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

// Append the client IP to X-Forwarded-For. Preserves anything Pangolin
// (or any upstream proxy) already set, so the backend can walk the chain
// when TRUSTED_PROXIES is configured to include the SvelteKit hop.
// `event.getClientAddress()` honours adapter-node's ADDRESS_HEADER /
// XFF_DEPTH config; if those aren't set it falls back to the immediate
// peer, which in practice is the reverse proxy.
function appendForwardedFor(headers: Headers, event: RequestEvent): void {
  let client: string;
  try {
    client = event.getClientAddress();
  } catch {
    return; // adapter doesn't expose it (e.g. some test harnesses) — skip
  }
  if (!client) return;
  const existing = headers.get('x-forwarded-for');
  headers.set('x-forwarded-for', existing ? `${existing}, ${client}` : client);
}

async function hydrateSession(event: RequestEvent) {
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
    const meHeaders = new Headers({ cookie: event.request.headers.get('cookie') ?? '' });
    appendForwardedFor(meHeaders, event);
    const res = await fetch(`${API_BASE}/api/auth/me`, { headers: meHeaders });
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
}

// Read our own Paraglide cookie from the request. Paraglide v2's compiled
// `extractLocaleFromCookie` is client-only (reads `document.cookie`); its
// server-side cookie path lives inside the middleware. We bypass the
// middleware so we can override with `user.locale` *before* locale resolution,
// which means we have to read the cookie ourselves here.
function extractParaglideCookie(req: Request): Locale | null {
  const header = req.headers.get('cookie');
  if (!header) return null;
  for (const part of header.split(';')) {
    const trimmed = part.trim();
    if (trimmed.startsWith(`${paraglideCookieName}=`)) {
      const value = trimmed.slice(paraglideCookieName.length + 1);
      return isLocale(value) ? value : null;
    }
  }
  return null;
}

// Negotiate Accept-Language against our supported locales. Lightweight — we
// only support two locales, so the first q-valued match wins.
function negotiateAcceptLanguage(req: Request): Locale | null {
  const header = req.headers.get('accept-language');
  if (!header) return null;
  const entries = header
    .split(',')
    .map((raw) => {
      const [tag, ...params] = raw.trim().split(';');
      const q = params.find((p) => p.trim().startsWith('q='));
      const qv = q ? Number(q.trim().slice(2)) : 1;
      return { tag: tag.toLowerCase(), q: Number.isFinite(qv) ? qv : 1 };
    })
    .sort((a, b) => b.q - a.q);
  for (const { tag } of entries) {
    const base = tag.split('-')[0];
    if (isLocale(base)) return base;
  }
  return null;
}

function resolveLocale(event: RequestEvent): Locale {
  if (event.locals.user && isLocale(event.locals.user.locale)) {
    return event.locals.user.locale;
  }
  const fromCookie = extractParaglideCookie(event.request);
  if (fromCookie) return fromCookie;
  const fromHeader = negotiateAcceptLanguage(event.request);
  if (fromHeader) return fromHeader;
  return defaultLocale();
}

// Paraglide's generated runtime leaves `serverAsyncLocalStorage` undefined
// and expects the middleware to populate it on first call. We bypass the
// middleware, so we initialize it ourselves exactly once per process.
if (!serverAsyncLocalStorage) {
  overwriteServerAsyncLocalStorage(new AsyncLocalStorage());
}

export const handle: Handle = async ({ event, resolve }) => {
  const nonce = randomBytes(16).toString('base64');
  event.locals.nonce = nonce;

  if (event.url.pathname.startsWith('/api/')) {
    return proxyToBackend(event);
  }

  await hydrateSession(event);
  const locale = resolveLocale(event);

  return serverAsyncLocalStorage!.run(
    { locale, origin: event.url.origin, messageCalls: new Set() },
    () =>
      resolve(event, {
        transformPageChunk: ({ html }) =>
          html.replace('%sveltekit.nonce%', nonce).replace('%lang%', locale)
      })
  );
};

// Forward the browser's session cookie *and* X-Forwarded-For on every
// server-side `event.fetch` call to the backend. SvelteKit's default only
// forwards cookies to the app's own origin or subdomains of it; our
// Dockerised backend (`http://backend:8080`) is neither, so without this
// hook every authenticated server load silently becomes a 401. The XFF
// stamp lets the backend's rate limiter see the real client IP rather
// than the SvelteKit container — see proxyToBackend for the same
// reasoning on the browser-→-backend proxy path.
export const handleFetch: HandleFetch = async ({ event, request, fetch }) => {
  if (request.url.startsWith(API_BASE)) {
    const cookie = event.request.headers.get('cookie');
    if (cookie) request.headers.set('cookie', cookie);
    appendForwardedFor(request.headers, event);
  }
  return fetch(request);
};
