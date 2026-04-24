// frontend/src/routes/(app)/groups/[gid]/+page.ts
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params }) => ({ gid: params.gid });
