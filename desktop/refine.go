package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Refine sends the transcript to an OpenAI-compatible chat completions endpoint
// (Ollama, llama-server, LM Studio, etc.) and returns the polished text.
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

	endpoint := strings.TrimRight(s.RefinementURL, "/") + "/chat/completions"

	payload, err := json.Marshal(map[string]any{
		"model": s.RefinementModel,
		"messages": []map[string]string{
			{
				"role": "system",
				"content": "You are a transcription editor. Fix punctuation, " +
					"capitalisation, and obvious speech-to-text artifacts. Keep the " +
					"original language, tone, and meaning unchanged. Return only the " +
					"corrected text — no explanations, no preamble.",
			},
			{
				"role":    "user",
				"content": text,
			},
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
