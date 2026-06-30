# Dictate Assistant

A native desktop dictation app. Record or drop in audio, get a transcript,
optionally polish it with a local LLM — all running on your machine, no
cloud dependency required.

Built with [Wails v2](https://wails.io) (Go backend + native webview), so it
ships as a single installable app — no Docker, no Node server, no browser tab.

---

## Quick start

### Option A — download a release

Grab the latest build for your platform from the
[Releases page](https://github.com/trnkatomas/personal_dictate_assistant/releases):

- **macOS** — `dictate-assistant-macos-universal.zip` (Apple Silicon + Intel)
- **Windows** — `dictate-assistant-amd64-installer.exe`
- **Linux** — `dictate-assistant-linux-amd64.tar.gz`

On first launch, a setup wizard detects your hardware, recommends a Whisper
model, and downloads everything it needs. No prior setup required beyond
that.

> **macOS note:** the app isn't notarized yet, so Gatekeeper will block the
> first launch. Right-click the app → **Open** to bypass it once.

### Option B — build from source

```bash
cd desktop
wails build
open build/bin/dictate-assistant.app   # macOS; see build/bin/ on other platforms
```

Requires Go 1.23+, Node 18+, and the [Wails CLI](https://wails.io/docs/gettingstarted/installation).

---

## First-run setup

The first time you launch the app, a wizard walks through:

1. **Welcome** — what's about to happen
2. **Model recommendation** — based on detected CPU/GPU, RAM, and system
   locale, it suggests a Whisper model size (e.g. `large-v3-turbo` on Apple
   Silicon with ≥8 GB RAM, `small`/`medium` on CPU-only machines). You can
   override the suggestion.
3. **Download** — on macOS this installs `whisper-cpp` and `ffmpeg` via
   Homebrew, then downloads the chosen GGML model from Hugging Face. Linux
   downloads a prebuilt `whisper-cli` binary directly. Windows integrated
   mode isn't implemented yet — use HTTP mode instead (see below).
4. **Ready** — drop straight into the main app.

You can skip the wizard entirely ("I know what I'm doing") and configure an
HTTP Whisper endpoint manually in Settings instead.

---

## Usage

| Action | How |
|---|---|
| Record | Click **Record** (or press **Space**) and speak; click again to stop |
| Transcribe a file | Click **Open file…** and pick an audio file, or drag one onto the window |
| Edit | The transcript is a plain editable text box |
| Copy | **Copy** button in the pane header |

Supported file formats for drag/drop and the file picker: anything
`ffmpeg` can decode (mp3, mp4/m4a, wav, flac, ogg, webm, aac, …).

---

## Settings

### Transcription mode

| Mode | What it does |
|---|---|
| **Integrated** (default) | Runs `whisper-cli` as a local subprocess against the model downloaded by the wizard. Fully offline. |
| **HTTP endpoint** | Posts audio to an external Whisper ASR service instead — e.g. the bundled `docker-compose.yml` Whisper container, or a remote URL. |

Language and task (transcribe vs. translate-to-English) apply to both modes.

### Text refinement

Optional LLM pass that polishes the raw transcript — fixes punctuation,
capitalization, and obvious speech-to-text artifacts without changing
meaning or wording choices.

Works against **any OpenAI-compatible chat completions endpoint**, so it's
designed to run against a local model:

- [Ollama](https://ollama.com): `http://localhost:11434/v1`
- [llama.cpp's `llama-server`](https://github.com/ggerganov/llama.cpp): `http://localhost:8080/v1`
- LM Studio, or any other local/remote OpenAI-compatible server

Enable it in **Settings → Text refinement**, set the URL and model name
(e.g. `qwen3:1.7b` — small, multilingual, fast enough for this on a laptop).

Once enabled, the main view splits into two panes:

- **Transcript** — the raw output
- **Refined** — has its own editable prompt box and a **Refine** button
  (refinement is manual, not automatic, so you control when it runs)

A **⟷ Diff** toggle in the top bar switches to a word-level diff between
the raw and refined text, so you can see exactly what the model changed.

---

## Running Whisper via Docker (HTTP mode)

If you'd rather not install anything locally, or want GPU acceleration on
a machine without Apple Silicon, the repo includes a `docker-compose.yml`
for the [Whisper ASR webservice](https://github.com/ahmetoner/whisper-asr-webservice):

```bash
docker compose up -d
```

Then in the app, switch **Settings → Transcription mode** to **HTTP
endpoint** and point it at `http://localhost:9000`.

### Model size

Edit `ASR_MODEL` in `docker-compose.yml` and recreate the container:

```bash
docker compose up -d --force-recreate whisper
```

| Model | VRAM | Speed | Quality |
|---|---|---|---|
| `tiny` | ~1 GB | fastest | acceptable |
| `base` | ~1 GB | fast | good |
| `small` | ~2 GB | medium | better |
| `medium` | ~5 GB | slow | great |
| `large` / `large-v3-turbo` | ~10 GB | slowest / fast | best |

### GPU support

Uncomment the `deploy.resources` block in `docker-compose.yml` and switch
the image tag to `:latest-gpu`. Requires the
[NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

---

## Architecture

```
desktop/
├── main.go              — Wails app entrypoint, window options
├── app.go                — App struct, Transcribe/LoadSettings/SaveSettings bindings
├── whisper.go            — HTTP + subprocess transcription dispatch
├── refine.go              — LLM refinement via OpenAI-compatible chat completions
├── setup.go               — hardware detection, model download, setup wizard backend
├── settings.go            — settings.json persistence (~/Library/Application Support/dictate-assistant/ on macOS)
└── frontend/
    └── src/
        ├── App.svelte                       — shell: topbar, panes, drag & drop
        ├── lib/
        │   ├── api.js                       — thin wrapper around generated Wails bindings
        │   ├── stores.js                    — Svelte stores; settings persist via Go, not localStorage
        │   └── components/
        │       ├── Recorder.svelte           — mic capture + live waveform
        │       ├── TranscriptPane.svelte     — raw transcript (editable)
        │       ├── RefinementPane.svelte     — editable prompt + Refine button + output
        │       ├── DiffView.svelte           — word-level diff (raw vs. refined)
        │       ├── Settings.svelte           — mode toggle, refinement config
        │       └── Wizard.svelte             — 4-step first-run setup
        └── wailsjs/                          — auto-generated JS↔Go bindings (don't hand-edit)
```

Audio path: `MediaRecorder` (WebM/Opus) → base64 over the Wails JS↔Go
bridge → decoded in Go → converted to 16 kHz mono WAV via `ffmpeg` →
`whisper-cli` subprocess → transcript on stdout. File-picker transcriptions
skip the base64 round-trip and read the path directly.

There's no proxy layer and no CORS handling anywhere — Go's `net/http`
talks directly to local or remote services, and a desktop webview has no
concept of cross-origin restrictions to begin with.

---

## Development

```bash
cd desktop
wails dev
```

Hot-reloads both the Svelte frontend and (on save) the Go backend. Settings
are stored at:

- macOS: `~/Library/Application Support/dictate-assistant/settings.json`
- Windows: `%AppData%/dictate-assistant/settings.json`
- Linux: `$XDG_CONFIG_HOME/dictate-assistant/settings.json` (or `~/.config/...`)

Delete that file (or the whole `dictate-assistant/` folder) to reset to a
fresh first-run state and re-trigger the wizard.

### Releases

Pushing a `v*` tag triggers [`.github/workflows/release.yml`](.github/workflows/release.yml),
which builds macOS (universal), Windows, and Linux artifacts in parallel
and publishes them to a GitHub Release.

```bash
git tag v0.1.0
git push origin --tags
```
