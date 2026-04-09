// frontend/src/routes/(public)/auth/verify/+page.server.ts
import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';
import { parse as parseCookies } from 'cookie';

export const load: PageServerLoad = async ({ url }) => {
  return {
    token: url.searchParams.get('token') ?? '',
    next: url.searchParams.get('next') ?? '/'
  };
};

export const actions: Actions = {
  default: async ({ request, fetch, cookies }) => {
    const data = await request.formData();
    const token = (data.get('token') as string | null) ?? '';
    const next = (data.get('next') as string | null) ?? '/';

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
        secure: true,
        sameSite: 'strict',
        maxAge: maxAgeMatch ? parseInt(maxAgeMatch[1]) : 720 * 3600
      });
    }

    throw redirect(303, next.startsWith('/') ? next : '/');
  }
};
