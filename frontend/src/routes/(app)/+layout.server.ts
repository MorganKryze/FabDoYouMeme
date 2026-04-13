import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals, url }) => {
  // Guest bypass: anonymous visitors reaching a room via the /join flow
  // carry `?as=guest` and only target /rooms/{code}. Any other (app) path
  // still requires a session — the auth gate is unchanged elsewhere.
  const isGuestRoomVisit =
    url.pathname.startsWith('/rooms/') && url.searchParams.get('as') === 'guest';

  if (!locals.user) {
    if (isGuestRoomVisit) {
      return { user: null, isGuest: true as const };
    }
    const next = url.pathname + url.search;
    throw redirect(303, `/auth/magic-link?next=${encodeURIComponent(next)}`);
  }
  return { user: locals.user, isGuest: false as const };
};
