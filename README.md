# Dictate Assistant

Local, privacy-first voice transcription with LLM cleanup.

```
Browser → Whisper ASR (speech-to-text) → Ollama (grammar / punctuation fix)
```

Everything runs locally via Docker Compose — no cloud, no data leaves your machine.

---

## Quick start

### 1. Start the stack

```bash
docker compose up -d
```

First run downloads the Whisper model weights (~150 MB for `base`) and the
Ollama image. Both are cached in named volumes so subsequent starts are instant.

### 2. Pull an Ollama model

```bash
docker exec -it dictate-assistant-ollama-1 ollama pull gemma3:1b
```

Any model listed on <https://ollama.com/library> works. Smaller = faster on CPU.
Good choices: `gemma3:1b` (default), `gemma4:4b`, `llama3.2`, `phi4-mini`.

### 3. Open the app

<http://localhost:3000>

---

## Usage

| Step | Action |
|------|--------|
| 1 | Click **Record** (or press **Space**) and speak |
| 2 | Click **Stop** — audio is sent to Whisper automatically |
| 3 | Raw transcription appears in the left pane (editable) |
| 4 | Edit the prompt if needed, then click **Transform ↗** |
| 5 | Cleaned text streams into the right pane |
| 6 | Click **⟷ Diff** to see exactly what the LLM changed |

All settings (API URLs, model name, prompt, language) are persisted in
`localStorage` — they survive page reloads.

---

## Settings

Open ⚙ **Settings** in the top-right corner.

| Setting | Default | Notes |
|---------|---------|-------|
| Whisper URL | `http://localhost:9000` | Exposed by Docker Compose |
| Ollama URL | `http://localhost:11434` | Exposed by Docker Compose |
| Ollama model | `gemma3:1b` | Must be pulled first (see step 2) |
| Language | *(auto)* | ISO 639-1 code, e.g. `en`, `de`, `cs` |
| Task | Transcribe | Switch to *Translate* to get English output |

---

## Whisper model size

Edit `ASR_MODEL` in `docker-compose.yml`:

| Model | VRAM | Speed | Quality |
|-------|------|-------|---------|
| `tiny`   | ~1 GB | fastest | acceptable |
| `base`   | ~1 GB | fast    | good ✓ default |
| `small`  | ~2 GB | medium  | better |
| `medium` | ~5 GB | slow    | great |
| `large`  | ~10 GB | slowest | best |

---

## GPU support

Uncomment the `deploy.resources` blocks in `docker-compose.yml` and switch
the Whisper image tag to `:latest-gpu`. Requires the
[NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

---

## Local development (hot-reload)

```bash
cd app
npm install
npm run dev
```

The dev server starts on <http://localhost:5173>.  
Make sure Whisper and Ollama are already running (`docker compose up whisper ollama -d`).

---

## Architecture

```
docker-compose.yml
├── whisper   onerahmet/openai-whisper-asr-webservice  :9000
├── ollama    ollama/ollama                            :11434
└── app       nginx serving the Svelte SPA             :3000 → :80

app/
├── src/
│   ├── lib/
│   │   ├── stores.js          — Svelte stores (persisted to localStorage)
│   │   ├── api.js             — fetch wrappers for Whisper & Ollama
│   │   └── components/
│   │       ├── Recorder.svelte       — mic + live waveform canvas
│   │       ├── TranscriptPane.svelte — left pane (raw text)
│   │       ├── TransformPane.svelte  — right pane (prompt + streaming output)
│   │       ├── DiffView.svelte       — jsdiff + diff2html word-level diff
│   │       └── Settings.svelte       — modal settings panel
│   └── routes/
│       └── +page.svelte       — main page wiring everything together
├── Dockerfile                 — multi-stage: node build → nginx serve
└── nginx.conf                 — SPA fallback to index.html
```
