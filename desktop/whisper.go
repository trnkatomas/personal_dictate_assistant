package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// transcribeSubprocess invokes the local whisper-cli binary and returns the transcript.
// Requires setup to have been completed via the wizard (binary + model downloaded).
func transcribeSubprocess(audio []byte, mimeType string, s Settings) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("Windows integrated mode not yet implemented — use HTTP mode instead")
	}
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

	// whisper-cli (brew build) uses miniaudio which only handles WAV/MP3/FLAC/OGG.
	// WKWebView produces WebM/Opus, so we convert to 16 kHz mono WAV first.
	audioPath, cleanup, err := convertToWAV(tmpAudio.Name())
	if err != nil {
		return "", err
	}
	defer cleanup()

	// -np suppresses all non-result output so stdout contains only the transcript.
	args := []string{
		"-m", modPath,
		"-f", audioPath,
		"--no-timestamps",
		"-np",
	}
	if s.WhisperLanguage != "" {
		args = append(args, "-l", s.WhisperLanguage)
	}
	if s.WhisperTask == "translate" {
		args = append(args, "--translate")
	}

	cmd := exec.Command(binPath, args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("whisper-cli: %s", strings.TrimSpace(stderrBuf.String()))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// convertToWAV converts any audio file to 16 kHz mono WAV using ffmpeg.
// Returns the WAV path and a cleanup function. The caller must call cleanup()
// even if err is non-nil to avoid temp file leaks.
func convertToWAV(srcPath string) (wavPath string, cleanup func(), err error) {
	ffmpeg, lookErr := exec.LookPath("ffmpeg")
	if lookErr != nil {
		return "", func() {}, fmt.Errorf(
			"ffmpeg not found — install it with: brew install ffmpeg\n" +
				"(Required to convert browser audio to a format whisper-cli can read.)")
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
	if out, err := cmd.CombinedOutput(); err != nil {
		return wavPath, cleanup, fmt.Errorf("audio conversion: %s", strings.TrimSpace(string(out)))
	}
	return wavPath, cleanup, nil
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
