package main

import (
	"strings"
	"testing"
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
