/**
 * Proxy all /api/ollama/* requests to the upstream URL supplied by the
 * client in the X-Proxy-Target header.  Same-origin from the browser,
 * so no CORS.  Streaming (NDJSON) responses pass through unchanged.
 */

import { proxyTo } from '$lib/server/proxy.js';

/** @type {import('./$types').RequestHandler} */
export const fallback = ({ request, params, url }) =>
  proxyTo(request, params.path, url.search);
