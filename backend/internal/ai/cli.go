package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CLIProvider runs a controlled headless CLI (e.g. codebuddy -p) as an optional
// L2 backend. A mutex forces strictly serial execution (capacity 1) so a 1C1G
// VPS never spawns concurrent model processes, and every call is bounded by
// AI_TIMEOUT_SECONDS via exec.CommandContext.
type CLIProvider struct {
	command string
	args    []string
	timeout time.Duration
	mu      sync.Mutex
}

func NewCLIProvider(command string, args []string, timeout time.Duration) *CLIProvider {
	return &CLIProvider{command: command, args: args, timeout: timeout}
}

func (p *CLIProvider) Name() string { return "cli" }

func (p *CLIProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	prompt := systemPrompt + "\n\n" + userPrompt
	cmd := exec.CommandContext(ctx, p.command, resolveCLIArgs(p.args, prompt)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("cli timeout after %s", p.timeout)
	}
	if err != nil {
		return "", fmt.Errorf("cli failed: %w: %s", err, snippet(stderr.Bytes(), 200))
	}
	return unwrapCLIOutput(stdout.String()), nil
}

// resolveCLIArgs substitutes {{prompt}} in every arg; when no placeholder is
// present the full prompt is appended as the final argument.
func resolveCLIArgs(template []string, prompt string) []string {
	out := make([]string, 0, len(template)+1)
	found := false
	for _, a := range template {
		if strings.Contains(a, "{{prompt}}") {
			found = true
			out = append(out, strings.ReplaceAll(a, "{{prompt}}", prompt))
		} else {
			out = append(out, a)
		}
	}
	if !found {
		out = append(out, prompt)
	}
	return out
}

// unwrapCLIOutput extracts the model text from common headless CLI envelopes:
// a single JSON object with result/content/message/text, a stream of JSON lines
// (last meaningful line wins), or raw text (returned untouched for the shared
// JSON extractor). The triage object itself passes through unchanged.
func unwrapCLIOutput(out string) string {
	trimmed := strings.TrimSpace(out)
	if s, ok := wrapperText(trimmed); ok {
		return s
	}
	if s, ok := arrayWrapperText(trimmed); ok {
		return s
	}
	lines := strings.Split(trimmed, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if s, ok := wrapperText(strings.TrimSpace(lines[i])); ok {
			return s
		}
	}
	return trimmed
}

func arrayWrapperText(s string) (string, bool) {
	if !strings.HasPrefix(s, "[") {
		return "", false
	}
	var items []map[string]any
	if json.Unmarshal([]byte(s), &items) != nil {
		return "", false
	}
	for i := len(items) - 1; i >= 0; i-- {
		if role, _ := items[i]["role"].(string); strings.EqualFold(role, "user") {
			continue
		}
		if s, ok := extractMapText(items[i]); ok {
			return s, true
		}
	}
	return "", false
}

func wrapperText(s string) (string, bool) {
	if !strings.HasPrefix(s, "{") {
		return "", false
	}
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) != nil {
		return "", false
	}
	return extractMapText(m)
}

func extractMapText(m map[string]any) (string, bool) {
	for _, key := range []string{"result", "content", "message", "text"} {
		if v, ok := m[key]; ok {
			if s, ok := cliText(v); ok {
				return s, true
			}
		}
	}
	return "", false
}

func cliText(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, strings.TrimSpace(t) != ""
	case map[string]any:
		for _, key := range []string{"content", "text", "result", "message"} {
			if s, ok := cliText(t[key]); ok {
				return s, true
			}
		}
	case []any:
		var sb strings.Builder
		for _, item := range t {
			if s, ok := cliText(item); ok {
				sb.WriteString(s)
			}
		}
		if sb.Len() > 0 {
			return sb.String(), true
		}
	}
	return "", false
}