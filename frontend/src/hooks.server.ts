import type { Handle, HandleFetch } from '@sveltejs/kit';
import { randomBytes } from 'node:crypto';
import { API_BASE } from '$lib/server/backend';

export const handle: Handle = async ({ event, resolve }) => {
  // Generate per-request CSP nonce
  const nonce = randomBytes(16).toString('base64');
  event.locals.nonce = nonce;

  // Load session from backend (session cookie is HttpOnly — forwarded automatically)
  try {
    const res = await fetch(`${API_BASE}/api/auth/me`, {
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
