<script>
  import { get } from 'svelte/store';
  import { onMount } from 'svelte';
  import { OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime';
  import Recorder       from '$lib/components/Recorder.svelte';
  import TranscriptPane from '$lib/components/TranscriptPane.svelte';
  import RefinementPane from '$lib/components/RefinementPane.svelte';
  import DiffView       from '$lib/components/DiffView.svelte';
  import Settings       from '$lib/components/Settings.svelte';
  import History        from '$lib/components/History.svelte';
  import Wizard         from '$lib/components/Wizard.svelte';

  import {
    settings, rawText, refinedText, isTranscribing, isRecording,
    showSettings, showWizard, showHistory, showDiff, audioBlob, endpointError,
    initSettings, recordTranscript,
    startTranscribeEstimate, clearTranscribeEstimate, finishTranscribeEstimate,
  } from '$lib/stores.js';
  import { transcribe, openAndTranscribeFile, transcribeFilePath, checkSetupState } from '$lib/api.js';
  import { t, initLocale } from '$lib/i18n';

  // Called when the Recorder component finishes a mic recording.
  async function handleRecorded(e) {
    const { blob, mimeType, durationSeconds } = e.detail;
    audioBlob.set(blob);
    endpointError.set(null);
    refinedText.set('');
    isTranscribing.set(true);
    startTranscribeEstimate(durationSeconds);
    const startedAt = performance.now();
    try {
      recordTranscript(await transcribe(blob, mimeType));
      finishTranscribeEstimate(durationSeconds, (performance.now() - startedAt) / 1000);
    } catch (err) {
      endpointError.set({ source: $t.app.errorSourceWhisper, message: err?.message ?? String(err) });
      clearTranscribeEstimate();
    } finally {
      isTranscribing.set(false);
    }
  }

  // Open native file picker → transcribe on the Go side (no base64 round-trip).
  async function handleOpenFile() {
    endpointError.set(null);
    refinedText.set('');
    // No cheaply-known audio duration for a picked file, so no time estimate —
    // just the plain "Transcribing…" indicator (see stores.js).
    clearTranscribeEstimate();
    isTranscribing.set(true);
    try {
      const text = await openAndTranscribeFile();
      if (text) recordTranscript(text);
    } catch (err) {
      endpointError.set({ source: $t.app.errorSourceWhisper, message: err?.message ?? String(err) });
    } finally {
      isTranscribing.set(false);
    }
  }

  // Drag & drop: accept audio files dropped anywhere on the window.
  // File content is read via Wails' native OS drag-and-drop (main.go's
  // DragAndDrop.EnableFileDrop), which delivers real file paths — the
  // browser's dataTransfer.files + base64-over-IPC path silently truncates
  // large files in `wails dev` (https://github.com/wailsapp/wails/issues/4211).
  let dragging = false;
  function onDragOver(e) { e.preventDefault(); dragging = true; }
  function onDragLeave()  { dragging = false; }
  // Prevent the browser's default "navigate to dropped file" behavior;
  // actual transcription happens in the OnFileDrop handler below.
  function onDrop(e) { e.preventDefault(); dragging = false; }

  async function handleFileDrop(x, y, paths) {
    dragging = false;
    const path = paths?.[0];
    if (!path) return;
    endpointError.set(null);
    refinedText.set('');
    clearTranscribeEstimate(); // see handleOpenFile — no known duration to estimate from
    isTranscribing.set(true);
    try {
      recordTranscript(await transcribeFilePath(path));
    } catch (err) {
      endpointError.set({ source: $t.app.errorSourceWhisper, message: err?.message ?? String(err) });
    } finally {
      isTranscribing.set(false);
    }
  }

  // Space-bar shortcut — toggle recording when not inside a text input
  function onKeyDown(e) {
    if (e.code !== 'Space') return;
    const tag = document.activeElement?.tagName ?? '';
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
    e.preventDefault();
    document.querySelector('.record-btn')?.click();
  }

  onMount(async () => {
    await initSettings();
    initLocale(); // best-effort OS-locale detection for uiLanguage === 'auto'
    const s = get(settings);
    // Show setup wizard if integrated mode is selected but engine isn't ready.
    if (s.mode === 'integrated') {
      const state = await checkSetupState();
      if (!state.binaryExists || !state.modelExists) {
        showWizard.set(true);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    OnFileDrop(handleFileDrop, false);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      OnFileDropOff();
    };
  });
</script>

<!-- Settings and History panels render their own overlays internally -->
<Settings />
<History />

{#if $showWizard}
  <!-- Full-screen setup wizard — replaces main UI during first-run -->
  <Wizard on:done={() => showWizard.set(false)} />
{:else}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="layout"
    class:drag-over={dragging}
    class:wide={$settings.refinementEnabled}
    on:dragover={onDragOver}
    on:dragleave={onDragLeave}
    on:drop={onDrop}
  >
    <!-- ── Top bar ──────────────────────────────────────────── -->
    <header class="topbar">
      <span class="app-name">🎙 Dictate</span>
      <div class="topbar-actions">
        <button
          class="tool-btn"
          on:click={() => showHistory.set(true)}
          title={$t.app.historyBtnTitle}
        >
          🕘 {$t.app.historyBtn}
        </button>
        {#if $settings.refinementEnabled}
          <button
            class="tool-btn"
            class:active={$showDiff}
            on:click={() => showDiff.update((v) => !v)}
            title={$t.app.diffToggleTitle}
          >
            ⟷ {$t.app.diffToggle}
          </button>
        {/if}
        <button
          class="tool-btn"
          on:click={() => showSettings.set(true)}
          title={$t.app.settingsBtnTitle}
        >
          ⚙ {$t.app.settingsBtn}
        </button>
      </div>
    </header>

    <!-- ── Recording section ───────────────────────────────── -->
    <section class="record-section">
      <Recorder on:recorded={handleRecorded} />
      <div class="file-row">
        <button
          class="file-btn"
          on:click={handleOpenFile}
          disabled={$isTranscribing || $isRecording}
          title={$t.app.openFileTitle}
        >
          {$t.app.openFile}
        </button>
        <span class="drop-hint">{$t.app.dropHint}</span>
      </div>
    </section>

    {#if dragging}
      <div class="drop-overlay">{$t.app.dropOverlay}</div>
    {/if}

    <!-- ── Endpoint error ───────────────────────────────────── -->
    {#if $endpointError}
      <div class="error-banner">
        <span class="error-source">{$endpointError.source}</span>
        {$endpointError.message}
      </div>
    {/if}

    <!-- ── Transcript / refinement panes (or diff) ───────────── -->
    <section class="panes">
      {#if $settings.refinementEnabled && $showDiff}
        <DiffView />
      {:else if $settings.refinementEnabled}
        <TranscriptPane />
        <div class="divider"></div>
        <RefinementPane />
      {:else}
        <TranscriptPane />
      {/if}
    </section>
  </div>
{/if}

<style>
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 0.75rem;
    gap: 0.75rem;
    box-sizing: border-box;
    max-width: 900px;
    margin: 0 auto;
    transition: max-width 0.15s;
  }

  .layout.wide {
    max-width: 1440px;
  }

  /* Top bar */
  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-shrink: 0;
  }

  .app-name {
    font-size: 1.05rem;
    font-weight: 700;
    color: #f1f5f9;
    letter-spacing: -0.01em;
  }

  .topbar-actions {
    display: flex;
    gap: 0.4rem;
  }

  .tool-btn {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #64748b;
    padding: 0.35rem 0.7rem;
    font-size: 0.78rem;
    cursor: pointer;
    transition: all 0.15s;
  }

  .tool-btn:hover  { background: #334155; color: #e2e8f0; }
  .tool-btn.active { background: #334155; color: #e2e8f0; border-color: #475569; }

  /* Record section */
  .record-section {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .file-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
  }

  .file-btn {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #94a3b8;
    font-size: 0.78rem;
    font-weight: 500;
    padding: 0.3rem 0.85rem;
    cursor: pointer;
    transition: all 0.15s;
    font-family: inherit;
  }
  .file-btn:hover:not(:disabled) { background: #334155; color: #e2e8f0; }
  .file-btn:disabled { opacity: 0.4; cursor: default; }

  .drop-hint {
    font-size: 0.72rem;
    color: #475569;
  }

  /* Drag-over overlay */
  .drop-overlay {
    position: fixed;
    inset: 0;
    background: rgba(124, 58, 237, 0.15);
    border: 2px dashed #7c3aed;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.1rem;
    font-weight: 600;
    color: #c4b5fd;
    pointer-events: none;
    z-index: 100;
  }

  .layout.drag-over { outline: none; }

  .error-banner {
    padding: 0.45rem 0.85rem;
    background: #2d0a0a;
    border: 1px solid #7f1d1d;
    border-radius: 6px;
    color: #fca5a5;
    font-size: 0.82rem;
    flex-shrink: 0;
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
  }

  .error-source {
    font-weight: 700;
    font-size: 0.7rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: #f87171;
    white-space: nowrap;
  }

  /* Panes */
  .panes {
    display: flex;
    flex: 1;
    min-height: 0;
    gap: 0;
  }

  .divider {
    width: 1px;
    background: #1e293b;
    margin: 0 0.5rem;
    flex-shrink: 0;
  }
</style>
