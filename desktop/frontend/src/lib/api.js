import { get } from 'svelte/store';
import { settings } from './stores.js';
import { Transcribe, Refine, OpenAndTranscribeFile, GetSetupState } from '../../wailsjs/go/main/App';

/** Check whether the integrated engine binary and model are present. */
export async function checkSetupState() {
  return GetSetupState();
}

/** Convert a Blob to a base64 string (no data: URL prefix). */
function blobToBase64(blob) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result.split(',')[1]);
    reader.onerror = reject;
    reader.readAsDataURL(blob);
  });
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

export async function refine(text) {
  const s = get(settings);
  return Refine(text, s);
}

/** Open a native file picker, transcribe the selected file on the Go side. */
export async function openAndTranscribeFile() {
  const s = get(settings);
  return OpenAndTranscribeFile(s);
}
