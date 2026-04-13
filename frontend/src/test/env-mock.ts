// Static stub for SvelteKit's `$env/dynamic/public` so vitest does not
// need to spin up the SvelteKit runtime to resolve public env vars.
export const env = {
  PUBLIC_API_URL: 'http://localhost:8080',
  PUBLIC_OPERATOR_NAME: 'Test Operator',
  PUBLIC_OPERATOR_CONTACT_EMAIL: 'test@example.com',
  PUBLIC_OPERATOR_URL: 'http://localhost:3000',
  PUBLIC_OPERATOR_SMTP_PROVIDER: 'Test SMTP Provider'
};
