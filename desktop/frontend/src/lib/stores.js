import { writable, get } from 'svelte/store';
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
  refinementPrompt:  "Fix punctuation, capitalization, spelling, and obvious speech-to-text " +
                      "mistakes. Keep the original language, tone, and meaning exactly as they " +
                      "are. Only change words you're confident are wrong — leave everything " +
                      "else untouched, and make sure corrections fit naturally with the " +
                      "surrounding sentence. Return only the corrected text, with no " +
                      "explanations or preamble.",
  uiLanguage:        'auto',
  appendTranscripts: false,
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
export const showHistory    = writable(false);
export const showDiff       = writable(false); // toggle: side-by-side panes vs diff view

// Endpoint error — { source: string, message: string } | null
export const endpointError  = writable(null);

// --- Session transcript history ---
// In-memory only — cleared on restart, never sent anywhere. Just a scrollback
// of this session's transcripts so an earlier one isn't lost when a new
// recording replaces the transcript pane.
export const history = writable([]); // { id, timestamp, text }[], newest first
const MAX_HISTORY_ENTRIES = 200;

// crypto.randomUUID() needs a secure context, which isn't guaranteed across
// every platform/scheme Wails serves the frontend over — a plain counter
// avoids depending on that entirely; these ids only ever need to be unique
// within this session's in-memory list.
let nextHistoryId = 0;
function historyEntryId() {
  return ++nextHistoryId;
}

/**
 * Records a completed transcription in the session history (skipped if
 * blank — e.g. silence), then applies it to the transcript pane: replacing
 * its contents, or appending after a blank line, per settings.appendTranscripts.
 */
export function recordTranscript(text) {
  if (text && text.trim()) {
    history.update((h) =>
      [{ id: historyEntryId(), timestamp: Date.now(), text }, ...h].slice(0, MAX_HISTORY_ENTRIES)
    );
  }
  rawText.update((existing) =>
    get(settings).appendTranscripts && existing.trim() ? existing + '\n\n' + text : text
  );
}

// --- Transcription time estimate ---
// Guesses how long an in-flight transcription will take from this session's
// own past recordings — audio duration vs. how long transcribing it actually
// took — so the "Transcribing…" indicator can count down instead of just
// sitting there. Mic recordings only: file-picker/drag-drop transcriptions
// don't have a cheaply-known audio duration to calibrate against, so they
// fall back to the plain indicator (transcribeEstimateSeconds stays null).
const MAX_TIMING_SAMPLES = 20;
let timingSamples = []; // { audioSeconds, processingSeconds }[], mic recordings only
let estimateTimer;

export const transcribeEstimateSeconds = writable(null); // null = no estimate available

/** Call as soon as a mic recording finishes, right before transcription starts. */
export function startTranscribeEstimate(audioSeconds) {
  clearInterval(estimateTimer);
  if (!audioSeconds || timingSamples.length === 0) {
    transcribeEstimateSeconds.set(null);
    return;
  }
  const avgRate = timingSamples.reduce((sum, s) => sum + s.processingSeconds / s.audioSeconds, 0)
    / timingSamples.length;
  let remaining = avgRate * audioSeconds;
  transcribeEstimateSeconds.set(remaining);
  estimateTimer = setInterval(() => {
    remaining -= 1;
    transcribeEstimateSeconds.set(Math.max(0, remaining));
  }, 1000);
}

/** Clears any in-progress estimate/ticker without recording a sample — for
 * transcription paths that never had a known audio duration to begin with. */
export function clearTranscribeEstimate() {
  clearInterval(estimateTimer);
  transcribeEstimateSeconds.set(null);
}

/** Call once a mic-recorded transcription settles (success or failure).
 * On success, records the actual timing as a new calibration sample. */
export function finishTranscribeEstimate(audioSeconds, processingSeconds) {
  clearTranscribeEstimate();
  if (audioSeconds && processingSeconds) {
    timingSamples = [...timingSamples, { audioSeconds, processingSeconds }].slice(-MAX_TIMING_SAMPLES);
  }
}
