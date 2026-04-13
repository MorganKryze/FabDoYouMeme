// frontend/src/routes/(public)/auth/magic-link/+page.server.ts
import { redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

export const load: PageServerLoad = async ({ locals }) => {
  if (locals.user) throw redirect(303, '/home');
  return {};
};

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const email = (data.get('email') as string | null) ?? '';

    // Fire-and-forget — always show success (no enumeration)
    await fetch(`${API_BASE}/api/auth/magic-link`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email })
    }).catch(() => {
      // Silently ignore network errors — user sees "link is on its way" regardless
    });

    return { sent: true };
  }
};
