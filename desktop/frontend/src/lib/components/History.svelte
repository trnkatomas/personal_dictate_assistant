<script>
  import { history, showHistory, rawText, refinedText } from '$lib/stores.js';
  import { t, locale } from '$lib/i18n';

  let copiedId = null;

  function formatTime(timestamp) {
    const localeTag = $locale === 'cs' ? 'cs-CZ' : 'en-US';
    return new Date(timestamp).toLocaleTimeString(localeTag, {
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  }

  // Loading an old entry back is a deliberate replace, regardless of the
  // append/overwrite setting — clicking a specific past transcript means
  // "show me that one", not "add it to whatever's already there".
  function restore(entry) {
    rawText.set(entry.text);
    refinedText.set('');
    showHistory.set(false);
  }

  async function copyEntry(e, entry) {
    e.stopPropagation();
    await navigator.clipboard.writeText(entry.text);
    copiedId = entry.id;
    setTimeout(() => { if (copiedId === entry.id) copiedId = null; }, 1500);
  }

  function clearHistory() {
    history.set([]);
  }

  function close() { showHistory.set(false); }

  function onOverlayClick(e) {
    if (e.target === e.currentTarget) close();
  }
</script>

{#if $showHistory}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={onOverlayClick}>
    <div class="panel" role="dialog" aria-label={$t.history.dialogLabel}>
      <div class="panel-header">
        <h3>{$t.history.title}</h3>
        <button class="close-btn" on:click={close}>✕</button>
      </div>

      {#if $history.length > 0}
        <p class="hint">{$t.history.restoreHint}</p>
        <div class="entries">
          {#each $history as entry (entry.id)}
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <div class="entry" on:click={() => restore(entry)}>
              <div class="entry-meta">
                <span class="entry-time">{formatTime(entry.timestamp)}</span>
                <button class="entry-copy" on:click={(e) => copyEntry(e, entry)}>
                  {copiedId === entry.id ? $t.common.copied : $t.common.copy}
                </button>
              </div>
              <p class="entry-preview">{entry.text}</p>
            </div>
          {/each}
        </div>
        <div class="panel-footer">
          <button class="clear-btn" on:click={clearHistory}>{$t.history.clear}</button>
        </div>
      {:else}
        <div class="empty">
          <p>{$t.history.emptyTitle}</p>
          <p class="hint">{$t.history.emptyHint}</p>
        </div>
      {/if}
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
    width: 460px;
    max-width: calc(100vw - 2rem);
    max-height: calc(100vh - 4rem);
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
    flex-shrink: 0;
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

  .hint {
    margin: 0;
    padding: 0.6rem 1.25rem 0;
    font-size: 0.72rem;
    color: #475569;
    flex-shrink: 0;
  }

  .entries {
    overflow-y: auto;
    padding: 0.6rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .entry {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: 0.5rem 0.65rem;
    cursor: pointer;
    transition: border-color 0.15s, background 0.15s;
  }
  .entry:hover {
    border-color: #7c3aed;
    background: #1e1b4b;
  }

  .entry-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.3rem;
  }

  .entry-time {
    font-size: 0.68rem;
    font-weight: 600;
    color: #64748b;
    font-variant-numeric: tabular-nums;
  }

  .entry-copy {
    font-size: 0.68rem;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    border: 1px solid #334155;
    background: transparent;
    color: #64748b;
    cursor: pointer;
    transition: all 0.15s;
  }
  .entry-copy:hover {
    background: #1e293b;
    color: #e2e8f0;
  }

  .entry-preview {
    margin: 0;
    font-size: 0.82rem;
    color: #cbd5e1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .panel-footer {
    display: flex;
    justify-content: flex-end;
    padding: 0.6rem 1.25rem 0.85rem;
    border-top: 1px solid #334155;
    flex-shrink: 0;
  }

  .clear-btn {
    background: none;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #64748b;
    font-size: 0.78rem;
    padding: 0.35rem 0.7rem;
    cursor: pointer;
    transition: all 0.15s;
  }
  .clear-btn:hover { background: #2d0a0a; color: #fca5a5; border-color: #7f1d1d; }

  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 2rem 1.25rem;
  }
  .empty p { margin: 0; font-size: 0.9rem; color: #64748b; text-align: center; }
  .empty .hint { padding: 0; font-size: 0.75rem; color: #475569; }
</style>
