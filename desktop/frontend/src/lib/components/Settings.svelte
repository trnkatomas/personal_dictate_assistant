<script>
  import { settings, showSettings, showWizard } from '$lib/stores.js';
  import { t } from '$lib/i18n';

  const DEFAULTS = {
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
  };

  // Labels come from the active dictionary so switching uiLanguage updates
  // them immediately, without touching the underlying preset values.
  $: PRESETS = [
    { id: 'local',    label: $t.settings.presetLocal,    whisperUrl: 'http://localhost:9000' },
    { id: 'external', label: $t.settings.presetExternal, whisperUrl: '' },
  ];

  // Preview state for the URL field — selecting a preset updates this
  // without touching the store; Apply commits it.
  let previewWhisperUrl = $settings.whisperUrl;
  let selectedPresetId  = 'current';

  function onPresetChange() {
    if (selectedPresetId === 'current') {
      previewWhisperUrl = $settings.whisperUrl;
    } else {
      const preset = PRESETS.find(p => p.id === selectedPresetId);
      if (preset) previewWhisperUrl = preset.whisperUrl;
    }
  }

  function applyPreset() {
    settings.update(s => ({ ...s, whisperUrl: previewWhisperUrl }));
    selectedPresetId = 'current';
  }

  $: isPreviewing = previewWhisperUrl !== $settings.whisperUrl;

  // When settings change externally (e.g. after LoadSettings resolves on startup),
  // keep the preview in sync if the user hasn't made a selection.
  $: if (selectedPresetId === 'current') previewWhisperUrl = $settings.whisperUrl;

  function setMode(mode) {
    settings.update(s => ({ ...s, mode }));
  }

  function changeModel() {
    showSettings.set(false);
    showWizard.set(true);
  }

  function reset() {
    settings.set({ ...DEFAULTS });
    previewWhisperUrl = DEFAULTS.whisperUrl;
    selectedPresetId  = 'current';
  }

  function close() { showSettings.set(false); }

  function onOverlayClick(e) {
    if (e.target === e.currentTarget) close();
  }
</script>

