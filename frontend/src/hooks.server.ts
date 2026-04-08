import type { Handle } from '@sveltejs/kit';
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
