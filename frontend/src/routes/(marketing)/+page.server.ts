import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

// Authed visitors land on their dashboard; anonymous visitors see the
// marketing splash. `?preview=1` escapes the redirect so you can preview
// the marketing page while signed in.
export const load: PageServerLoad = async ({ locals, url }) => {
  if (locals.user && url.searchParams.get('preview') !== '1') {
    throw redirect(303, '/home');
  }
  return {};
};
