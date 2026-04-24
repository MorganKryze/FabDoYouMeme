// frontend/src/routes/(public)/auth/register/+page.server.ts
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { API_BASE, apiFetch } from '$lib/server/backend';
import type { InvitePreview } from '$lib/api/groups';

const ERROR_MESSAGES: Record<string, string> = {
  invalid_invite: 'That invite token is invalid, expired, or already used.',
  consent_required: 'You must agree to the Privacy Policy to register.',
  age_affirmation_required: 'You must confirm you are at least 16 years old.',
  username_taken: 'That username is already taken. Please choose another.',
  // Phase 2 — group_invite_token paths
  invite_revoked: 'That invite has been revoked.',
  invite_expired: 'That invite has expired.',
  invite_exhausted: 'That invite has no remaining uses.',
  wrong_invite_kind: 'That code is for existing users; sign in and redeem it from the app.',
  group_not_found: 'The group this invite points to no longer exists.',
  nsfw_age_affirmation_required: 'You must confirm you are of legal age for adult content to join this group.'
};

export const load: PageServerLoad = async ({ locals, url, fetch }) => {
  if (locals.user) throw redirect(303, '/home');

  // Phase 2: when a platform+group invite token is in the URL we preview
  // the target group identity so the registration page can show "Joining
  // <group name>" + (for NSFW) the age-affirmation checkbox. A failed
  // preview surfaces as an empty groupPreview — the form shows the
  // generic registration UI and the backend rejects the bad token at
  // submit time anyway.
  const groupInviteToken = url.searchParams.get('group_invite_token') ?? '';
  let groupPreview: InvitePreview | null = null;
  if (groupInviteToken) {
    try {
      groupPreview = await apiFetch<InvitePreview>(
        fetch,
        `/api/groups/invites/preview?token=${encodeURIComponent(groupInviteToken)}`
      );
    } catch {
      groupPreview = null;
    }
  }

  return {
    inviteToken: url.searchParams.get('invite') ?? '',
    groupInviteToken,
    groupPreview
  };
};

export const actions: Actions = {
  default: async ({ request, fetch }) => {
    const data = await request.formData();
    const invite_token = (data.get('invite_token') as string | null) ?? '';
    const group_invite_token = (data.get('group_invite_token') as string | null) ?? '';
    const username = (data.get('username') as string | null) ?? '';
    const email = (data.get('email') as string | null) ?? '';
    // HTML checkboxes omit the field entirely when unchecked. Presence = ticked.
    const consent = data.has('consent');
    const age_affirmation = data.has('age_affirmation');
    const nsfw_age_affirmation = data.has('nsfw_age_affirmation');

    const payload: Record<string, unknown> = { username, email, consent, age_affirmation };
    if (group_invite_token) {
      payload.group_invite_token = group_invite_token;
      payload.nsfw_age_affirmation = nsfw_age_affirmation;
    } else {
      payload.invite_token = invite_token;
    }

    const res = await fetch(`${API_BASE}/api/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
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
        group_invite_token,
        username,
        email,
        error: ERROR_MESSAGES[code] ?? 'Registration failed. Please try again.',
        consent,
        age_affirmation,
        nsfw_age_affirmation
      });
    }

    const body = await res.json();
    return {
      success: true,
      warning: body.warning ?? null
    };
  }
};
