package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

const logFileName = "app.log"

// logFilePath is set once by setupLogging and exposed to the frontend via
// App.LogFilePath so a user can be pointed at it without knowing their OS's
// config-directory convention.
var logFilePath string

// setupLogging redirects the standard logger to a persistent log file (in
// addition to stderr) so diagnostics survive on platforms where stderr is
// invisible. Wails builds a GUI-subsystem executable on Windows, with no
// console attached — log.Printf output otherwise has nowhere to go and is
// silently lost, which is why crashes and whisper-cli failures have shown up
// as "the app just doesn't do anything" reports with no error to act on.
//
// Returns the log file path (empty if file logging couldn't be set up, in
// which case logging still works to stderr only) and a close func to run on
// shutdown.
func setupLogging() (path string, closeFn func()) {
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("logging: could not determine config directory, file logging disabled: %v", err)
		return "", func() {}
	}
	logDir := filepath.Join(dir, settingsAppDirName, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Printf("logging: could not create log directory %s: %v", logDir, err)
		return "", func() {}
	}

	path = filepath.Join(logDir, logFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("logging: could not open log file %s: %v", path, err)
		return "", func() {}
	}

	log.SetOutput(io.MultiWriter(f, os.Stderr))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Printf("─── dictate-assistant starting — logging to %s ───", path)
	logFilePath = path
	return path, func() { f.Close() }
}
