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

/**
 * Stream tokens from Ollama generate API.
 * Yields each text token as it arrives.
 *
 * @param {string} text       - raw transcription to transform
 * @param {string} promptText - the instruction prompt
 * @returns {AsyncGenerator<string>}
 */
export async function* transform(text, promptText) {
  const s = get(settings);

  const res = await fetch(`/api/ollama/api/generate`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-proxy-target': s.ollamaUrl
    },
    body: JSON.stringify({
      model: s.ollamaModel,
      prompt: `${promptText}\n\nText to process:\n${text}`,
      stream: true
    })
  });

  if (!res.ok) {
    const msg = await res.text().catch(() => res.statusText);
    throw new Error(`Ollama ${res.status}: ${msg}`);
  }

  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buf += dec.decode(value, { stream: true });
    const lines = buf.split('\n');
    buf = lines.pop() ?? '';           // keep incomplete last line

    for (const line of lines) {
      if (!line.trim()) continue;
      try {
        const obj = JSON.parse(line);
        if (obj.response) yield obj.response;
        if (obj.done) return;
      } catch { /* ignore malformed lines */ }
    }
  }
}
