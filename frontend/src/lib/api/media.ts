// Helpers for rendering media URLs from the backend.
//
// The backend emits media_url as `/api/assets/media?key=<object-key>` (see
// assets.go MediaURL). Registered users carry the session cookie automatically
// on <img> fetches; guests have no cookie and must pass their per-room guest
// token as a query param so GetMedia can resolve identity. This helper bakes
// that token into the URL when the caller is a guest in `roomCode`.

import { guest } from '$lib/state/guest.svelte';

/**
 * Return the image-loadable URL for a server-emitted media_url. Pass the
 * current room code so we can look up the guest token for that room; if no
 * token exists (registered user, or URL is absolute/external) the input is
 * returned unchanged.
 */
export function mediaSrc(mediaUrl: string | null | undefined, roomCode: string | null | undefined): string {
  if (!mediaUrl) return '';
  if (!roomCode) return mediaUrl;
  // Only rewrite our own backend media URLs — leave absolute URLs alone.
  if (!mediaUrl.startsWith('/api/assets/media')) return mediaUrl;
  const token = guest.token(roomCode);
  if (!token) return mediaUrl;
  const sep = mediaUrl.includes('?') ? '&' : '?';
  return `${mediaUrl}${sep}guest_token=${encodeURIComponent(token)}`;
}
