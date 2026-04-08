import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => {
  const [statsRes, auditRes] = await Promise.all([
    fetch('/api/admin/stats'),
    fetch('/api/admin/audit-log?limit=10')
  ]);

  const stats = statsRes.ok ? await statsRes.json() : null;
  const auditLog = auditRes.ok ? await auditRes.json() : [];

  return { stats, auditLog };
};
