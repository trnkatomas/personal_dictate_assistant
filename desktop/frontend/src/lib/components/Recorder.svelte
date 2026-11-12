<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { isRecording } from '$lib/stores.js';
  import { t } from '$lib/i18n';
  import { UsesNativeRecorder, StartRecording, CancelRecording } from '../../../wailsjs/go/main/App';

  const dispatch = createEventDispatcher();

  let canvas;
  let stream      = null;
  let recorder    = null;
  let audioCtx    = null;
  let analyser    = null;
  let animFrame   = null;
  let chunks      = [];
  let startedAt   = 0;

  // On Linux the webview can't record (see desktop/recorder.go): the Go side
  // captures with ffmpeg instead, and there's no audio stream here to draw a
  // waveform from — just an elapsed-time indicator.
  let native      = false;
  let elapsed     = 0;
  let elapsedTimer = null;
  $: elapsedLabel = `${Math.floor(elapsed / 60)}:${String(elapsed % 60).padStart(2, '0')}`;

  // ── Canvas sizing ──────────────────────────────────────────────
  // ResizeObserver fires once the element actually has non-zero dimensions,
  // which is more reliable than relying on onMount timing in WKWebView.
  let canvasObserver;

  onMount(() => {
    UsesNativeRecorder().then((v) => { native = v; }).catch(() => {});
    canvasObserver = new ResizeObserver(() => fitCanvas());
    canvasObserver.observe(canvas);
    window.addEventListener('resize', fitCanvas);
    return () => {
      canvasObserver?.disconnect();
      window.removeEventListener('resize', fitCanvas);
    };
  });

  function fitCanvas() {
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) return;
    canvas.width  = rect.width  * window.devicePixelRatio;
    canvas.height = rect.height * window.devicePixelRatio;
    const ctx = canvas.getContext('2d');
    ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
    if (!$isRecording) drawIdle();
  }

  // ── Recording ──────────────────────────────────────────────────
  // StartRecording waits ~0.6 s to confirm ffmpeg is actually capturing;
  // ignore further Record clicks / Space presses until it answers.
  let nativeStarting = false;

  async function startNative() {
    if (nativeStarting) return;
    nativeStarting = true;
    try {
      await StartRecording();
    } catch (e) {
      alert($t.recorder.nativeStartFailed(e?.message ?? String(e)));
      return;
    } finally {
      nativeStarting = false;
    }
    startedAt = Date.now();
    elapsed = 0;
    elapsedTimer = setInterval(() => { elapsed = Math.floor((Date.now() - startedAt) / 1000); }, 250);
    isRecording.set(true);
  }

  function stopNative() {
    clearInterval(elapsedTimer);
    elapsedTimer = null;
    isRecording.set(false);
    // The Go side still holds the capture; App.svelte stops it and transcribes.
    dispatch('nativeStopped', { durationSeconds: (Date.now() - startedAt) / 1000 });
  }

  async function start() {
    if (native) return startNative();
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
    } catch (e) {
      alert($t.recorder.micDenied(e.message));
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
      const durationSeconds = (Date.now() - startedAt) / 1000;
      dispatch('recorded', { blob, mimeType: recorder.mimeType, durationSeconds });
      cleanup();
    };

    recorder.start(100);          // chunk every 100 ms
    startedAt = Date.now();
    isRecording.set(true);
    drawLive();
  }

  function stop() {
    if (native) return stopNative();
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
    if (native && $isRecording) {
      clearInterval(elapsedTimer);
      isRecording.set(false);
      CancelRecording();
    }
    cancelAnimationFrame(animFrame);
    if (recorder?.state !== 'inactive') recorder?.stop();
    cleanup();
  });
</script>

<div class="recorder">
  <canvas bind:this={canvas} class="waveform" class:hidden={native}></canvas>
  {#if native}
    <div class="native-status" class:active={$isRecording}>
      {#if $isRecording}
        <span class="rec-dot"></span>{$t.recorder.recordingElapsed(elapsedLabel)}
      {:else if nativeStarting}
        {$t.recorder.nativeStarting}
      {:else}
        {$t.recorder.nativeIdle}
      {/if}
    </div>
  {/if}

  <button
    class="record-btn"
    class:recording={$isRecording}
    on:click={toggle}
    title={$isRecording ? $t.recorder.stopTitle : $t.recorder.startTitle}
  >
    {#if $isRecording}
      <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><rect x="5" y="5" width="14" height="14" rx="2"/></svg>
      {$t.recorder.stop}
    {:else}
      <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><circle cx="12" cy="12" r="7"/></svg>
      {$t.recorder.record}
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

  .waveform.hidden { display: none; }

  .native-status {
    width: 100%;
    height: 72px;
    background: #0a0f1a;
    border: 1px solid #1e293b;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    font-size: 0.85rem;
    color: #475569;
    font-variant-numeric: tabular-nums;
  }
  .native-status.active { color: #fca5a5; }

  .rec-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: #dc2626;
    animation: blink 1s ease-in-out infinite;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50%      { opacity: 0.3; }
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
