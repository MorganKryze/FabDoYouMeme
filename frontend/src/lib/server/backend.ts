// Server-side only — provides the internal Docker URL to reach the backend.
// Never import this from client-side code.
import { env } from '$env/dynamic/private';

export const API_BASE = env.API_URL ?? 'http://localhost:8080';
