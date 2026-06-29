<script>
  import { onMount } from 'svelte';
  import Recorder       from '$lib/components/Recorder.svelte';
  import TranscriptPane  from '$lib/components/TranscriptPane.svelte';
  import Settings        from '$lib/components/Settings.svelte';

  import {
    rawText, isTranscribing, isRecording,
    showSettings, audioBlob, endpointError
  } from '$lib/stores.js';
  import { transcribe } from '$lib/api.js';

  async function handleRecorded(e) {
    const blob = e.detail;
    audioBlob.set(blob);
    endpointError.set(null);
    isTranscribing.set(true);

    try {
      const text = await transcribe(blob);
      rawText.set(text);
    } catch (err) {
      endpointError.set({ source: 'Whisper', message: err.message });
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
    // Recorder component handles its own toggle via the button;
    // we programmatically click the record button so all logic stays there.
    document.querySelector('.record-btn')?.click();
  }

  onMount(() => {
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });
</script>

<Settings />

<div class="layout">
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
  </section>

  <!-- ── Unified endpoint error ─────────────────────────── -->
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

<style>
  .layout {
    display: flex;
    flex-direction: column;
    height: 100dvh;
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
  }

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
