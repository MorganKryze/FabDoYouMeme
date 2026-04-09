// frontend/src/routes/(public)/auth/register/+page.server.ts
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE } from '$lib/server/backend';

const ERROR_MESSAGES: Record<string, string> = {
  invalid_invite: 'That invite token is invalid, expired, or already used.',
  consent_required: 'You must agree to the Privacy Policy to register.',
  age_affirmation_required: 'You must confirm you are at least 16 years old.',
  username_taken: 'That username is already taken. Please choose another.'
};

export const load: PageServerLoad = async ({ url }) => {
  return {
    inviteToken: url.searchParams.get('invite') ?? ''
  };
};

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const invite_token = (data.get('invite_token') as string | null) ?? '';
    const username = (data.get('username') as string | null) ?? '';
    const email = (data.get('email') as string | null) ?? '';
    const consent = data.get('consent') === 'true';
    const age_affirmation = data.get('age_affirmation') === 'true';

    const res = await fetch(`${API_BASE}/api/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        invite_token,
        username,
        email,
        consent,
        age_affirmation
      })
    });

    if (!res.ok) {
      let code = 'unknown_error';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        // ignore parse failure
      }
      return fail(res.status, {
        invite_token,
        username,
        email,
        error: ERROR_MESSAGES[code] ?? 'Registration failed. Please try again.',
        consent: data.get('consent') === 'on',
        age_affirmation: data.get('age_affirmation') === 'on',
      });
    }

    const body = await res.json();
    return {
      success: true,
      warning: body.warning ?? null
    };
  }
};
