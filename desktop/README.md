# Dictate Assistant — desktop app

Wails v2 (Go + Svelte webview) app. See the [repo root README](../README.md)
for what this app does and how to use it.

## Development

```bash
wails dev
```

Hot-reloads the Svelte frontend on save; rebuilds and restarts on Go
changes. After any change to bound `App` method signatures, Wails
regenerates `frontend/wailsjs/go/main/App.{js,d.ts}` automatically — don't
hand-edit those files.

## Building

```bash
wails build
```

Produces a platform-native bundle in `build/bin/`. See
[`../.github/workflows/release.yml`](../.github/workflows/release.yml) for
how this is automated across macOS/Windows/Linux on tagged releases.

## Layout

| File | Responsibility |
|---|---|
| `app.go` | Bound `App` struct — `Transcribe`, `OpenAndTranscribeFile`, `LoadSettings`/`SaveSettings` |
| `whisper.go` | HTTP and subprocess transcription backends |
| `refine.go` | LLM text refinement via OpenAI-compatible chat completions |
| `setup.go` | Hardware detection, binary/model download, first-run wizard backend |
| `settings.go` | `settings.json` persistence under the OS config dir |
| `frontend/src/` | Svelte UI — see component table in the root README |
