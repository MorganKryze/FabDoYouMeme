import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals, url }) => {
  if (!locals.user) {
    const next = url.pathname + url.search;
    throw redirect(303, `/auth/magic-link?next=${encodeURIComponent(next)}`);
  }
  return { user: locals.user };
};
