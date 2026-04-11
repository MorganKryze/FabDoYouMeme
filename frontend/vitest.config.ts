import { defineConfig, mergeConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';
import viteConfig from './vite.config';

export default mergeConfig(
  viteConfig,
  defineConfig({
    resolve: {
      alias: {
        // Avoid SvelteKit's runtime `$env` resolver — use a static stub.
        '$env/dynamic/public': fileURLToPath(
          new URL('./src/test/env-mock.ts', import.meta.url)
        ),
        // Explicit `$lib` alias. The sveltekit() vite plugin normally
        // provides this, but its configResolved-time registration can
        // race with vitest's config merge — pinning it here avoids the
        // intermittent "Cannot find module '$lib/...'" failure.
        $lib: fileURLToPath(new URL('./src/lib', import.meta.url))
      },
      // Force Svelte to resolve its browser entry. Without this, vitest's
      // node environment picks `index-server.js`, and `mount()` throws
      // "lifecycle_function_unavailable" when @testing-library/svelte
      // tries to render a component.
      conditions: ['browser']
    },
    test: {
      environment: 'happy-dom',
      globals: false,
      include: ['src/**/*.{test,spec}.{js,ts}'],
      setupFiles: ['src/test/setup.ts'],
      clearMocks: true,
      restoreMocks: true
    }
  })
);
