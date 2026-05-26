/**
 * Proxy all /api/whisper/* requests to the upstream URL supplied by the
 * client in the X-Proxy-Target header.  This keeps every request same-origin
 * from the browser's perspective, so CORS is never an issue regardless of
 * where the Whisper service is actually running.
 */

import { proxyTo } from '$lib/server/proxy.js';

/** @type {import('./$types').RequestHandler} */
export const fallback = ({ request, params, url }) =>
  proxyTo(request, params.path, url.search);
