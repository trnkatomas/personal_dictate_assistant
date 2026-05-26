/**
 * Generic server-side proxy helper.
 *
 * The client puts the real upstream base URL in an X-Proxy-Target header
 * (e.g. "http://whisper:9000" or "https://xyz.proxy.runpod.net").
 * This function strips that header, constructs the full upstream URL, and
 * tunnels the request/response — including streaming bodies — transparently.
 *
 * Running the proxy on the SvelteKit server means the browser always talks
 * to the same origin, so CORS never enters the picture no matter what URL
 * the user configures at runtime.
 */

// Headers that must not be forwarded between hops.
const HOP_BY_HOP = new Set([
  'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization',
  'te', 'trailer', 'transfer-encoding', 'upgrade'
]);

/**
 * @param {Request} request   - the incoming SvelteKit request
 * @param {string}  path      - the [...path] rest param (may be empty)
 * @param {string}  search    - url.search (e.g. "?task=transcribe")
 * @returns {Promise<Response>}
 */
export async function proxyTo(request, path, search) {
  const target = request.headers.get('x-proxy-target');
  if (!target) {
    return new Response('Missing X-Proxy-Target header', { status: 400 });
  }

  const base     = target.replace(/\/+$/, '');          // strip trailing slash
  const suffix   = path ? `/${path}` : '';
  const upstream = `${base}${suffix}${search}`;
  console.log(upstream);

  // Forward all headers except hop-by-hop ones and our custom proxy header.
  const fwdHeaders = new Headers();
  for (const [k, v] of request.headers) {
    const lk = k.toLowerCase();
    if (lk === 'host' || lk === 'x-proxy-target' || HOP_BY_HOP.has(lk)) continue;
    fwdHeaders.set(k, v);
  }

  /** @type {RequestInit & { duplex?: string }} */
  const init = { method: request.method, headers: fwdHeaders };
  if (request.method !== 'GET' && request.method !== 'HEAD') {
    init.body   = request.body;
    init.duplex = 'half'; // required by Node for streaming request bodies
  }

  let upstreamRes;
  try {
    upstreamRes = await fetch(upstream, init);
  } catch (err) {
    return new Response(`Proxy error: ${err.message}`, { status: 502 });
  }

  // Strip hop-by-hop headers from the upstream response.
  const resHeaders = new Headers();
  for (const [k, v] of upstreamRes.headers) {
    if (!HOP_BY_HOP.has(k.toLowerCase())) resHeaders.set(k, v);
  }

  // Pass the body (and streaming body) straight through.
  return new Response(upstreamRes.body, {
    status:  upstreamRes.status,
    headers: resHeaders
  });
}
