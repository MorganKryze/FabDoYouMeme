// frontend/src/routes/(public)/auth/verify/+page.server.ts
import { redirect, fail } from '@sveltejs/kit';
import { dev } from '$app/environment';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';
import { parse as parseCookies } from 'cookie';

export const load: PageServerLoad = async ({ locals, url }) => {
  // Already signed in — skip the "click to log in" ceremony entirely.
  if (locals.user) throw redirect(303, '/home');
  return {
    token: url.searchParams.get('token') ?? '',
    next: url.searchParams.get('next') ?? '/home'
  };
};

export const actions: Actions = {
  default: async ({ request, fetch, cookies }) => {
    const data = await request.formData();
    const token = (data.get('token') as string | null) ?? '';
    const next = (data.get('next') as string | null) ?? '/home';

    const res = await fetch(`${API_BASE}/api/auth/verify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token })
    });

    if (!res.ok) {
      let code = 'invalid_token';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        // ignore
      }
      return fail(400, { error: code });
    }

    // The backend sets the session cookie on its own response, but SvelteKit does not
    // automatically forward Set-Cookie headers from proxied fetches to the browser.
    // Parse the raw header and re-issue it via cookies.set() so the browser receives it.
    const rawCookie = res.headers.get('set-cookie') ?? '';
    const maxAgeMatch = rawCookie.match(/[Mm]ax-[Aa]ge=(\d+)/);
    const parsed = parseCookies(rawCookie.split(';')[0]);
    const sessionValue = parsed['session'];

    if (sessionValue) {
      cookies.set('session', sessionValue, {
        path: '/',
        httpOnly: true,
        // Browsers silently drop `Secure` cookies on insecure origins, so in dev
        // (plain HTTP) we must not set the flag — otherwise the user appears to
        // "log in" successfully but the cookie never reaches the browser.
        secure: !dev,
        // Lax (not Strict): F5 / address-bar reloads have a null initiator, and
        // Chromium drops Strict cookies on those navigations — users appear
        // logged-out after every refresh even though the cookie is still in the
        // jar. Lax still blocks cookies on cross-site POSTs, which is the
        // real CSRF vector. Must match the backend attribute in
        // backend/internal/auth/tokens.go:setSessionCookie.
        sameSite: 'lax',
        maxAge: maxAgeMatch ? parseInt(maxAgeMatch[1]) : 720 * 3600
      });
    }

    throw redirect(303, next.startsWith('/') ? next : '/home');
  }
};