{#if $showSettings}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={onOverlayClick}>
    <div class="panel" role="dialog" aria-label={$t.settings.dialogLabel}>
      <div class="panel-header">
        <h3>{$t.settings.title}</h3>
        <button class="close-btn" on:click={close}>✕</button>
      </div>

      <div class="fields">

        <!-- ── Transcription mode frame ── -->
        <div class="endpoint-frame">
          <span class="frame-title">{$t.settings.modeFrameTitle}</span>

          <!-- Mode toggle -->
          <div class="mode-toggle">
            <button
              class="mode-btn"
              class:active={$settings.mode === 'integrated'}
              on:click={() => setMode('integrated')}
            >{$t.settings.modeIntegrated}</button>
            <button
              class="mode-btn"
              class:active={$settings.mode === 'http'}
              on:click={() => setMode('http')}
            >{$t.settings.modeHttp}</button>
          </div>

          {#if $settings.mode === 'integrated'}
            <!-- Integrated mode: show model status + change button -->
            <div class="model-row">
              {#if $settings.modelName}
                <span class="model-status">
                  {$t.settings.modelPrefix} <strong>{$settings.modelName}</strong>
                </span>
              {:else}
                <span class="model-status missing">{$t.settings.noModel}</span>
              {/if}
              <button class="change-model-btn" on:click={changeModel}>
                {$settings.modelName ? $t.settings.changeModel : $t.settings.downloadModel}
              </button>
            </div>
          {:else}
            <!-- HTTP mode: preset dropdown + URL field -->
            <div class="preset-row">
              <select bind:value={selectedPresetId} on:change={onPresetChange}>
                <option value="current">{$t.settings.presetCurrent}</option>
                {#each PRESETS as p}
                  <option value={p.id}>{p.label}</option>
                {/each}
              </select>
              <button class="apply-btn" on:click={applyPreset} disabled={!isPreviewing}>
                {$t.settings.apply}
              </button>
            </div>

            <label>
              <span>{$t.settings.whisperUrlLabel}</span>
              <input
                type="url"
                bind:value={previewWhisperUrl}
                placeholder="http://localhost:9000"
                class:previewing={isPreviewing}
              />
            </label>
          {/if}
        </div>

        <!-- ── Text refinement frame ── -->
        <div class="endpoint-frame">
          <span class="frame-title">{$t.settings.refinementFrameTitle}</span>

          <label class="toggle-label">
            <input type="checkbox" bind:checked={$settings.refinementEnabled} />
            <span>{$t.settings.refinementToggleLabel}</span>
          </label>

          {#if $settings.refinementEnabled}
            <label>
              <span>{$t.settings.refinementUrlLabel} <em>{$t.settings.refinementUrlHint}</em></span>
              <input type="url" bind:value={$settings.refinementUrl} placeholder="http://localhost:11434/v1" />
            </label>
            <label>
              <span>{$t.settings.refinementModelLabel}</span>
              <input type="text" bind:value={$settings.refinementModel} placeholder="qwen3:1.7b" />
            </label>
          {/if}
        </div>

        <!-- ── Language & task (always visible) ── -->
        <label>
          <span>{$t.settings.whisperLanguageLabel} <em>{$t.settings.whisperLanguageHint}</em></span>
          <input type="text" bind:value={$settings.whisperLanguage} placeholder="e.g. en, de, cs" />
        </label>

        <label>
          <span>{$t.settings.whisperTaskLabel}</span>
          <select bind:value={$settings.whisperTask}>
            <option value="transcribe">{$t.settings.taskTranscribe}</option>
            <option value="translate">{$t.settings.taskTranslate}</option>
          </select>
        </label>

        <label class="toggle-label">
          <input type="checkbox" bind:checked={$settings.appendTranscripts} />
          <span>{$t.settings.appendToggleLabel}</span>
        </label>

        <!-- ── App language ── -->
        <label>
          <span>{$t.settings.languageFrameTitle}</span>
          <select bind:value={$settings.uiLanguage}>
            <option value="auto">{$t.settings.languageAuto}</option>
            <option value="en">{$t.settings.languageEnglish}</option>
            <option value="cs">{$t.settings.languageCzech}</option>
          </select>
        </label>

      </div>

      <div class="panel-footer">
        <button class="reset-btn" on:click={reset}>{$t.settings.reset}</button>
        <button class="save-btn"  on:click={close}>{$t.settings.done}</button>
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

  /* Mode toggle */
  .mode-toggle {
    display: flex;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 6px;
    overflow: hidden;
  }

  .mode-btn {
    flex: 1;
    background: none;
    border: none;
    color: #64748b;
    padding: 0.4rem 0;
    font-size: 0.8rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;
    font-family: inherit;
  }
  .mode-btn:hover { color: #e2e8f0; }
  .mode-btn.active {
    background: #7c3aed;
    color: #fff;
    font-weight: 600;
  }

  /* Integrated model row */
  .model-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }

  .model-status {
    font-size: 0.82rem;
    color: #94a3b8;
  }
  .model-status strong {
    color: #e2e8f0;
  }
  .model-status.missing {
    color: #f87171;
    font-size: 0.8rem;
  }

  .change-model-btn {
    background: #1e3a5f;
    border: 1px solid #2d5a8e;
    border-radius: 6px;
    color: #93c5fd;
    font-size: 0.78rem;
    font-weight: 600;
    padding: 0.35rem 0.75rem;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s;
    font-family: inherit;
  }
  .change-model-btn:hover {
    background: #1d4ed8;
    border-color: #3b82f6;
    color: #fff;
  }

  /* HTTP preset row */
  .preset-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
  }
  .preset-row select { flex: 1; }

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
  .apply-btn:disabled { opacity: 0.35; cursor: default; }

  /* ── Shared form elements ── */
  label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .toggle-label {
    flex-direction: row;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
  }

  .toggle-label input[type="checkbox"] {
    accent-color: #7c3aed;
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    border: none;
    background: none;
    padding: 0;
  }

  .toggle-label span {
    font-size: 0.82rem;
    color: #94a3b8;
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
  input.previewing            { border-color: #92400e; }
  input.previewing:focus      { border-color: #f59e0b; }

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
