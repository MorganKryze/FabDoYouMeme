// Server-side only — provides the internal Docker URL to reach the backend.
// Never import this from client-side code.
import { error, type NumericRange } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

export const API_BASE = env.API_URL ?? 'http://localhost:8080';

type SvelteFetch = typeof fetch;

/**
 * Server-side wrapper for backend API calls. Logs failures with method/path/
 * status/body context and throws a SvelteKit `error()` on any non-ok or
 * network failure, so problems surface as a visible error page instead of
 * being papered over with empty fallbacks. Use from `+page.server.ts` /
 * `+layout.server.ts` `load` functions. For form `actions`, keep calling
 * `fetch` directly so you can return typed `fail()` results for the form UI.
 */
export async function apiFetch<T = unknown>(
  fetch: SvelteFetch,
  path: string,
  init?: RequestInit
): Promise<T> {
  const method = init?.method ?? 'GET';
  const url = `${API_BASE}${path}`;

  let res: Response;
  try {
    res = await fetch(url, init);
  } catch (e) {
    console.error(`[apiFetch] network error ${method} ${path}`, e);
    throw error(502, 'Backend unreachable');
  }

  if (!res.ok) {
    let body = '';
    try {
      body = (await res.text()).slice(0, 500);
    } catch {
      /* response body already consumed or unreadable */
    }
    console.error(
      `[apiFetch] ${method} ${path} → ${res.status} ${res.statusText}`,
      body
    );
    throw error(
      res.status as NumericRange<400, 599>,
      `Backend ${res.status} ${res.statusText}`
    );
  }

  try {
    return (await res.json()) as T;
  } catch (e) {
    console.error(`[apiFetch] failed to parse JSON ${method} ${path}`, e);
    throw error(502, 'Invalid response from backend');
  }
}
