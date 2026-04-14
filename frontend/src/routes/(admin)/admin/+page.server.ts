import type { PageServerLoad } from './$types';
import { apiFetch, API_BASE } from '$lib/server/backend';
import type {
  DeepHealthResponse,
  AdminStats,
  AuditEntry
} from '$lib/api/types';

export const load: PageServerLoad = async ({ fetch }) => {
  // Three independent reads — parallelize so SSR isn't bottlenecked by the
  // slowest probe (typically /api/health/deep if SMTP is sluggish).
  const [health, stats, audit] = await Promise.all([
    // /api/health/deep returns 503 when any probe is degraded; bypass apiFetch
    // (which throws on non-2xx) and read the body unconditionally.
    fetch(`${API_BASE}/api/health/deep`)
      .then((r) => r.json() as Promise<DeepHealthResponse>)
      .catch(() => null),
    apiFetch<AdminStats>(fetch, '/api/admin/stats').catch(() => null),
    apiFetch<{ data: AuditEntry[] }>(fetch, '/api/admin/audit?limit=10')
      .then((r) => r.data)
      .catch(() => [] as AuditEntry[])
  ]);

  return {
    stats,
    health,
    audit
  };
};
