// frontend/src/routes/(admin)/admin/danger/+page.ts
import { error } from '@sveltejs/kit';
import { env } from '$env/dynamic/public';

// Prod gate — UX safeguard layered on top of the backend's 404. The
// backend already refuses to mount /api/admin/danger/* routes in prod
// (see cmd/server/main.go), but we want the page itself to be
// unreachable client-side too so the warning banner never flashes mid-
// flight. Fails safe: if PUBLIC_APP_ENV is unset, treat as prod — this
// matches the backend default and guarantees a forgotten env var cannot
// accidentally expose the danger zone.
export const load = () => {
  const appEnv = env.PUBLIC_APP_ENV || 'prod';
  if (appEnv === 'prod') {
    throw error(404, 'Not found');
  }
  return {};
};
