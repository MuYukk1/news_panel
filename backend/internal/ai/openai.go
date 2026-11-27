package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider talks to any OpenAI-compatible POST /chat/completions endpoint
// using a single shared *http.Client (connection pool reused across items).
type OpenAIProvider struct {
	endpoint   string
	apiKey     string
	model      string
	jsonFormat bool
	client     *http.Client
}

func NewOpenAIProvider(baseURL, apiKey, model string, jsonFormat bool, timeout time.Duration) *OpenAIProvider {
	return &OpenAIProvider{
		endpoint:   chatCompletionsURL(baseURL),
		apiKey:     apiKey,
		model:      model,
		jsonFormat: jsonFormat,
		client:     &http.Client{Timeout: timeout},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

// chatCompletionsURL normalizes AI_BASE_URL: an explicit /chat/completions
// suffix is used as-is, a /v1 suffix gets /chat/completions, anything else gets
// /v1/chat/completions.
func chatCompletionsURL(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if b == "" {
		b = defaultBaseURL
	}
	switch {
	case strings.HasSuffix(b, "/chat/completions"):
		return b
	case strings.HasSuffix(b, "/v1"):
		return b + "/chat/completions"
	default:
		return b + "/v1/chat/completions"
	}
}

func (p *OpenAIProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	if p.jsonFormat {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openai encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("openai: invalid request URL")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", context.DeadlineExceeded
		}
		return "", errors.New("openai request failed")
	}
	defer resp.Body.Close()
	respBody, err := readModelResponse(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openai read response: %s", redact(err.Error(), p.apiKey))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai status %d: [REDACTED]", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("openai decode response: %w", err)
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", errors.New("openai response has empty choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func snippet(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
