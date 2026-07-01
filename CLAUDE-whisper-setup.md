# Project Context: Local Whisper Transcription App

## Goal
A distributable desktop app that lets regular (non-technical) users run Whisper speech-to-text locally, with a simple UI and no complex setup.

## Transcription Engine
- **Model**: Whisper large-v3 or large-v3-turbo
- **Runtime**: [`faster-whisper-xxl`](https://github.com/Purfview/whisper-standalone-win) — standalone pre-built binary, no Python install required
- **Mode**: CPU-only with INT8 quantization (`--compute_type int8`) — fast enough on modern hardware, avoids CUDA complexity
- **CUDA libs are a separate optional download** — the base binary works CPU-only and is ~150MB (not 2GB)

## API / Docker (for dev/server use)
- **Image**: [`ahmetoner/whisper-asr-webservice`](https://github.com/ahmetoner/whisper-asr-webservice) (onerahmet on Docker Hub) — the de facto standard, 3k stars, 1M+ pulls
- Exposes an **OpenAI-compatible REST API** on port 9000, Swagger UI at `/docs`
- Supports faster-whisper backend, multiple output formats, word timestamps, VAD, speaker diarization
- GPU via `--gpus all`, model set via `-e ASR_MODEL=large-v3`

## Distribution Approach
- **Not** Docker for end users (requires Docker Desktop)
- **Not** Go + whisper.cpp bindings (CGO kills cross-platform builds)
- **Yes**: `faster-whisper-xxl.exe` + model file + Wails GUI installer

### Model download
- Model (~3GB for large-v3-turbo) must be downloaded **before first run**
- The installer/setup app handles this download with a progress UI

## GUI Framework: Wails
- **[Wails v2](https://wails.io)** (stable) — Go backend + HTML/CSS/JS frontend, single native binary
- No embedded browser (uses OS native webview), lightweight Electron alternative
- Go handles: platform detection, HTTP model download, process management, launching faster-whisper
- Frontend: plain HTML/CSS/JS (or React/Vue) for the setup wizard and UI
- Truly cross-platform binary — same codebase for Windows, macOS, Linux

```
[Wails binary]
  ├── Go backend: download model, launch faster-whisper-xxl, manage process
  └── HTML frontend: setup wizard, progress bar, simple transcription UI
```

## Key Decisions & Trade-offs
| Decision | Rationale |
|---|---|
| CPU-only | Avoids CUDA/driver complexity for end users |
| faster-whisper-xxl over whisper.cpp | Pre-built, Python-free, good Windows support |
| Wails over TUI (Bubbletea) | Regular users need a GUI, not a terminal |
| Wails over Fyne | Web-based UI is more flexible and familiar to style |
| Wails v2 over v3 | v3 is still alpha; v2 is production-stable |
| No Docker for distribution | Too much friction for non-technical users |
| Docker Compose kept for dev/server use | One-liner deploy: `docker compose -f https://github.com/... up` |

## Open Questions
- Which frontend framework (vanilla HTML, React, Vue) for the Wails UI?
- Whether to bundle the model file (adds ~3GB) or always download on first run
- macOS / Linux support scope — Windows is primary target
