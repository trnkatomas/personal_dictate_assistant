import { get } from 'svelte/store';
import { settings } from './stores.js';
import { Transcribe, GetSetupState } from '../../wailsjs/go/main/App';

/** Check whether the integrated engine binary and model are present. */
export async function checkSetupState() {
  return GetSetupState();
}

/** Convert a Blob to a base64 string (no data: URL prefix). */
async function blobToBase64(blob) {
  const buf = await blob.arrayBuffer();
  const bytes = new Uint8Array(buf);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
}

/**
 * Send a recorded audio blob to the Go backend, which posts it directly to
 * the configured Whisper ASR endpoint (no browser fetch, no CORS — Go's
 * net/http has no concept of cross-origin restrictions).
 */
export async function transcribe(blob, mimeType) {
  const s = get(settings);
  const audioBase64 = await blobToBase64(blob);
  return Transcribe(audioBase64, mimeType, s);
}
