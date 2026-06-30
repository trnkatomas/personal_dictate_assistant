import { writable } from 'svelte/store';
import { LoadSettings, SaveSettings } from '../../wailsjs/go/main/App';

// Settings are persisted by the Go backend to a JSON config file
// (see desktop/settings.go) rather than localStorage — see initSettings().
export const settings = writable({
  mode:            'integrated',
  whisperUrl:      'http://localhost:9000',
  whisperLanguage: '',
  whisperTask:     'transcribe',
  modelName:       '',
});

/** Load persisted settings from the Go backend. Call once on app startup. */
export async function initSettings() {
  const loaded = await LoadSettings();
  settings.set(loaded);
}

let saveTimer;
let firstEmit = true;
settings.subscribe((s) => {
  if (firstEmit) { firstEmit = false; return; } // skip the initial placeholder value
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => SaveSettings(s), 300); // debounce rapid edits
});

// --- Ephemeral UI state ---

export const isRecording    = writable(false);
export const isTranscribing = writable(false);
export const audioBlob      = writable(null);
export const rawText        = writable('');
export const showSettings   = writable(false);
export const showWizard     = writable(false);

// Endpoint error — { source: 'Whisper', message: string } | null
export const endpointError  = writable(null);
