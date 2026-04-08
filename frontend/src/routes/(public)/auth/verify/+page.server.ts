// frontend/src/routes/(public)/auth/verify/+page.server.ts
import { redirect, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
  return {
    token: url.searchParams.get('token') ?? '',
    next: url.searchParams.get('next') ?? '/'
  };
};

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const token = (data.get('token') as string | null) ?? '';
    const next = (data.get('next') as string | null) ?? '/';

    const res = await fetch('/api/auth/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      // SvelteKit's event.fetch propagates Set-Cookie from same-origin responses automatically
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

    throw redirect(303, next.startsWith('/') ? next : '/');
  }
};
