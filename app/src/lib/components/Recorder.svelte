<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { browser } from '$app/environment';
  import { isRecording } from '$lib/stores.js';

  const dispatch = createEventDispatcher();

  let canvas;
  let stream      = null;
  let recorder    = null;
  let audioCtx    = null;
  let analyser    = null;
  let animFrame   = null;
  let chunks      = [];

  // ── Canvas sizing ──────────────────────────────────────────────
  onMount(() => {
    fitCanvas();
    window.addEventListener('resize', fitCanvas);
    drawIdle();
    return () => window.removeEventListener('resize', fitCanvas);
  });

  function fitCanvas() {
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    canvas.width  = rect.width  * window.devicePixelRatio;
    canvas.height = rect.height * window.devicePixelRatio;
    const ctx = canvas.getContext('2d');
    ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
    if (!$isRecording) drawIdle();
  }

  // ── Recording ──────────────────────────────────────────────────
  async function start() {
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (e) {
      alert(`Microphone access denied: ${e.message}`);
      return;
    }

    audioCtx = new AudioContext();
    const source = audioCtx.createMediaStreamSource(stream);
    analyser = audioCtx.createAnalyser();
    analyser.fftSize = 2048;
    source.connect(analyser);

    chunks = [];
    const mimeType = getBestMime();
    recorder = new MediaRecorder(stream, mimeType ? { mimeType } : {});
    recorder.ondataavailable = (e) => { if (e.data.size > 0) chunks.push(e.data); };
    recorder.onstop = () => {
      const blob = new Blob(chunks, { type: recorder.mimeType });
      dispatch('recorded', blob);
      cleanup();
    };

    recorder.start(100);          // chunk every 100 ms
    isRecording.set(true);
    drawLive();
  }

  function stop() {
    if (recorder?.state !== 'inactive') recorder.stop();
    isRecording.set(false);
    cancelAnimationFrame(animFrame);
    drawIdle();
  }

  function toggle() {
    if ($isRecording) stop(); else start();
  }

  function cleanup() {
    stream?.getTracks().forEach((t) => t.stop());
    audioCtx?.close();
    stream   = null;
    audioCtx = null;
    analyser = null;
  }

  function getBestMime() {
    const candidates = [
      'audio/webm;codecs=opus',
      'audio/webm',
      'audio/ogg;codecs=opus',
      'audio/mp4'
    ];
    return candidates.find((t) => MediaRecorder.isTypeSupported(t)) ?? '';
  }

  // ── Waveform drawing ───────────────────────────────────────────
  function drawLive() {
    if (!analyser || !canvas) return;
    const ctx    = canvas.getContext('2d');
    const buf    = new Uint8Array(analyser.frequencyBinCount);
    analyser.getByteTimeDomainData(buf);

    const W = canvas.width  / window.devicePixelRatio;
    const H = canvas.height / window.devicePixelRatio;
    ctx.clearRect(0, 0, W, H);

    ctx.lineWidth   = 1.5;
    ctx.strokeStyle = '#4ade80';
    ctx.beginPath();

    const step = W / buf.length;
    for (let i = 0; i < buf.length; i++) {
      const y = (buf[i] / 128) * (H / 2);
      i === 0 ? ctx.moveTo(0, y) : ctx.lineTo(i * step, y);
    }
    ctx.lineTo(W, H / 2);
    ctx.stroke();

    animFrame = requestAnimationFrame(drawLive);
  }

  function drawIdle() {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const W   = canvas.width  / window.devicePixelRatio;
    const H   = canvas.height / window.devicePixelRatio;
    ctx.clearRect(0, 0, W, H);
    ctx.lineWidth   = 1;
    ctx.strokeStyle = '#1f2937';
    ctx.beginPath();
    ctx.moveTo(0, H / 2);
    ctx.lineTo(W, H / 2);
    ctx.stroke();
  }

  onDestroy(() => {
    if (!browser) return;
    cancelAnimationFrame(animFrame);
    if (recorder?.state !== 'inactive') recorder?.stop();
    cleanup();
  });
</script>

<div class="recorder">
  <canvas bind:this={canvas} class="waveform"></canvas>

  <button
    class="record-btn"
    class:recording={$isRecording}
    on:click={toggle}
    title={$isRecording ? 'Stop recording (Space)' : 'Start recording (Space)'}
  >
    {#if $isRecording}
      <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><rect x="5" y="5" width="14" height="14" rx="2"/></svg>
      Stop
    {:else}
      <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><circle cx="12" cy="12" r="7"/></svg>
      Record
    {/if}
  </button>
</div>

<style>
  .recorder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
  }

  .waveform {
    width: 100%;
    height: 72px;
    background: #0a0f1a;
    border: 1px solid #1e293b;
    border-radius: 8px;
    display: block;
  }

  .record-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.6rem 1.75rem;
    border-radius: 9999px;
    border: none;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    background: #2563eb;
    color: #fff;
    transition: background 0.15s, transform 0.1s;
    user-select: none;
  }

  .record-btn:hover  { background: #1d4ed8; }
  .record-btn:active { transform: scale(0.97); }

  .record-btn.recording {
    background: #dc2626;
    animation: pulse-red 1.5s ease-in-out infinite;
  }

  @keyframes pulse-red {
    0%, 100% { box-shadow: 0 0 0 0 rgba(220,38,38,0.4); }
    50%       { box-shadow: 0 0 0 8px rgba(220,38,38,0); }
  }
</style>
