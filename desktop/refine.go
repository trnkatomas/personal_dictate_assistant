package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// defaultContextTokens is used when the backend's context window can't be
// detected — matches Ollama's own default num_ctx, so it's a safe floor
// rather than an optimistic guess.
const defaultContextTokens = 4096

// charsPerToken is a conservative estimate. English averages ~4 chars/token,
// but diacritic-heavy languages (Czech, etc.) tokenize less efficiently, so
// a lower ratio keeps chunks safely under budget rather than over it.
const charsPerToken = 3.0

// RefineProgress is emitted as a Wails event while a multi-chunk refinement
// is in progress. Single-chunk refinements (the common case) emit nothing.
type RefineProgress struct {
	Current int  `json:"current"`
	Total   int  `json:"total"`
	Done    bool `json:"done"`
}

// Refine sends the transcript to an OpenAI-compatible chat completions endpoint
// (Ollama, llama-server, LM Studio, etc.) and returns the polished text. Text
// too long for the model's context window is split into sentence-aligned
// chunks, refined sequentially, and concatenated back together.
func (a *App) Refine(text string, s Settings) (string, error) {
	if !s.RefinementEnabled {
		return text, nil
	}
	if s.RefinementURL == "" {
		return "", fmt.Errorf("refinement URL is not configured")
	}
	if s.RefinementModel == "" {
		return "", fmt.Errorf("refinement model is not configured")
	}

	prompt := s.RefinementPrompt
	if prompt == "" {
		prompt = defaultRefinementPrompt
	}

	ctxTokens := detectContextWindow(s.RefinementURL, s.RefinementModel)
	budget := refinementCharBudget(ctxTokens, prompt)
	chunks := chunkText(text, budget)

	if len(chunks) == 1 {
		return refineChunk(prompt, chunks[0], s)
	}

	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		wailsRuntime.EventsEmit(a.ctx, "refine:progress", RefineProgress{
			Current: i + 1,
			Total:   len(chunks),
		})
		refined, err := refineChunk(prompt, chunk, s)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "refine:progress", RefineProgress{Done: true})
			return "", fmt.Errorf("chunk %d/%d: %w", i+1, len(chunks), err)
		}
		out = append(out, refined)
	}
	wailsRuntime.EventsEmit(a.ctx, "refine:progress", RefineProgress{Done: true})
	return strings.Join(out, " "), nil
}

// refineChunk sends a single chunk of text through the refinement endpoint.
func refineChunk(prompt, text string, s Settings) (string, error) {
	endpoint := strings.TrimRight(s.RefinementURL, "/") + "/chat/completions"

	payload, err := json.Marshal(map[string]any{
		"model": s.RefinementModel,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": text},
		},
		"stream": false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refinement request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("refinement API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode refinement response: %w", err)
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from refinement model")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ── Context window detection ──────────────────────────────────────────────

// introspectClient has a short timeout so a hung/unreachable backend doesn't
// stall the whole refinement — detection failures just fall back to the
// conservative default.
var introspectClient = &http.Client{Timeout: 3 * time.Second}

// detectContextWindow best-effort probes known local-server introspection
// endpoints. The OpenAI-compatible /chat/completions surface itself has no
// standard way to report context size, so this is backend-specific.
func detectContextWindow(baseURL, model string) int {
	if n := detectOllamaContext(baseURL, model); n > 0 {
		return n
	}
	if n := detectLlamaServerContext(baseURL); n > 0 {
		return n
	}
	return defaultContextTokens
}

// nativeBaseURL strips a trailing "/v1" from an OpenAI-compatible base URL
// to reach a backend's native (non-OpenAI) API root, e.g.
// "http://localhost:11434/v1" -> "http://localhost:11434".
func nativeBaseURL(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, "/v1")
	return strings.TrimRight(u, "/")
}

// detectOllamaContext queries Ollama's /api/show for the configured model.
// Ollama runs with num_ctx=2048 by default regardless of what the model
// architecture actually supports, and silently truncates rather than
// erroring if that's exceeded — so this prefers the explicitly configured
// num_ctx (from parameters) and falls back to Ollama's own default, never
// to the architecture's larger theoretical max, which would risk the exact
// truncation this feature exists to avoid.
func detectOllamaContext(baseURL, model string) int {
	payload, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return 0
	}
	req, err := http.NewRequest(http.MethodPost, nativeBaseURL(baseURL)+"/api/show", bytes.NewReader(payload))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := introspectClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}

	var out struct {
		Parameters string `json:"parameters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0
	}
	for _, line := range strings.Split(out.Parameters, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "num_ctx" {
			if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	// This IS a real Ollama response (200 + parseable), so we know Ollama's
	// own default applies rather than falling through to defaultContextTokens
	// (which is the same value today, but this stays correct if that changes).
	return 2048
}

// detectLlamaServerContext queries llama-server's /props endpoint, which
// reports the context size it was actually launched with (-c flag) — a hard
// server-wide cap, unlike Ollama's per-model default.
func detectLlamaServerContext(baseURL string) int {
	req, err := http.NewRequest(http.MethodGet, nativeBaseURL(baseURL)+"/props", nil)
	if err != nil {
		return 0
	}
	resp, err := introspectClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}

	var out struct {
		DefaultGenerationSettings struct {
			NCtx int `json:"n_ctx"`
		} `json:"default_generation_settings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0
	}
	return out.DefaultGenerationSettings.NCtx
}

