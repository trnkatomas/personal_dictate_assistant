<script>
  import { get } from 'svelte/store';
  import { onMount } from 'svelte';
  import Recorder       from '$lib/components/Recorder.svelte';
  import TranscriptPane from '$lib/components/TranscriptPane.svelte';
  import Settings       from '$lib/components/Settings.svelte';
  import Wizard         from '$lib/components/Wizard.svelte';

  import {
    settings, rawText, refinedText, isTranscribing, isRefining, isRecording,
    showSettings, showWizard, audioBlob, endpointError,
    initSettings
  } from '$lib/stores.js';
  import { transcribe, refine, openAndTranscribeFile, checkSetupState } from '$lib/api.js';

  // Shared post-transcription flow: optional refinement.
  async function afterTranscribe(text) {
    if (!$settings.refinementEnabled || !text) return;
    isRefining.set(true);
    try {
      refinedText.set(await refine(text));
    } catch (err) {
      endpointError.set({ source: 'Refinement', message: err?.message ?? String(err) });
    } finally {
      isRefining.set(false);
    }
  }

  // Called when the Recorder component finishes a mic recording.
  async function handleRecorded(e) {
    const { blob, mimeType } = e.detail;
    audioBlob.set(blob);
    endpointError.set(null);
    refinedText.set('');
    isTranscribing.set(true);
    try {
      const text = await transcribe(blob, mimeType);
      rawText.set(text);
      await afterTranscribe(text);
    } catch (err) {
      endpointError.set({ source: 'Whisper', message: err?.message ?? String(err) });
    } finally {
      isTranscribing.set(false);
    }
  }

  // Open native file picker → transcribe on the Go side (no base64 round-trip).
  async function handleOpenFile() {
    endpointError.set(null);
    refinedText.set('');
    isTranscribing.set(true);
    try {
      const text = await openAndTranscribeFile();
      if (text) {
        rawText.set(text);
        await afterTranscribe(text);
      }
    } catch (err) {
      endpointError.set({ source: 'Whisper', message: err?.message ?? String(err) });
    } finally {
      isTranscribing.set(false);
    }
  }

  // Drag & drop: accept audio files dropped anywhere on the window.
  let dragging = false;
  function onDragOver(e) { e.preventDefault(); dragging = true; }
  function onDragLeave()  { dragging = false; }
  async function onDrop(e) {
    e.preventDefault();
    dragging = false;
    const file = e.dataTransfer?.files?.[0];
    if (!file) return;
    endpointError.set(null);
    refinedText.set('');
    isTranscribing.set(true);
    try {
      const text = await transcribe(file, file.type || 'audio/webm');
      rawText.set(text);
      await afterTranscribe(text);
    } catch (err) {
      endpointError.set({ source: 'Whisper', message: err?.message ?? String(err) });
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
    const s = get(settings);
    // Show setup wizard if integrated mode is selected but engine isn't ready.
    if (s.mode === 'integrated') {
      const state = await checkSetupState();
      if (!state.binaryExists || !state.modelExists) {
        showWizard.set(true);
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });
</script>

<!-- Settings panel (renders its own overlay internally) -->
<Settings />

{#if $showWizard}
  <!-- Full-screen setup wizard — replaces main UI during first-run -->
  <Wizard on:done={() => showWizard.set(false)} />
{:else}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="layout"
    class:drag-over={dragging}
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
          on:click={() => showSettings.set(true)}
          title="Open settings"
        >
          ⚙ Settings
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
          title="Transcribe an audio file from disk"
        >
          Open file…
        </button>
        <span class="drop-hint">or drop an audio file anywhere</span>
      </div>
    </section>

    {#if dragging}
      <div class="drop-overlay">Drop audio file to transcribe</div>
    {/if}

    <!-- ── Endpoint error ───────────────────────────────────── -->
    {#if $endpointError}
      <div class="error-banner">
        <span class="error-source">{$endpointError.source}</span>
        {$endpointError.message}
      </div>
    {/if}

    <!-- ── Transcript pane ──────────────────────────────────── -->
    <section class="panes">
      <TranscriptPane />
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
</style>
