import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Invite } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  const invites = await apiFetch<Invite[]>(fetch, '/api/admin/invites');
  return { invites };
};

export const actions: Actions = {
  createInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const label = (data.get('label') as string | null) ?? '';
    const restricted_email =
      (data.get('restricted_email') as string | null) || null;
    const max_uses = Number(data.get('max_uses') ?? 0);
    const expires_at = (data.get('expires_at') as string | null) || null;

    const res = await fetch('/api/admin/invites', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, restricted_email, max_uses, expires_at })
    });

    if (!res.ok)
      return fail(res.status, { createError: 'Failed to create invite.' });
    const invite = await res.json();
    return { created: invite };
  },

  revokeInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const inviteId = data.get('invite_id') as string;

    const res = await fetch(`/api/admin/invites/${inviteId}`, {
      method: 'DELETE'
    });
    if (!res.ok)
      return fail(res.status, { revokeError: 'Failed to revoke invite.' });
    return { revoked: inviteId };
  }
};
