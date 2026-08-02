package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// transcribeWhisper dispatches to the appropriate transcription backend based on settings.Mode.
func transcribeWhisper(audio []byte, mimeType string, s Settings) (string, error) {
	if s.Mode == "integrated" {
		return transcribeSubprocess(audio, mimeType, s)
	}
	return transcribeHTTP(audio, mimeType, s)
}

// ── Subprocess (integrated) mode ────────────────────────────────────────────

// transcribeFileAt transcribes an existing file on disk (any format ffmpeg can read).
// Used by the file picker and drag-drop paths where we already have a path.
func transcribeFileAt(filePath string, s Settings) (string, error) {
	if s.ModelName == "" {
		return "", fmt.Errorf("no model selected — run the setup wizard")
	}
	binPath, err := resolveWhisperBin()
	if err != nil {
		return "", fmt.Errorf("whisper-cli not found — run setup or install via `brew install whisper-cpp`")
	}
	modPath, err := modelPath(s.ModelName)
	if err != nil {
		return "", fmt.Errorf("model path: %w", err)
	}
	return runWhisper(binPath, modPath, filePath, s)
}

// transcribeSubprocess invokes the local whisper-cli binary and returns the transcript.
// Requires setup to have been completed via the wizard (binary + model downloaded).
func transcribeSubprocess(audio []byte, mimeType string, s Settings) (string, error) {
	if s.ModelName == "" {
		return "", fmt.Errorf("no model selected — run the setup wizard")
	}

	binPath, err := resolveWhisperBin()
	if err != nil {
		return "", fmt.Errorf("whisper-cli not found — run setup or install via `brew install whisper-cpp`")
	}
	modPath, err := modelPath(s.ModelName)
	if err != nil {
		return "", fmt.Errorf("model path: %w", err)
	}

	// Write audio to a temp file; whisper-cli needs a file path, not stdin.
	tmpAudio, err := os.CreateTemp("", "dictate-audio-*"+audioExt(mimeType))
	if err != nil {
		return "", fmt.Errorf("create temp audio: %w", err)
	}
	defer os.Remove(tmpAudio.Name())
	if _, err := tmpAudio.Write(audio); err != nil {
		tmpAudio.Close()
		return "", fmt.Errorf("write temp audio: %w", err)
	}
	tmpAudio.Close()
	log.Printf("transcribe: recorder reported mimeType %q, %d bytes -> %s", mimeType, len(audio), tmpAudio.Name())

	return runWhisper(binPath, modPath, tmpAudio.Name(), s)
}

// runWhisper converts srcPath to WAV if needed, then invokes whisper-cli.
func runWhisper(binPath, modPath, srcPath string, s Settings) (string, error) {
	wavPath, cleanup, err := convertToWAV(srcPath)
	if err != nil {
		return "", err
	}
	defer cleanup()

	// -np suppresses all non-result output so stdout contains only the transcript.
	// whisper-cli's own default for -l is "en", not auto-detect — omitting the
	// flag silently forces English decoding on non-English audio, which causes
	// hallucination loops and large stretches of the file decoding as empty.
	lang := s.WhisperLanguage
	if lang == "" {
		lang = "auto"
	}
	args := []string{
		"-m", modPath,
		"-f", wavPath,
		"--no-timestamps",
		"-np",
		"-l", lang,
	}
	if s.WhisperTask == "translate" {
		args = append(args, "--translate")
	}

	// If whisper-cli crashes right after loading a CPU backend variant (see
	// engine_variants.go), quarantine that variant and rerun — the scorer
	// picks the next-fastest one. Bounded by the number of variants shipped.
	const maxVariantRetries = 8
	for attempt := 0; ; attempt++ {
		cmd := exec.Command(binPath, args...)
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf
		log.Printf("whisper-cli attempt %d: %s", attempt+1, cmd.Args)
		stdout, err := cmd.Output()
		if err == nil {
			log.Printf("whisper-cli attempt %d: succeeded", attempt+1)
			return strings.TrimSpace(string(stdout)), nil
		}

		stderr := strings.TrimSpace(stderrBuf.String())
		code := exitCode(err)
		log.Printf("whisper-cli attempt %d: failed (exit %d): %s", attempt+1, code, stderr)

		// A silent crash right after loading a CPU backend variant (no error
		// text at all — see looksLikeBackendCrash) is the signature of a bad
		// variant, and worth retrying against the next-fastest one. Anything
		// else is a real whisper-cli error (bad audio, missing model, wrong
		// input format, …) that would just recur identically on retry, so it
		// must be surfaced as-is rather than run through the CPU-variant
		// diagnosis below — otherwise the actual error gets buried under an
		// irrelevant "try a smaller model / install vc_redist" message.
		if looksLikeBackendCrash(stderr) {
			if attempt < maxVariantRetries {
				if lib := loadedCPUVariant(stderr); lib != "" {
					if qErr := quarantineCPUVariant(binPath, lib); qErr == nil {
						continue
					}
				}
			}
			// Every variant we tried — including, in the worst case, the
			// universally-compatible baseline — crashed the same way. Sweep
			// the whole quarantine directory (not just what this call
			// quarantined) so healthy, faster builds don't stay disabled
			// forever because of an earlier session's mistaken quarantine.
			restoreAllQuarantinedVariants(binPath)
			return "", fmt.Errorf("whisper-cli: %s", diagnoseCrash(s.ModelName, code, stderr))
		}

		return "", fmt.Errorf("whisper-cli: %s", plainFailureMessage(code, stderr))
	}
}

