// Static stub for SvelteKit's `$env/dynamic/public` so vitest does not
// need to spin up the SvelteKit runtime to resolve public env vars.
export const env = {
  PUBLIC_API_URL: 'http://localhost:8080'
};
