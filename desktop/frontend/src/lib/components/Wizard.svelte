<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
  import { GetSystemInfo, StartDownload, CancelDownload } from '../../../wailsjs/go/main/App';
  import { settings, initSettings } from '$lib/stores.js';

  const dispatch = createEventDispatcher();

  // steps: 'welcome' | 'recommendation' | 'downloading' | 'ready'
  let step = 'welcome';

  // Step 2 — hardware info + model selection
  let systemInfo = null;
  let recommendedModel = '';  // model ID to highlight with a badge
  let selectedModel = 'large-v3-turbo';

  // Step 3 — download progress
  let downloadLabel = 'Starting…';
  let downloadPercent = 0;
  let downloadError = '';

  const MODELS = [
    { id: 'small',          label: 'Small',       size: '244 MB',  desc: 'Fast; English-optimized' },
    { id: 'medium',         label: 'Medium',      size: '769 MB',  desc: 'Good multilingual accuracy' },
    { id: 'large-v3-turbo', label: 'Large Turbo', size: '809 MB',  desc: 'Near-identical accuracy to Large v3, much faster on Apple Silicon' },
    { id: 'large-v3',       label: 'Large v3',    size: '1.5 GB',  desc: 'Maximum accuracy; larger download with marginal gain over Turbo' },
  ];

  function recommendModel({ gpu, ramGB, locale }) {
    const nonEnglish = !locale.startsWith('en');
    if (gpu === 'apple_silicon') {
      // Large Turbo matches Large v3 accuracy on Apple Silicon with Metal,
      // at half the size — always prefer it over the full model.
      return ramGB < 8 ? 'medium' : 'large-v3-turbo';
    }
    if (gpu === 'cuda') return 'large-v3';
    return nonEnglish ? 'medium' : 'small';
  }

  function gpuLabel(gpu) {
    if (gpu === 'apple_silicon') return 'Apple Silicon';
    if (gpu === 'cuda')          return 'NVIDIA GPU (CUDA)';
    return 'CPU only';
  }

  // Pre-fetch system info while the user reads the welcome screen
  onMount(async () => {
    try {
      systemInfo = await GetSystemInfo();
    } catch (_) {
      systemInfo = { gpu: 'none', ramGB: 0, locale: 'en', platform: '' };
    }
  });

  async function goToRecommendation() {
    if (!systemInfo) {
      try { systemInfo = await GetSystemInfo(); } catch (_) {
        systemInfo = { gpu: 'none', ramGB: 0, locale: 'en', platform: '' };
      }
    }
    recommendedModel = recommendModel(systemInfo);
    selectedModel    = recommendedModel;
    step = 'recommendation';
  }

  async function startDownload() {
    downloadLabel = 'Starting…';
    downloadPercent = 0;
    downloadError = '';
    step = 'downloading';

    EventsOn('download:progress', (progress) => {
      if (progress.error) {
        downloadError = progress.error;
        return;
      }
      if (progress.done) {
        onDownloadDone();
        return;
      }
      downloadLabel   = progress.label;
      downloadPercent = progress.percent;
    });

    await StartDownload(selectedModel);
  }

  async function onDownloadDone() {
    EventsOff('download:progress');
    // Reload settings — Go saved modelName + mode during the download.
    await initSettings();
    step = 'ready';
  }

  function finish() {
    dispatch('done');
  }

  function skip() {
    // Switch to HTTP mode so the wizard doesn't re-appear on next launch.
    // The user can configure the URL in Settings, or switch back to Integrated
    // at any time via Settings → "Download model".
    settings.update(s => ({ ...s, mode: 'http' }));
    dispatch('done');
  }

  onDestroy(() => {
    EventsOff('download:progress');
  });
</script>

