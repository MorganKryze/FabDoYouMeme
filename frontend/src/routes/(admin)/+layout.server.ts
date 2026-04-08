import { error } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals }) => {
  if (!locals.user) {
    throw error(401, 'Not authenticated');
  }
  if (locals.user.role !== 'admin') {
    throw error(403, 'Admin access required');
  }
  return { user: locals.user };
};
