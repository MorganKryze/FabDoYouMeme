import type { LayoutServerLoad } from './$types';

// Marketing group is publicly accessible. We still read the session so the
// navbar can swap "Sign in" for a "Dashboard" link when the visitor happens
// to already be logged in.
export const load: LayoutServerLoad = async ({ locals }) => {
  return { user: locals.user ?? null };
};
