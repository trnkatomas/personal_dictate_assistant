import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

// No proxy config needed: /api/whisper/* and /api/ollama/* are handled by
// SvelteKit server routes in src/routes/api/, both in dev and production.
export default defineConfig({
  plugins: [sveltekit()]
});
