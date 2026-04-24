// frontend/src/routes/(app)/groups/[gid]/pending/+page.ts
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params }) => ({ gid: params.gid });