// ── Chunking ─────────────────────────────────────────────────────────────

// refinementCharBudget converts a token-based context window into a
// character budget for a single input chunk. Local chat APIs share one
// context window across prompt + completion, and a light-touch refinement
// pass produces output roughly as long as its input, so only about half the
// window (after subtracting the system prompt) is usable for input text.
func refinementCharBudget(ctxTokens int, systemPrompt string) int {
	promptTokens := int(float64(len(systemPrompt))/charsPerToken) + 20 // +chat-template overhead
	usableTokens := (ctxTokens - promptTokens) / 2
	if usableTokens < 200 {
		usableTokens = 200 // floor so we always make forward progress
	}
	return int(float64(usableTokens) * charsPerToken)
}

var sentenceEndRe = regexp.MustCompile(`([.!?]+)(\s+)`)

// splitSentences splits on sentence-ending punctuation followed by
// whitespace, keeping the punctuation attached to the preceding sentence.
// A lightweight heuristic (not a full NLP segmenter) — good enough for
// chunking purposes; occasionally splitting after an abbreviation just
// costs one sentence-worth of shared context, not correctness.
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	lastEnd := 0
	for _, m := range sentenceEndRe.FindAllStringSubmatchIndex(text, -1) {
		sentences = append(sentences, strings.TrimSpace(text[lastEnd:m[3]]))
		lastEnd = m[1]
	}
	if lastEnd < len(text) {
		sentences = append(sentences, strings.TrimSpace(text[lastEnd:]))
	}
	out := sentences[:0]
	for _, s := range sentences {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// packWords greedily packs whitespace-separated words into strings no
// longer than budget. Fallback for a single "sentence" that alone exceeds
// the chunk budget — common in raw dictation, which often lacks punctuation
// entirely over long stretches.
func packWords(s string, budget int) []string {
	var out []string
	var cur strings.Builder
	for _, w := range strings.Fields(s) {
		if cur.Len() > 0 && cur.Len()+1+len(w) > budget {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// chunkText splits text into pieces that each fit within charBudget,
// preferring sentence boundaries and never splitting mid-word. Respects
// whatever budget it's given — the floor for "too small to be useful"
// belongs in refinementCharBudget, which has the token-cost context to pick
// a sensible one; overriding it here would silently violate that budget.
func chunkText(text string, charBudget int) []string {
	var units []string
	for _, sent := range splitSentences(text) {
		if len(sent) <= charBudget {
			units = append(units, sent)
			continue
		}
		units = append(units, packWords(sent, charBudget)...)
	}

	var chunks []string
	var cur strings.Builder
	for _, u := range units {
		if cur.Len() > 0 && cur.Len()+1+len(u) > charBudget {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(u)
	}
	if cur.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(cur.String()))
	}
	if len(chunks) == 0 {
		return []string{text}
	}
	return chunks
}
