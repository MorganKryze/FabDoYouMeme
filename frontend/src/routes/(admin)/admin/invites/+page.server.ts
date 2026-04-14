import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import type { Invite } from '$lib/api/types';
import { apiFetch } from '$lib/server/backend';

export const load: PageServerLoad = async ({ fetch }) => {
  // Backend returns `{data, total, next_cursor}` — unwrap so the component
  // receives a plain `Invite[]`, otherwise `{#each invites}` silently fails.
  const envelope = await apiFetch<{ data: Invite[]; total: number; next_cursor: string | null }>(
    fetch,
    '/api/admin/invites'
  );
  return { invites: envelope.data ?? [] };
};

async function readBackendError(res: Response, fallback: string): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string; code?: string };
    if (body?.error) return body.code ? `${body.error} (${body.code})` : body.error;
  } catch {
    /* non-JSON body — fall through */
  }
  return `${fallback} (HTTP ${res.status})`;
}

export const actions: Actions = {
  createInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const label = (data.get('label') as string | null)?.trim() || null;
    const restricted_email =
      (data.get('restricted_email') as string | null)?.trim() || null;
    const max_uses_raw = data.get('max_uses');
    const max_uses =
      max_uses_raw === null || max_uses_raw === '' ? 0 : Number(max_uses_raw);
    if (!Number.isFinite(max_uses) || max_uses < 0) {
      return fail(400, { createError: 'Max uses must be a non-negative number.' });
    }

    // Backend expects RFC3339. `<input type="datetime-local">` yields
    // "YYYY-MM-DDTHH:mm" with no seconds/timezone; append them so Go's
    // time.Parse accepts it.
    const expires_at_raw = (data.get('expires_at') as string | null) || null;
    const expires_at = expires_at_raw
      ? new Date(expires_at_raw).toISOString()
      : null;

    const res = await fetch('/api/admin/invites', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, restricted_email, max_uses, expires_at })
    });

    if (!res.ok) {
      const msg = await readBackendError(res, 'Failed to create invite');
      return fail(res.status, { createError: msg });
    }
    const invite = await res.json();
    return { created: invite };
  },

  revokeInvite: async ({ request, fetch }) => {
    const data = await request.formData();
    const inviteId = data.get('invite_id') as string;

    const res = await fetch(`/api/admin/invites/${inviteId}`, {
      method: 'DELETE'
    });
    if (!res.ok) {
      const msg = await readBackendError(res, 'Failed to revoke invite');
      return fail(res.status, { revokeError: msg });
    }
    return { revoked: inviteId };
  }
};
