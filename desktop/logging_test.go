package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupLoggingCreatesFile(t *testing.T) {
	origPath := logFilePath
	t.Cleanup(func() {
		logFilePath = origPath
		log.SetOutput(os.Stderr) // setupLogging redirects the global logger; restore it
	})

	dir := t.TempDir()
	// os.UserConfigDir() reads a different env var per OS (XDG_CONFIG_HOME on
	// Linux, AppData on Windows, HOME on macOS) — set all three so the test
	// writes into the temp dir instead of the real, persistent config dir
	// regardless of which platform it runs on.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)

	path, closeFn := setupLogging()
	t.Cleanup(closeFn)

	if path == "" {
		t.Fatal("expected a non-empty log file path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file was not created at %s: %v", path, err)
	}
	if filepath.Base(path) != logFileName {
		t.Errorf("expected log file named %s, got %s", logFileName, path)
	}
	if logFilePath != path {
		t.Errorf("logFilePath global should be set to %s, got %s", path, logFilePath)
	}
}
