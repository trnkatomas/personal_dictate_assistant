import { writable } from 'svelte/store';

/** Writable store that syncs its value to localStorage. */
function persisted(key, initial) {
  let stored = initial;
  if (typeof window !== 'undefined') {
    try {
      const item = localStorage.getItem(key);
      if (item !== null) stored = JSON.parse(item);
    } catch { /* ignore */ }
  }

  const store = writable(stored);

  if (typeof window !== 'undefined') {
    store.subscribe((v) => {
      try { localStorage.setItem(key, JSON.stringify(v)); } catch { /* ignore */ }
    });
  }

  return store;
}

// --- Persisted across page reloads ---

export const settings = persisted('da:settings', {
  whisperUrl:      'http://localhost:9000',   // upstream Whisper ASR base URL
  ollamaUrl:       'http://localhost:11434',  // upstream Ollama base URL
  ollamaModel:     'gemma3:1b',
  whisperLanguage: '',           // empty = auto-detect
  whisperTask:     'transcribe'  // 'transcribe' | 'translate'
});

export const prompt = persisted(
  'da:prompt',
  'Add punctuation, fix grammar errors, and format into proper sentences. Return only the corrected text — no explanations, no preamble.'
);

// --- Ephemeral UI state ---

export const isRecording    = writable(false);
export const isTranscribing = writable(false);
export const isTransforming = writable(false);
export const audioBlob      = writable(null);
export const rawText        = writable('');
export const transformedText = writable('');
export const showSettings   = writable(false);
export const showDiff       = writable(false);

// Unified endpoint error — { source: 'Whisper' | 'Ollama', message: string } | null
export const endpointError  = writable(null);
