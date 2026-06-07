import adapter from '@sveltejs/adapter-node';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    adapter: adapter(),
    csrf: {
      // The proxy routes are designed to receive multipart form POSTs from the browser.
      // Cross-origin concerns are handled at the server layer (browser → SvelteKit is
      // always same-origin; SvelteKit → Whisper/Ollama is server-to-server).
      // SvelteKit's CSRF origin check is therefore redundant here and must be off.
      checkOrigin: false
    }
  }
};

export default config;
