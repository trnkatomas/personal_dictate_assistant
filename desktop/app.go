package main

import (
	"context"
	"encoding/base64"
	"fmt"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Settings holds all user-configurable preferences.
type Settings struct {
	Mode            string `json:"mode"`            // "integrated" | "http"
	WhisperURL      string `json:"whisperUrl"`      // used when Mode == "http"
	WhisperLanguage string `json:"whisperLanguage"`
	WhisperTask     string `json:"whisperTask"`
	ModelName       string `json:"modelName"` // e.g. "large-v3-turbo", set by wizard

	RefinementEnabled bool   `json:"refinementEnabled"`
	RefinementURL     string `json:"refinementUrl"`    // OpenAI-compatible base URL, e.g. "http://localhost:11434/v1"
	RefinementModel   string `json:"refinementModel"`  // e.g. "qwen3:1.7b"
	RefinementPrompt  string `json:"refinementPrompt"` // editable system prompt sent to the refinement model

	UILanguage string `json:"uiLanguage"` // "auto" | "en" | "cs" — UI display language, not WhisperLanguage
}

// App is the Wails application struct. All exported methods are bound to the JS frontend.
type App struct {
	ctx            context.Context
	downloadCancel context.CancelFunc
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so we can call runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Transcribe decodes base64 audio and sends it to the configured Whisper backend.
// mimeType is the MediaRecorder mimeType (e.g. "audio/mp4") used to pick the temp filename extension.
func (a *App) Transcribe(audioBase64 string, mimeType string, settings Settings) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return "", fmt.Errorf("decode audio: %w", err)
	}
	return transcribeWhisper(raw, mimeType, settings)
}

// OpenAndTranscribeFile shows a native file picker, then transcribes the selected
// audio file directly from disk (no base64 round-trip). Returns empty string if
// the user cancelled the dialog.
func (a *App) OpenAndTranscribeFile(s Settings) (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select audio file",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "Audio files (*.mp3;*.mp4;*.m4a;*.wav;*.flac;*.ogg;*.webm;*.aac)",
				Pattern:     "*.mp3;*.mp4;*.m4a;*.wav;*.flac;*.ogg;*.webm;*.aac",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}
	return transcribeFileAt(path, s)
}

// TranscribeFileAtPath transcribes a file already on disk, given its absolute
// path. Used by the native OS drag-and-drop handler (see main.go's
// DragAndDrop.EnableFileDrop), which delivers real file paths — avoiding the
// base64-over-IPC transfer that Transcribe uses for in-browser recordings.
func (a *App) TranscribeFileAtPath(path string, s Settings) (string, error) {
	return transcribeFileAt(path, s)
}

// LoadSettings reads persisted settings from the on-disk config file,
// falling back to defaults if none exist yet.
func (a *App) LoadSettings() (Settings, error) {
	return loadSettings()
}

// SaveSettings persists settings to the on-disk config file.
func (a *App) SaveSettings(s Settings) error {
	return saveSettings(s)
}

// LogFilePath returns the path to the persistent app log file, so the
// frontend can show the user where to find it (or an empty string if file
// logging couldn't be set up on this machine — see setupLogging).
func (a *App) LogFilePath() string {
	return logFilePath
}
