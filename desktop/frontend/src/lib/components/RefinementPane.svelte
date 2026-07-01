<script>
  import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime';
  import { settings, refinedText, isRefining, rawText, endpointError } from '$lib/stores.js';
  import { refine } from '$lib/api.js';

  let copied = false;
  let progress = null; // { current, total } | null — only set for multi-chunk refinements

  async function handleRefine() {
    if (!$rawText.trim()) return;
    endpointError.set(null);
    refinedText.set('');
    isRefining.set(true);
    progress = null;

    EventsOn('refine:progress', (p) => {
      progress = p.done ? null : { current: p.current, total: p.total };
    });

    try {
      refinedText.set(await refine($rawText));
    } catch (err) {
      endpointError.set({ source: 'Refinement', message: err?.message ?? String(err) });
    } finally {
      isRefining.set(false);
      progress = null;
      EventsOff('refine:progress');
    }
  }

  async function copy() {
    await navigator.clipboard.writeText($refinedText);
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }
</script>

<div class="pane">
  <div class="pane-header">
    <h2>Refined</h2>
    <div class="pane-actions">
      {#if $isRefining}
        <span class="badge loading">
          {progress ? `Refining ${progress.current}/${progress.total}…` : 'Refining…'}
        </span>
      {:else if $refinedText}
        <button class="action-btn" on:click={copy}>{copied ? '✓ Copied' : 'Copy'}</button>
      {/if}
    </div>
  </div>

  <!-- Editable prompt strip -->
  <div class="prompt-strip">
    <textarea
      class="prompt-input"
      bind:value={$settings.refinementPrompt}
      rows="2"
      placeholder="Refinement instruction prompt…"
      aria-label="Refinement prompt"
    ></textarea>
    <button
      class="refine-btn"
      on:click={handleRefine}
      disabled={$isRefining || !$rawText.trim()}
    >
      {#if $isRefining}
        <svg class="spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
          <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>
        </svg>
      {:else}
        ↗
      {/if}
      Refine
    </button>
  </div>

  <textarea
    class="pane-body"
    bind:value={$refinedText}
    placeholder="Refined text will appear here…"
    spellcheck="true"
    aria-label="Refined text"
  ></textarea>
</div>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }

  .pane-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.45rem 0.85rem;
    background: #1e293b;
    border: 1px solid #334155;
    border-bottom: none;
    border-radius: 8px 8px 0 0;
    flex-shrink: 0;
  }

  h2 {
    margin: 0;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: #64748b;
  }

  .pane-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  /* Prompt strip */
  .prompt-strip {
    display: flex;
    gap: 0.5rem;
    align-items: stretch;
    padding: 0.5rem;
    background: #0f1e33;
    border: 1px solid #334155;
    border-top: none;
    border-bottom: none;
    flex-shrink: 0;
  }

  .prompt-input {
    flex: 1;
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #94a3b8;
    font-size: 0.78rem;
    line-height: 1.5;
    padding: 0.4rem 0.6rem;
    resize: none;
    font-family: inherit;
    outline: none;
    transition: border-color 0.15s;
  }

  .prompt-input:focus { border-color: #7c3aed; }

  .refine-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0 1rem;
    background: #7c3aed;
    color: #fff;
    border: none;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .refine-btn:hover:not(:disabled) { background: #6d28d9; }
  .refine-btn:disabled { opacity: 0.45; cursor: not-allowed; }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to   { transform: rotate(360deg); }
  }

  .pane-body {
    flex: 1;
    padding: 0.85rem;
    background: #0f172a;
    border: 1px solid #334155;
    border-top: none;
    border-radius: 0 0 8px 8px;
    color: #e2e8f0;
    font-size: 0.95rem;
    line-height: 1.7;
    resize: none;
    font-family: inherit;
    outline: none;
    width: 100%;
  }

  .pane-body::placeholder { color: #334155; }

  .badge {
    font-size: 0.7rem;
    padding: 0.15rem 0.55rem;
    border-radius: 9999px;
    font-weight: 600;
  }

  .loading {
    background: #3b0764;
    color: #d8b4fe;
    animation: blink 1.4s ease-in-out infinite;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50%       { opacity: 0.5; }
  }

  .action-btn {
    font-size: 0.7rem;
    padding: 0.15rem 0.55rem;
    border-radius: 4px;
    border: 1px solid #334155;
    background: transparent;
    color: #64748b;
    cursor: pointer;
    transition: all 0.15s;
  }

  .action-btn:hover {
    background: #1e293b;
    color: #e2e8f0;
  }
</style>
