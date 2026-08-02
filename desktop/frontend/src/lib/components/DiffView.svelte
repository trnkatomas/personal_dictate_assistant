<script>
  import { rawText, refinedText } from '$lib/stores.js';
  import { diffWords } from 'diff';
  import { t } from '$lib/i18n';

  $: parts = ($rawText && $refinedText)
    ? diffWords($rawText, $refinedText)
    : [];

  $: removedCount = parts.filter(p => p.removed).length;
  $: addedCount   = parts.filter(p => p.added).length;
</script>

<div class="diff-pane">
  <div class="diff-header">
    <div class="diff-title">
      <span class="side old">{$t.diff.transcriptionSide}</span>
      <span class="arrow">→</span>
      <span class="side new">{$t.diff.refinedSide}</span>
    </div>
    {#if parts.length > 0}
      <div class="stats">
        <span class="stat del">{$t.diff.removedWords(removedCount)}</span>
        <span class="stat ins">{$t.diff.addedWords(addedCount)}</span>
      </div>
    {/if}
  </div>

  {#if parts.length > 0}
    <div class="diff-body">
      <p class="diff-text">
        {#each parts as part}
          {#if part.removed}
            <del>{part.value}</del>
          {:else if part.added}
            <ins>{part.value}</ins>
          {:else}
            {part.value}
          {/if}
        {/each}
      </p>
    </div>
  {:else}
    <div class="empty">
      <p>{$t.diff.emptyTitle}</p>
      <p class="hint">{$t.diff.emptyHint}</p>
    </div>
  {/if}
</div>

<style>
  .diff-pane {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
    border: 1px solid #334155;
    border-radius: 8px;
    overflow: hidden;
  }

  .diff-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.45rem 0.85rem;
    background: #1e293b;
    border-bottom: 1px solid #334155;
    flex-shrink: 0;
  }

  .diff-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .side.old  { color: #f87171; }
  .side.new  { color: #4ade80; }
  .arrow     { color: #475569; }

  .stats {
    display: flex;
    gap: 0.6rem;
  }

  .stat {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 0.1rem 0.45rem;
    border-radius: 9999px;
  }

  .stat.del { background: #2d0a0a; color: #f87171; }
  .stat.ins { background: #052e16; color: #4ade80; }

  .diff-body {
    flex: 1;
    overflow-y: auto;
    padding: 1rem 1.25rem;
    background: #0f172a;
  }

  .diff-text {
    margin: 0;
    font-size: 1rem;
    line-height: 1.85;
    color: #e2e8f0;
    white-space: pre-wrap;
    word-break: break-word;
  }

  del {
    background: #2d0a0a;
    color: #fca5a5;
    text-decoration: line-through;
    text-decoration-color: #ef4444;
    border-radius: 3px;
    padding: 0.05em 0.2em;
  }

  ins {
    background: #052e16;
    color: #86efac;
    text-decoration: none;
    border-radius: 3px;
    padding: 0.05em 0.2em;
  }

  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
  }

  .empty p   { margin: 0; font-size: 0.95rem; color: #334155; }
  .hint      { font-size: 0.78rem !important; color: #1e293b !important; }
</style>
