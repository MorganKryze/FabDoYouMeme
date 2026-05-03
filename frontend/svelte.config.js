import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter(),
    csp: {
      mode: 'nonce',
      directives: {
        'default-src': ['self'],
        'script-src': ['self'],
        'style-src': ['self'],
        'style-src-elem': ['self'],
        'style-src-attr': ['unsafe-inline'],
        'font-src': ['self'],
        'img-src': ['self', 'data:', 'blob:'],
        'media-src': ['self'],
        'connect-src': ['self', 'wss:', 'ws:'],
        'frame-ancestors': ['none']
      }
    }
  }
};

export default config;
