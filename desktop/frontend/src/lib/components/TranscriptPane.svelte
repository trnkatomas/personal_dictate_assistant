<script>
  import { rawText, isTranscribing } from '$lib/stores.js';
  import { t } from '$lib/i18n';

  let copied = false;

  async function copy() {
    await navigator.clipboard.writeText($rawText);
    copied = true;
    setTimeout(() => (copied = false), 1500);
  }
</script>

<div class="pane">
  <div class="pane-header">
    <h2>{$t.transcript.title}</h2>
    <div class="pane-actions">
      {#if $isTranscribing}
        <span class="badge loading">{$t.transcript.transcribing}</span>
      {:else if $rawText}
        <button class="action-btn" on:click={copy}>{copied ? $t.common.copied : $t.common.copy}</button>
      {/if}
    </div>
  </div>

  <textarea
    class="pane-body"
    bind:value={$rawText}
    placeholder={$t.transcript.placeholder}
    spellcheck="true"
    aria-label={$t.transcript.ariaLabel}
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

  .pane-body {
    flex: 1;
    padding: 0.85rem;
    background: #0f172a;
    border: 1px solid #334155;
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
    background: #1e3a8a;
    color: #93c5fd;
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