<div class="wizard-overlay">
  <div class="wizard-panel">

    <!-- ── Step 1: Welcome ──────────────────────────────────── -->
    {#if step === 'welcome'}
      <div class="step">
        <div class="step-icon">🎙</div>
        <h2>Welcome to Dictate</h2>
        <p class="step-desc">
          Dictate transcribes your speech locally — no cloud, no data leaving your machine.
          The first time you run it, a small setup is needed: the transcription engine and model
          will be downloaded (~800 MB for the recommended model).
        </p>
        <div class="step-actions">
          <button class="primary-btn" on:click={goToRecommendation}>Get started</button>
        </div>
        <button class="skip-link" on:click={skip}>
          I know what I'm doing — skip setup
        </button>
      </div>

    <!-- ── Step 2: Recommendation ───────────────────────────── -->
    {:else if step === 'recommendation'}
      <div class="step">
        <h2>Choose a model</h2>

        {#if systemInfo}
          <div class="hw-summary">
            <span class="hw-chip">{gpuLabel(systemInfo.gpu)}</span>
            {#if systemInfo.ramGB > 0}
              <span class="hw-chip">{systemInfo.ramGB} GB RAM</span>
            {/if}
            {#if systemInfo.locale && systemInfo.locale !== 'en'}
              <span class="hw-chip">Language: {systemInfo.locale}</span>
            {/if}
          </div>
        {/if}

        <div class="model-list">
          {#each MODELS as m}
            <label class="model-option" class:selected={selectedModel === m.id}>
              <input type="radio" name="model" value={m.id} bind:group={selectedModel} />
              <div class="model-info">
                <span class="model-label">{m.label}</span>
                {#if m.id === recommendedModel}
                  <span class="rec-badge">Recommended</span>
                {/if}
                <span class="model-size">{m.size}</span>
              </div>
              <span class="model-desc">{m.desc}</span>
            </label>
          {/each}
        </div>

        <p class="download-note">
          Download includes the whisper-cli engine and the selected model.
        </p>

        <div class="step-actions">
          <button class="secondary-btn" on:click={() => step = 'welcome'}>Back</button>
          <button class="primary-btn" on:click={startDownload}>Download &amp; install</button>
        </div>
      </div>

    <!-- ── Step 3: Downloading ──────────────────────────────── -->
    {:else if step === 'downloading'}
      <div class="step">
        <h2>Downloading…</h2>

        {#if downloadError}
          <div class="error-box">{downloadError}</div>
          <div class="step-actions">
            <button class="secondary-btn" on:click={async () => {
              await CancelDownload();
              downloadError = '';
              step = 'recommendation';
            }}>Try again</button>
          </div>
        {:else}
          <p class="dl-label">{downloadLabel}</p>
          <div class="progress-track">
            <div class="progress-bar" style="width: {Math.round(downloadPercent)}%"></div>
          </div>
          <p class="dl-pct">{Math.round(downloadPercent)}%</p>
          <p class="dl-note">Please keep the app open during download.</p>
          <div class="step-actions">
            <button class="secondary-btn danger" on:click={async () => {
              await CancelDownload();
              step = 'recommendation';
            }}>Cancel</button>
          </div>
        {/if}
      </div>

    <!-- ── Step 4: Ready ────────────────────────────────────── -->
    {:else if step === 'ready'}
      <div class="step">
        <div class="step-icon">✅</div>
        <h2>All set!</h2>
        <p class="step-desc">
          Dictate is ready to transcribe locally using the <strong>{selectedModel}</strong> model.
          No internet connection is required for transcription.
          You can change the model or switch to HTTP mode at any time in Settings.
        </p>
        <div class="step-actions">
          <button class="primary-btn" on:click={finish}>Start using Dictate</button>
        </div>
      </div>
    {/if}

  </div>
</div>

<style>
  .wizard-overlay {
    position: fixed;
    inset: 0;
    background: #0a0f1a;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 300;
  }

  .wizard-panel {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 16px;
    width: 520px;
    max-width: calc(100vw - 2rem);
    padding: 2rem 2rem 1.75rem;
  }

  .step {
    display: flex;
    flex-direction: column;
    gap: 1.1rem;
  }

  .step-icon {
    font-size: 2.5rem;
    text-align: center;
  }

  h2 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 700;
    color: #f1f5f9;
    text-align: center;
  }

  .step-desc {
    margin: 0;
    color: #94a3b8;
    font-size: 0.875rem;
    line-height: 1.6;
    text-align: center;
  }

  /* ── Hardware chips ── */
  .hw-summary {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    justify-content: center;
  }

  .hw-chip {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 999px;
    padding: 0.2rem 0.7rem;
    font-size: 0.75rem;
    color: #64748b;
  }

  /* ── Model list ── */
  .model-list {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }

  .model-option {
    display: grid;
    grid-template-columns: auto 1fr;
    grid-template-rows: auto auto;
    column-gap: 0.6rem;
    row-gap: 0.1rem;
    padding: 0.6rem 0.75rem;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    cursor: pointer;
    transition: border-color 0.15s;
    align-items: start;
  }

  .model-option:hover { border-color: #475569; }
  .model-option.selected { border-color: #7c3aed; background: #1e1b4b; }

  .model-option input[type="radio"] {
    margin-top: 0.2rem;
    grid-row: 1 / 3;
    accent-color: #7c3aed;
  }

  .model-info {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
  }

  .model-label {
    font-size: 0.875rem;
    font-weight: 600;
    color: #e2e8f0;
  }

  .model-size {
    font-size: 0.75rem;
    color: #64748b;
  }

  .rec-badge {
    background: #312e81;
    border: 1px solid #4f46e5;
    border-radius: 999px;
    color: #a5b4fc;
    font-size: 0.65rem;
    font-weight: 700;
    letter-spacing: 0.04em;
    padding: 0.1rem 0.45rem;
    text-transform: uppercase;
  }

  .model-desc {
    font-size: 0.75rem;
    color: #64748b;
    grid-column: 2;
  }

  .download-note {
    margin: 0;
    font-size: 0.75rem;
    color: #475569;
    text-align: center;
  }

  /* ── Progress ── */
  .dl-label {
    margin: 0;
    font-size: 0.875rem;
    color: #94a3b8;
    text-align: center;
    font-weight: 600;
  }

  .progress-track {
    height: 8px;
    background: #0f172a;
    border-radius: 999px;
    overflow: hidden;
    border: 1px solid #1e293b;
  }

  .progress-bar {
    height: 100%;
    background: linear-gradient(90deg, #7c3aed, #4f46e5);
    border-radius: 999px;
    transition: width 0.3s ease;
  }

  .dl-pct {
    margin: 0;
    font-size: 1.1rem;
    font-weight: 700;
    color: #c4b5fd;
    text-align: center;
  }

  .dl-note {
    margin: 0;
    font-size: 0.75rem;
    color: #475569;
    text-align: center;
  }

  .error-box {
    background: #2d0a0a;
    border: 1px solid #7f1d1d;
    border-radius: 8px;
    color: #fca5a5;
    font-size: 0.82rem;
    padding: 0.6rem 0.85rem;
    text-align: left;
  }

  /* ── Buttons ── */
  .step-actions {
    display: flex;
    justify-content: center;
    gap: 0.6rem;
    margin-top: 0.4rem;
  }

  .primary-btn {
    background: #7c3aed;
    border: none;
    border-radius: 8px;
    color: #fff;
    font-size: 0.9rem;
    font-weight: 600;
    padding: 0.6rem 1.6rem;
    cursor: pointer;
    transition: background 0.15s;
    font-family: inherit;
  }
  .primary-btn:hover { background: #6d28d9; }

  .secondary-btn {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #94a3b8;
    font-size: 0.9rem;
    padding: 0.6rem 1.2rem;
    cursor: pointer;
    transition: all 0.15s;
    font-family: inherit;
  }
  .secondary-btn:hover { background: #334155; color: #e2e8f0; }
  .secondary-btn.danger { color: #f87171; border-color: #7f1d1d; }
  .secondary-btn.danger:hover { background: #2d0a0a; }

  .skip-link {
    background: none;
    border: none;
    color: #475569;
    font-size: 0.75rem;
    cursor: pointer;
    padding: 0;
    text-align: center;
    text-decoration: underline;
    text-underline-offset: 2px;
    transition: color 0.15s;
    font-family: inherit;
  }
  .skip-link:hover { color: #94a3b8; }
</style>
