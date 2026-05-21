import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [sveltekit()],
  server: {
    proxy: {
      // Mirror the nginx proxy paths for local `npm run dev`
      '/api/whisper': {
        target: 'http://localhost:9000',
        rewrite: (path) => path.replace(/^\/api\/whisper/, '')
      },
      '/api/ollama': {
        target: 'http://localhost:11434',
        rewrite: (path) => path.replace(/^\/api\/ollama/, '')
      }
    }
  }
});
