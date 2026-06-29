import { get } from 'svelte/store';
import { settings } from './stores.js';

/**
 * POST audio blob to Whisper ASR and return the transcribed text string.
 *
 * The request goes to our own SvelteKit server at /api/whisper/* which then
 * proxies it to wherever `settings.whisperUrl` points — no CORS, ever.
 *
 * @param {Blob} blob
 * @returns {Promise<string>}
 */
export async function transcribe(blob) {
  const s = get(settings);

  const form = new FormData();
  form.append('audio_file', blob, 'recording.webm');

  const params = new URLSearchParams({ task: s.whisperTask, output: 'json', 'encode': true });
  if (s.whisperLanguage) params.set('language', s.whisperLanguage);


  const res = await fetch(`/api/whisper/asr?${params}`, {
    method: 'POST',
    headers: { 'x-proxy-target': s.whisperUrl },
    body: form
  });

  if (!res.ok) {
    const msg = await res.text().catch(() => res.statusText);
    throw new Error(`Whisper ${res.status}: ${msg}`);
  }

  const data = await res.json();
  return (data.text ?? '').trim();
}
