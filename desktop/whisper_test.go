package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlainFailureMessagePreservesWhisperCliText(t *testing.T) {
	// A real whisper-cli error (e.g. an unsupported input format) must come
	// through verbatim — this is the case that was previously getting
	// overwritten by the unrelated "same crash on every CPU build" message.
	got := plainFailureMessage(1, "error: failed to read WAV file, unsupported format")
	want := "error: failed to read WAV file, unsupported format"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPlainFailureMessageHandlesEmptyStderr(t *testing.T) {
	got := plainFailureMessage(2, "")
	if got == "" {
		t.Error("must not return an empty message even with no stderr")
	}
	if !strings.Contains(got, "2") {
		t.Errorf("message should mention the exit code: %q", got)
	}
}

// withTempConfigDir points os.UserConfigDir() at a fresh temp directory for
// the duration of the test, regardless of platform (see setupLogging's own
// test for why all three env vars are needed).
func withTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
}

func TestSaveFailedClipMovesFileOutOfTemp(t *testing.T) {
	withTempConfigDir(t)

	src := filepath.Join(t.TempDir(), "dictate-audio-123.webm")
	if err := os.WriteFile(src, []byte("fake audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	saveFailedClip(src)

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("original temp file should no longer exist")
	}

	dir, err := failedClipsDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 saved clip, got %d", len(entries))
	}
	if !strings.HasSuffix(entries[0].Name(), "dictate-audio-123.webm") {
		t.Errorf("saved clip name = %q, want it to preserve the original basename", entries[0].Name())
	}
}

func TestSaveFailedClipIgnoresEmptyOrMissingPath(t *testing.T) {
	withTempConfigDir(t)

	saveFailedClip("") // must not panic or create anything
	saveFailedClip(filepath.Join(t.TempDir(), "does-not-exist.wav"))

	dir, err := failedClipsDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no saved clips, got %d", len(entries))
	}
}

func TestPruneFailedClipsKeepsOnlyMostRecent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for i := 0; i < maxFailedClips+5; i++ {
		name := filepath.Join(dir, "clip-"+string(rune('a'+i)))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Spread mtimes so "most recent" is well-defined regardless of
		// filesystem timestamp resolution.
		modTime := now.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(name, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	pruneFailedClips(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != maxFailedClips {
		t.Fatalf("got %d files after pruning, want %d", len(entries), maxFailedClips)
	}
	// The oldest 5 (a..e) should be gone; the newest maxFailedClips should remain.
	for _, e := range entries {
		if e.Name() <= "clip-e" {
			t.Errorf("expected %s to have been pruned as one of the oldest", e.Name())
		}
	}
}
