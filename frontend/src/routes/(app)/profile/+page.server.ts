import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ locals }) => {
  return { user: locals.user! };
};

export const actions: Actions = {
  updateUsername: async ({ request, fetch }) => {
    const data = await request.formData();
    const username = (data.get('username') as string | null) ?? '';

    const res = await fetch(`${API_BASE}/api/users/me`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username })
    });

    if (!res.ok) {
      let code = 'error';
      try {
        const b = await res.json();
        code = b.code ?? code;
      } catch {
        /**/
      }
      if (code === 'username_taken') {
        return fail(409, { usernameError: 'That username is already taken.' });
      }
      return fail(400, { usernameError: 'Failed to update username.' });
    }
    return { usernameSuccess: true };
  },

  requestEmailChange: async ({ request, fetch }) => {
    const data = await request.formData();
    const email = (data.get('email') as string | null) ?? '';

    const res = await fetch(`${API_BASE}/api/users/me`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    });

    if (!res.ok) {
      return fail(400, { emailError: 'Failed to send verification email.' });
    }
    return { emailSent: true };
  },

  updateLocale: async ({ request, fetch, cookies }) => {
    const data = await request.formData();
    const locale = (data.get('locale') as string | null) ?? '';
    if (locale !== 'en' && locale !== 'fr') {
      return fail(400, { localeError: 'Locale must be en or fr.' });
    }

    const res = await fetch(`${API_BASE}/api/users/me`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ locale })
    });

    if (!res.ok) {
      return fail(400, { localeError: 'Failed to update language.' });
    }

    // Mirror the DB-stored preference into the Paraglide cookie. Must be
    // readable from client JS: Paraglide's client-side locale strategy
    // reads `document.cookie` to pick the locale on hydration, so an
    // HttpOnly cookie would cause SSR to render FR and hydration to flip
    // back to the base locale.
    cookies.set('PARAGLIDE_LOCALE', locale, {
      path: '/',
      maxAge: 60 * 60 * 24 * 365,
      httpOnly: false,
      sameSite: 'lax'
    });

    return { localeSuccess: true };
  }
};
