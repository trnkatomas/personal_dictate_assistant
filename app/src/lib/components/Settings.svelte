<script>
  import { settings, showSettings } from '$lib/stores.js';

  const DEFAULTS = {
    whisperUrl:      'https://n9xft9yspaqujp-9000.proxy.runpod.net',
    whisperLanguage: '',
    whisperTask:     'transcribe',
  };

  const PRESETS = [
    {
      id:         'docker',
      label:      'Docker Compose',
      whisperUrl: 'http://whisper:9000',
    },
    {
      id:         'local',
      label:      'Local',
      whisperUrl: 'http://localhost:9000',
    },
    {
      id:         'external',
      label:      'External',
      whisperUrl: '',
    },
  ];

  // What's shown in the URL input — starts from the saved store value.
  // Selecting a preset updates this without touching the store.
  // Apply commits it to the store.
  let previewWhisperUrl = $settings.whisperUrl;

  let selectedPresetId = 'current';

  function onPresetChange() {
    if (selectedPresetId === 'current') {
      // Snap preview back to whatever is actually saved
      previewWhisperUrl = $settings.whisperUrl;
    } else {
      const preset = PRESETS.find(p => p.id === selectedPresetId);
      if (preset) {
        previewWhisperUrl = preset.whisperUrl;
      }
    }
  }

  function applyPreset() {
    settings.update(s => ({ ...s, whisperUrl: previewWhisperUrl }));
    selectedPresetId = 'current';
    // preview already matches the store — no further update needed
  }

  // Whether the preview differs from what's actually stored
  $: isPreviewing = previewWhisperUrl !== $settings.whisperUrl;

  function reset() {
    settings.set({ ...DEFAULTS });
    previewWhisperUrl = DEFAULTS.whisperUrl;
    selectedPresetId  = 'current';
  }

  function close() {
    showSettings.set(false);
  }

  function onOverlayClick(e) {
    if (e.target === e.currentTarget) close();
  }
</script>

{#if $showSettings}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={onOverlayClick}>
    <div class="panel" role="dialog" aria-label="Settings">
      <div class="panel-header">
        <h3>Settings</h3>
        <button class="close-btn" on:click={close}>✕</button>
      </div>

      <div class="fields">

        <!-- ── Endpoint frame ── -->
        <div class="endpoint-frame">
          <span class="frame-title">Service endpoints</span>

          <div class="preset-row">
            <select bind:value={selectedPresetId} on:change={onPresetChange}>
              <option value="current">Current</option>
              {#each PRESETS as p}
                <option value={p.id}>{p.label}</option>
              {/each}
            </select>
            <button class="apply-btn" on:click={applyPreset} disabled={!isPreviewing}>
              Apply
            </button>
          </div>

          <label>
            <span>Whisper URL</span>
            <input
              type="url"
              bind:value={previewWhisperUrl}
              placeholder="http://localhost:9000"
              class:previewing={isPreviewing}
            />
          </label>
        </div>

        <!-- ── Other settings ── -->
        <label>
          <span>Whisper language <em>(leave blank to auto-detect)</em></span>
          <input type="text" bind:value={$settings.whisperLanguage} placeholder="e.g. en, de, cs" />
        </label>

        <label>
          <span>Whisper task</span>
          <select bind:value={$settings.whisperTask}>
            <option value="transcribe">Transcribe (keep original language)</option>
            <option value="translate">Translate to English</option>
          </select>
        </label>

      </div>

      <div class="panel-footer">
        <button class="reset-btn" on:click={reset}>Reset to defaults</button>
        <button class="save-btn"  on:click={close}>Done</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 200;
    backdrop-filter: blur(2px);
  }

  .panel {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    width: 420px;
    max-width: calc(100vw - 2rem);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem 0.75rem;
    border-bottom: 1px solid #334155;
  }

  h3 {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  .close-btn {
    background: none;
    border: none;
    color: #475569;
    cursor: pointer;
    font-size: 1rem;
    padding: 0.2rem 0.4rem;
    border-radius: 4px;
    transition: color 0.15s;
  }

  .close-btn:hover { color: #e2e8f0; }

  .fields {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    padding: 1rem 1.25rem;
  }

  /* ── Endpoint frame ── */
  .endpoint-frame {
    position: relative;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: 1rem 0.85rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .frame-title {
    position: absolute;
    top: -0.55rem;
    left: 0.65rem;
    background: #1e293b;
    padding: 0 0.3rem;
    font-size: 0.65rem;
    font-weight: 700;
    letter-spacing: 0.07em;
    text-transform: uppercase;
    color: #475569;
  }

  .preset-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }

  .preset-row select {
    flex: 1;
  }

  .apply-btn {
    background: #1e3a5f;
    border: 1px solid #2d5a8e;
    border-radius: 6px;
    color: #93c5fd;
    font-size: 0.8rem;
    font-weight: 600;
    padding: 0.4rem 0.85rem;
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
    font-family: inherit;
  }

  .apply-btn:hover:not(:disabled) {
    background: #1d4ed8;
    border-color: #3b82f6;
    color: #fff;
  }

  .apply-btn:disabled {
    opacity: 0.35;
    cursor: default;
  }

  /* ── Shared form elements ── */
  label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  label span {
    font-size: 0.75rem;
    color: #64748b;
  }

  label em {
    font-style: normal;
    color: #475569;
  }

  input, select {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #e2e8f0;
    padding: 0.4rem 0.65rem;
    font-size: 0.85rem;
    outline: none;
    transition: border-color 0.15s;
    font-family: inherit;
  }

  input:focus, select:focus { border-color: #7c3aed; }

  /* Subtle amber tint while showing an unapplied preview */
  input.previewing { border-color: #92400e; }
  input.previewing:focus { border-color: #f59e0b; }

  select option { background: #1e293b; }

  /* ── Footer ── */
  .panel-footer {
    display: flex;
    justify-content: space-between;
    padding: 0.75rem 1.25rem 1rem;
    border-top: 1px solid #334155;
  }

  .reset-btn {
    background: none;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #64748b;
    font-size: 0.8rem;
    padding: 0.4rem 0.8rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .reset-btn:hover { background: #334155; color: #e2e8f0; }

  .save-btn {
    background: #7c3aed;
    border: none;
    border-radius: 6px;
    color: #fff;
    font-size: 0.8rem;
    font-weight: 600;
    padding: 0.4rem 1.2rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .save-btn:hover { background: #6d28d9; }
</style>
