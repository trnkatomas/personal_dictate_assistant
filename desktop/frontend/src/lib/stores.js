import { writable } from 'svelte/store';
import { LoadSettings, SaveSettings } from '../../wailsjs/go/main/App';

// Settings are persisted by the Go backend to a JSON config file
// (see desktop/settings.go) rather than localStorage — see initSettings().
export const settings = writable({
  mode:              'integrated',
  whisperUrl:        'http://localhost:9000',
  whisperLanguage:   '',
  whisperTask:       'transcribe',
  modelName:         '',
  refinementEnabled: false,
  refinementUrl:     'http://localhost:11434/v1',
  refinementModel:   'qwen3:1.7b',
  refinementPrompt:  'Fix punctuation, capitalisation, and obvious speech-to-text artifacts. ' +
                      'Keep the original language, tone, and meaning unchanged. Return only ' +
                      'the corrected text — no explanations, no preamble.',
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
export const isRefining     = writable(false);
export const audioBlob      = writable(null);
export const rawText        = writable('');
export const refinedText    = writable('');   // populated after a manual refine
export const showSettings   = writable(false);
export const showWizard     = writable(false);
export const showDiff       = writable(false); // toggle: side-by-side panes vs diff view

// Endpoint error — { source: string, message: string } | null
export const endpointError  = writable(null);
