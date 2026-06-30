package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const settingsAppDirName = "dictate-assistant"
const settingsFileName = "settings.json"

const defaultRefinementPrompt = "Fix punctuation, capitalization, spelling, and obvious " +
	"speech-to-text mistakes. Keep the original language, tone, and meaning exactly as they " +
	"are. Only change words you're confident are wrong — leave everything else untouched, " +
	"and make sure corrections fit naturally with the surrounding sentence. Return only the " +
	"corrected text, with no explanations or preamble."

func defaultSettings() Settings {
	return Settings{
		Mode:              "integrated",
		WhisperURL:        "http://localhost:9000",
		WhisperLanguage:   "",
		WhisperTask:       "transcribe",
		ModelName:         "",
		RefinementEnabled: false,
		RefinementURL:     "http://localhost:11434/v1",
		RefinementModel:   "qwen3:1.7b",
		RefinementPrompt:  defaultRefinementPrompt,
	}
}

// settingsPath resolves the platform-appropriate config file path:
// ~/Library/Application Support/dictate-assistant/settings.json on macOS,
// %AppData%/dictate-assistant/settings.json on Windows,
// $XDG_CONFIG_HOME/dictate-assistant/settings.json (or ~/.config/...) on Linux.
func settingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, settingsAppDirName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, settingsFileName), nil
}

func loadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return defaultSettings(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultSettings(), nil
		}
		return defaultSettings(), err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings(), err
	}
	// Migrate settings written before the Mode field existed: those users were
	// using HTTP mode, so treat missing mode as "http" rather than "integrated".
	if s.Mode == "" {
		s.Mode = "http"
	}
	// Migrate settings written before refinementPrompt existed.
	if s.RefinementPrompt == "" {
		s.RefinementPrompt = defaultRefinementPrompt
	}
	return s, nil
}

func saveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
