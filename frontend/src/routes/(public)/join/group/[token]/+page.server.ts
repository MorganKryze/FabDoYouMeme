// frontend/src/routes/(public)/join/group/[token]/+page.server.ts
//
// Phase 2 of the groups paradigm. The token in the URL is the only identity
// the server has at this point — preview is unauthenticated by design (the
// recipient may not be logged in yet, or may not have an account at all).
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { apiFetch } from '$lib/server/backend';
import type { InvitePreview } from '$lib/api/groups';

export const load: PageServerLoad = async ({ params, fetch, locals, url }) => {
  const token = params.token;

  // platform_plus_group codes are redeemed via the registration flow; if
  // someone clicks a /join/group/<token> URL with that kind of code we
  // route them to /auth/register so they create their account first.
  let preview: InvitePreview;
  try {
    preview = await apiFetch<InvitePreview>(
      fetch,
      `/api/groups/invites/preview?token=${encodeURIComponent(token)}`
    );
  } catch {
    throw error(404, 'Invite not found');
  }
  if (preview.invite_kind === 'platform_plus_group') {
    throw redirect(303, `/auth/register?group_invite_token=${encodeURIComponent(token)}`);
  }

  // group_join codes need an existing session. Bounce through the magic-
  // link flow with a `next` redirect so login lands the user back here.
  if (!locals.user) {
    const next = url.pathname + url.search;
    throw redirect(303, `/auth/magic-link?next=${encodeURIComponent(next)}`);
  }

  return { token, preview };
};
