import * as m from '$lib/paraglide/messages';

// Map backend error codes (REST + WebSocket) to localized messages. See
// docs/reference/error-codes.md for the canonical list. Any unknown code
// falls back to the server-supplied message so new codes surface safely
// (and legibly in English, which is the backend's own language).
export function errorMessage(code: string | undefined, serverMessage?: string): string {
  const key = 'errors_' + (code ?? '').toLowerCase();
  const fn = (m as unknown as Record<string, (() => string) | undefined>)[key];
  if (fn) return fn();
  if (serverMessage && serverMessage.trim().length > 0) return serverMessage;
  return m.errors_generic();
}
