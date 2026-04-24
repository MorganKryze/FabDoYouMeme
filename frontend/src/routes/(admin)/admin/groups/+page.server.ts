// frontend/src/routes/(admin)/admin/groups/+page.server.ts
// Phase 5 — platform-admin overview of every group on the instance.
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { apiFetch, API_BASE } from '$lib/server/backend';

type AdminGroupRow = {
  id: string;
  name: string;
  description: string;
  classification: 'sfw' | 'nsfw';
  language: 'en' | 'fr' | 'multi';
  member_cap: number;
  quota_bytes: number;
  created_at: string;
  deleted_at: string | null;
  member_count: number;
};

export const load: PageServerLoad = async ({ fetch }) => {
  const data = await apiFetch<{ data: AdminGroupRow[]; next_cursor: string | null }>(
    fetch,
    '/api/admin/groups?limit=100'
  );
  return { groups: data.data ?? [] };
};

export const actions: Actions = {
  setQuota: async ({ request, fetch }) => {
    const data = await request.formData();
    const gid = data.get('group_id') as string;
    const quotaBytes = Number(data.get('quota_bytes'));
    if (!Number.isFinite(quotaBytes) || quotaBytes < 0) {
      return fail(400, { error: 'quota_bytes must be a non-negative integer' });
    }
    const res = await fetch(`${API_BASE}/api/admin/groups/${gid}/quota`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ quota_bytes: quotaBytes })
    });
    if (!res.ok) {
      return fail(res.status, { error: `Quota update failed (${res.status})` });
    }
    return { ok: true };
  },
  setMemberCap: async ({ request, fetch }) => {
    const data = await request.formData();
    const gid = data.get('group_id') as string;
    const memberCap = Number(data.get('member_cap'));
    if (!Number.isFinite(memberCap) || memberCap <= 0) {
      return fail(400, { error: 'member_cap must be a positive integer' });
    }
    const res = await fetch(`${API_BASE}/api/admin/groups/${gid}/member_cap`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ member_cap: memberCap })
    });
    if (!res.ok) {
      let code = 'unknown';
      try {
        const body = await res.json();
        code = body.code ?? code;
      } catch {
        /* ignore */
      }
      const messages: Record<string, string> = {
        member_cap_below_current:
          'Cannot set the cap below the current member count. Remove members first or raise the cap.'
      };
      return fail(res.status, { error: messages[code] ?? `Member-cap update failed (${res.status})` });
    }
    return { ok: true };
  }
};
