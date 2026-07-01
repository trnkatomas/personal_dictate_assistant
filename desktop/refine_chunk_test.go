package main

import (
	"strings"
	"testing"
)

func TestChunkTextRoundTrip(t *testing.T) {
	text := "Zaklínač. Část první. Čeješ si? Pivo, odpověděl příchozí. Neměl zrovna nejpříjemnější hlas. " +
		"Krčmář si otřel ruce o zástěru a naplnil otřískaný hliněný korbel. Neznámý nebyl starý, avšak " +
		"vlasy uměl už docela bílé. Pod pláštěm měl odřenou koženou kazajku se šněrovadli pod krkem a na pažích."

	chunks := chunkText(text, 80)
	t.Logf("split into %d chunks with budget=80:", len(chunks))
	for i, c := range chunks {
		t.Logf("  [%d] (%d bytes) %q", i, len(c), c)
		if len(c) > 80+50 { // sentence-length overflow tolerance, not word-packing
			t.Errorf("chunk %d is %d bytes, way over budget 80: %q", i, len(c), c)
		}
	}

	rejoined := strings.Join(chunks, " ")
	origWords := strings.Fields(text)
	gotWords := strings.Fields(rejoined)
	if len(origWords) != len(gotWords) {
		t.Errorf("word count mismatch: original=%d rejoined=%d", len(origWords), len(gotWords))
	}
	for i := range origWords {
		if i >= len(gotWords) || origWords[i] != gotWords[i] {
			t.Fatalf("word mismatch at %d: want %q got %q", i, origWords[i], safeIndex(gotWords, i))
		}
	}
}

func safeIndex(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}

func TestChunkTextSingleChunkWhenSmall(t *testing.T) {
	text := "Short sentence. Another one."
	chunks := chunkText(text, 4096)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != text {
		t.Errorf("expected chunk to equal original text, got %q", chunks[0])
	}
}

func TestChunkTextOversizedSentenceFallsBackToWords(t *testing.T) {
	// No punctuation at all — simulates unpunctuated rambling in raw dictation.
	text := strings.Repeat("word ", 100)
	text = strings.TrimSpace(text)
	chunks := chunkText(text, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for oversized unpunctuated text, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 50 {
			t.Errorf("chunk %d is %d bytes, over budget 50: %q", i, len(c), c)
		}
	}
	rejoined := strings.Join(chunks, " ")
	if strings.Fields(rejoined)[0] != "word" || len(strings.Fields(rejoined)) != 100 {
		t.Errorf("word packing lost/altered content: got %d words", len(strings.Fields(rejoined)))
	}
}

func TestRefinementCharBudgetLeavesRoomForOutput(t *testing.T) {
	budget := refinementCharBudget(4096, "short system prompt")
	// Budget should be well under the full context, leaving room for the
	// system prompt and the model's own output.
	if budget > 4096*3 { // *3 to convert token ctx to a rough char ceiling
		t.Errorf("budget %d seems too large relative to 4096-token context", budget)
	}
	if budget < 200 {
		t.Errorf("budget %d is suspiciously small", budget)
	}
	t.Logf("4096-token context -> %d char budget per chunk", budget)
}
