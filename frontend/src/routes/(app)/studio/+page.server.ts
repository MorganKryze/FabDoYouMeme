import type { PageServerLoad } from './$types';
import type { Pack } from '$lib/api/types';
import type { GroupListItem } from '$lib/api/groups';
import { apiFetch } from '$lib/server/backend';

export interface StudioGroup {
  id: string;
  name: string;
  classification: 'sfw' | 'nsfw';
  language: 'en' | 'fr' | 'multi';
  member_role: 'admin' | 'member';
}

export const load: PageServerLoad = async ({ fetch, url }) => {
  const cursor = url.searchParams.get('cursor') ?? '';
  const q = cursor ? `?after=${encodeURIComponent(cursor)}` : '';

  // Since ListPacksForUser now includes group-owned packs for caller
  // members, /api/packs alone is enough to populate the studio pack list.
  // /api/groups is still needed for the navigator's per-group metadata
  // (name, member_role) so we can label sections and gate rename/delete.
  const [packsBody, groupsList] = await Promise.all([
    apiFetch<{ data: Pack[]; next_cursor: string | null }>(
      fetch,
      `/api/packs${q}`
    ),
    apiFetch<GroupListItem[]>(fetch, '/api/groups').catch(() => [] as GroupListItem[])
  ]);

  const groups: StudioGroup[] = groupsList.map((g) => ({
    id: g.id,
    name: g.name,
    classification: g.classification,
    language: g.language,
    member_role: g.member_role
  }));

  const packs = packsBody.data ?? [];

  // Deep-link preselection. Returned as a separate field (instead of
  // mutating global state in an onMount) so the page can sync it
  // deterministically inside the same `$effect` that assigns `studio.packs`,
  // avoiding an "effects vs onMount" ordering race on first paint.
  const requestedPackId = url.searchParams.get('pack');
  const preselectedPackId =
    requestedPackId && packs.some((p) => p.id === requestedPackId) ? requestedPackId : null;

  return {
    packs,
    groups,
    preselectedPackId,
    nextCursor: packsBody.next_cursor ?? null
  };
};