// plainFailureMessage formats a whisper-cli failure that whisper-cli itself
// already explained (i.e. not the silent CPU-backend-crash signature — that
// case goes through diagnoseCrash instead). Returned as close to verbatim as
// possible: whisper-cli's own error text (e.g. an unsupported input format)
// is the most useful diagnostic available and must not be buried under
// unrelated guidance.
func plainFailureMessage(code int, stderr string) string {
	if stderr == "" {
		return fmt.Sprintf("(no output — process exit code %d)", code)
	}
	return stderr
}

// exitCode extracts the process exit code from cmd.Output()'s error, or -1
// if the process never started (e.g. binPath itself is missing/unrunnable).
func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// diagnoseCrash builds an actionable message for a whisper-cli failure that
// survived every CPU-variant retry. Reaching this point means the crash
// wasn't variant-specific, so the likely causes are environmental: not
// enough RAM for the selected model, or a missing C++ runtime dependency —
// both far more common on an arbitrary end-user Windows machine than a
// genuine ggml bug, and both fixable without a code change.
func diagnoseCrash(modelName string, code int, stderr string) string {
	if stderr == "" {
		stderr = fmt.Sprintf("(no output — process exit code %d)", code)
	} else {
		stderr = fmt.Sprintf("%s (exit code %d)", stderr, code)
	}
	return stderr + "\n\nThis crash happened the same way on every CPU build available, " +
		"including the safest baseline one, so it's unlikely to be about your CPU. Two " +
		"common causes on Windows:\n" +
		" • Not enough free RAM for the \"" + modelName + "\" model — try a smaller model in Settings.\n" +
		" • Missing Visual C++ Redistributable — install it from https://aka.ms/vs/17/release/vc_redist.x64.exe"
}

// convertToWAV converts any audio file to 16 kHz mono WAV using ffmpeg.
// Returns the WAV path and a cleanup function. The caller must call cleanup()
// even if err is non-nil to avoid temp file leaks.
func convertToWAV(srcPath string) (wavPath string, cleanup func(), err error) {
	ffmpeg, lookErr := exec.LookPath("ffmpeg")
	if lookErr != nil {
		return "", func() {}, fmt.Errorf("%s\n(Required to convert browser audio to a format whisper-cli can read.)",
			ffmpegInstallHint())
	}

	tmp, err := os.CreateTemp("", "dictate-wav-*.wav")
	if err != nil {
		return "", func() {}, fmt.Errorf("create wav temp: %w", err)
	}
	tmp.Close()
	wavPath = tmp.Name()
	cleanup = func() { os.Remove(wavPath) }

	cmd := exec.Command(ffmpeg,
		"-i", srcPath,
		"-ar", "16000", // 16 kHz — whisper works best at this rate
		"-ac", "1",     // mono
		"-f", "wav",
		"-y", // overwrite
		wavPath,
	)
	out, err := cmd.CombinedOutput()
	// ffmpeg logs the input stream it detected (codec, sample rate, channels)
	// to stderr on every run, success or failure — the one place that shows
	// what the browser actually recorded. Logging it unconditionally means a
	// "wrong input format" problem is visible here instead of only surfacing
	// as an opaque whisper-cli failure further down the pipeline.
	log.Printf("ffmpeg %s -> %s: %s", srcPath, wavPath, strings.TrimSpace(string(out)))
	if err != nil {
		return wavPath, cleanup, fmt.Errorf("audio conversion: %s", strings.TrimSpace(string(out)))
	}
	return wavPath, cleanup, nil
}

// ffmpegInstallHint returns a platform-specific instruction for installing
// ffmpeg, since there's no single command that works everywhere.
func ffmpegInstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "ffmpeg not found — install it with: winget install ffmpeg\n" +
			"(or download a build from https://www.gyan.dev/ffmpeg/builds/ and add it to PATH)"
	case "linux":
		return "ffmpeg not found — install it with your package manager, e.g.:\n" +
			"  sudo apt install ffmpeg   (Debian/Ubuntu)\n" +
			"  sudo dnf install ffmpeg   (Fedora)"
	default: // darwin
		return "ffmpeg not found — install it with: brew install ffmpeg"
	}
}

func audioExt(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return ".mp4"
	case strings.Contains(mimeType, "ogg"):
		return ".ogg"
	default:
		return ".webm"
	}
}

// ── HTTP mode ────────────────────────────────────────────────────────────────

// transcribeHTTP posts the recorded audio to the Whisper ASR service's /asr endpoint.
// Go's net/http is not subject to browser CORS restrictions, so no proxy is needed.
func transcribeHTTP(audio []byte, mimeType string, s Settings) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("audio_file", recordingFilename(mimeType))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("write audio: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	q := url.Values{}
	q.Set("task", s.WhisperTask)
	q.Set("output", "json")
	q.Set("encode", "true")
	if s.WhisperLanguage != "" {
		q.Set("language", s.WhisperLanguage)
	}

	endpoint := strings.TrimRight(s.WhisperURL, "/") + "/asr?" + q.Encode()
	req, err := http.NewRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper %d: %s", resp.StatusCode, string(msg))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode whisper response: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// recordingFilename picks a multipart filename whose extension matches the
// MediaRecorder mimeType, so the Whisper service's ffmpeg-based decoder gets
// an accurate hint.
func recordingFilename(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "mp4"):
		return "recording.mp4"
	case strings.Contains(mimeType, "ogg"):
		return "recording.ogg"
	case strings.Contains(mimeType, "webm"):
		return "recording.webm"
	default:
		return "recording.webm"
	}
}
