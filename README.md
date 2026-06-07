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

First run downloads the Whisper model weights and the Ollama image.
Both are cached in named volumes so subsequent starts are instant.

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

The **Service endpoints** section has three built-in presets. Select one from
the dropdown — the URL fields immediately preview the values — then click
**Apply** to commit. Pick **Current** to discard the preview and revert to
whatever is saved.

| Preset | Whisper URL | Ollama URL |
|--------|-------------|------------|
| Docker Compose | `http://whisper:9000` | `http://ollama:11434` |
| Local | `http://localhost:9000` | `http://localhost:11434` |
| External | *(clear — enter your own)* | *(clear — enter your own)* |

> **Note:** these URLs are resolved by the SvelteKit server, not the browser.
> Use Docker service names (e.g. `whisper`) when the app itself runs in Docker Compose,
> and `localhost` when running the dev server directly on your machine.

| Setting | Default | Notes |
|---------|---------|-------|
| Ollama model | `gemma3:1b` | Must be pulled first (see step 2) |
| Language | *(auto)* | ISO 639-1 code, e.g. `en`, `de`, `cs` |
| Task | Transcribe | Switch to *Translate* to get English output |

---

## Whisper model size

### Changing the model (requires restart)

Edit `ASR_MODEL` in `docker-compose.yml` and recreate the whisper container:

```bash
docker compose up -d --force-recreate whisper
```

Model weights are cached in the `whisper-models` volume, so restarting with a
previously-used model takes only a few seconds.

| Model | VRAM | Speed | Quality |
|-------|------|-------|---------|
| `tiny`   | ~1 GB  | fastest | acceptable |
| `base`   | ~1 GB  | fast    | good |
| `small`  | ~2 GB  | medium  | better |
| `medium` | ~5 GB  | slow    | great |
| `large`  | ~10 GB | slowest | best |

### Switching between model sizes without restarting

The Whisper service loads its model once at startup and offers no API to change
it at runtime. The cleanest workaround is to run multiple Whisper containers
in parallel — one per model size — and switch between them via the Settings
endpoint presets.

Add extra whisper services to `docker-compose.yml`:

```yaml
services:
  whisper-small:
    image: onerahmet/openai-whisper-asr-webservice:latest
    environment:
      - ASR_MODEL=small
      - ASR_ENGINE=openai_whisper
    volumes:
      - whisper-models:/root/.cache/whisper

  whisper-large:
    image: onerahmet/openai-whisper-asr-webservice:latest
    environment:
      - ASR_MODEL=large
      - ASR_ENGINE=openai_whisper
    volumes:
      - whisper-models:/root/.cache/whisper   # shared cache — no duplicate downloads
```

Then add matching presets in `Settings.svelte`:

```js
{ id: 'docker-small', label: 'Docker — Whisper small',
  whisperUrl: 'http://whisper-small:9000', ollamaUrl: 'http://ollama:11434' },
{ id: 'docker-large', label: 'Docker — Whisper large',
  whisperUrl: 'http://whisper-large:9000', ollamaUrl: 'http://ollama:11434' },
```

Model switching becomes instant — just pick a preset and click **Apply**.
The trade-off is that both models occupy RAM simultaneously.

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
Make sure Whisper and Ollama are already running:

```bash
docker compose up whisper ollama -d
```

In the app's Settings, use the **Local** preset so the URLs point at
`localhost` (where the services are port-forwarded from Docker).

---

## Architecture

```
docker-compose.yml
├── whisper   onerahmet/openai-whisper-asr-webservice  :9000
├── ollama    ollama/ollama                            :11434
└── app       SvelteKit Node server                    :3000

app/
├── src/
│   ├── lib/
│   │   ├── server/
│   │   │   └── proxy.js           — server-side proxy helper (avoids CORS)
│   │   ├── stores.js              — Svelte stores (persisted to localStorage)
│   │   ├── api.js                 — fetch wrappers for Whisper & Ollama
│   │   └── components/
│   │       ├── Recorder.svelte         — mic + live waveform canvas
│   │       ├── TranscriptPane.svelte   — left pane (raw text)
│   │       ├── TransformPane.svelte    — right pane (prompt + streaming output)
│   │       ├── DiffView.svelte         — jsdiff + diff2html word-level diff
│   │       └── Settings.svelte         — modal settings panel with presets
│   └── routes/
│       ├── api/whisper/[...path]/+server.js  — proxy → Whisper
│       ├── api/ollama/[...path]/+server.js   — proxy → Ollama
│       └── +page.svelte                      — main page
└── Dockerfile   — multi-stage: node build → node serve
```

### Request flow

The browser always talks to the SvelteKit server on the same origin.
The server proxies requests to Whisper and Ollama using the URL configured
in Settings, which is sent as an `X-Proxy-Target` header. This means:

- No CORS issues regardless of where the upstream services run
- Upstream URLs are changed at runtime through the Settings UI — no rebuild needed
- The URL must be reachable from the **server** (inside Docker), not the browser
