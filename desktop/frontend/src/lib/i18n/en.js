// English dictionary — also the fallback for any locale/key that isn't
// otherwise available. Keep this in sync with cs.js: every key here should
// have a Czech counterpart, and vice versa.
export default {
  app: {
    diffToggle: 'Diff',
    diffToggleTitle: 'Toggle diff view',
    settingsBtn: 'Settings',
    settingsBtnTitle: 'Open settings',
    openFile: 'Open file…',
    openFileTitle: 'Transcribe an audio file from disk',
    dropHint: 'or drop an audio file anywhere',
    dropOverlay: 'Drop audio file to transcribe',
    errorSourceWhisper: 'Whisper',
    errorSourceRefinement: 'Refinement',
  },

  common: {
    copy: 'Copy',
    copied: '✓ Copied',
  },

  settings: {
    title: 'Settings',
    dialogLabel: 'Settings',
    modeFrameTitle: 'Transcription mode',
    modeIntegrated: 'Integrated',
    modeHttp: 'HTTP endpoint',
    modelPrefix: 'Model:',
    noModel: 'No model downloaded',
    changeModel: 'Change model',
    downloadModel: 'Download model',
    presetCurrent: 'Current',
    presetLocal: 'Local',
    presetExternal: 'External',
    apply: 'Apply',
    whisperUrlLabel: 'Whisper URL',
    refinementFrameTitle: 'Text refinement',
    refinementToggleLabel: 'Automatically polish transcription with an LLM',
    refinementUrlLabel: 'Model URL',
    refinementUrlHint: '(OpenAI-compatible base, e.g. Ollama)',
    refinementModelLabel: 'Model name',
    whisperLanguageLabel: 'Whisper language',
    whisperLanguageHint: '(leave blank to auto-detect)',
    whisperTaskLabel: 'Whisper task',
    taskTranscribe: 'Transcribe (keep original language)',
    taskTranslate: 'Translate to English',
    languageFrameTitle: 'App language',
    languageAuto: 'Auto (system)',
    languageEnglish: 'English',
    languageCzech: 'Čeština',
    reset: 'Reset to defaults',
    done: 'Done',
  },

  wizard: {
    welcomeTitle: 'Welcome to Dictate',
    welcomeDesc: 'Dictate transcribes your speech locally — no cloud, no data leaving ' +
      'your machine. The first time you run it, a small setup is needed: the transcription ' +
      'engine and model will be downloaded (~800 MB for the recommended model).',
    getStarted: 'Get started',
    skipSetup: "I know what I'm doing — skip setup",
    chooseModel: 'Choose a model',
    gpuAppleSilicon: 'Apple Silicon',
    gpuCuda: 'NVIDIA GPU (CUDA)',
    gpuCpu: 'CPU only',
    ramChip: (ramGB) => `${ramGB} GB RAM`,
    languageChip: (locale) => `Language: ${locale}`,
    models: {
      small:              { label: 'Small',       desc: 'Fast; English-optimized' },
      medium:             { label: 'Medium',      desc: 'Good multilingual accuracy' },
      'large-v3-turbo':   { label: 'Large Turbo', desc: 'Near-identical accuracy to Large v3, much faster on Apple Silicon' },
      'large-v3':         { label: 'Large v3',    desc: 'Maximum accuracy; larger download with marginal gain over Turbo' },
    },
    recommended: 'Recommended',
    downloadNote: 'Download includes the whisper-cli engine and the selected model.',
    back: 'Back',
    downloadInstall: 'Download & install',
    starting: 'Starting…',
    downloading: 'Downloading…',
    tryAgain: 'Try again',
    pleaseKeepOpen: 'Please keep the app open during download.',
    cancel: 'Cancel',
    allSet: 'All set!',
    // Split around the model name so the template can keep it <strong>.
    readyDescBefore: 'Dictate is ready to transcribe locally using the',
    readyDescAfter: ' model. No internet connection is required for transcription. ' +
      'You can change the model or switch to HTTP mode at any time in Settings.',
    startUsing: 'Start using Dictate',
  },

  recorder: {
    micDenied: (message) => `Microphone access denied: ${message}`,
    stopTitle: 'Stop recording (Space)',
    startTitle: 'Start recording (Space)',
    stop: 'Stop',
    record: 'Record',
  },

  transcript: {
    title: 'Transcription',
    transcribing: 'Transcribing…',
    placeholder: 'Raw transcription will appear here after recording…',
    ariaLabel: 'Raw transcription',
  },

  refine: {
    title: 'Refined',
    refining: 'Refining…',
    refiningProgress: (current, total) => `Refining ${current}/${total}…`,
    promptPlaceholder: 'Refinement instruction prompt…',
    promptAriaLabel: 'Refinement prompt',
    refineBtn: 'Refine',
    placeholder: 'Refined text will appear here…',
    ariaLabel: 'Refined text',
  },

  diff: {
    transcriptionSide: 'Transcription',
    refinedSide: 'Refined',
    removedWords: (n) => `−${n} words`,
    addedWords: (n) => `+${n} words`,
    emptyTitle: 'Nothing to diff yet.',
    emptyHint: 'Record → transcribe → refine, then come back here.',
  },
};
