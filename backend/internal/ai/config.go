// Package ai implements local filtering, optional decision gating and model scoring.
package ai

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL             = "https://api.openai.com/v1"
	defaultModel               = "gpt-4o-mini"
	defaultTopics              = "投资催化剂与宏观突发,AI前沿动态与内测泄露,高时效薅羊毛与云资源优惠"
	defaultKeepMinScore        = 45
	defaultTimeoutSeconds      = 8
	defaultWriteTimeoutSeconds = 120
	defaultL1MinContentRunes   = 10
)

// Config holds every AI_* knob with safe defaults. APIKey is secret: it must
// never appear in logs or returned errors.
type Config struct {
	Decision          DecisionConfig
	Enabled           bool
	Provider          string // openai | cli | l1
	BaseURL           string
	APIKey            string
	Model             string
	Topics            string
	KeepMinScore      int
	Timeout           time.Duration
	FallbackMode      string // keep_l1 | fail
	OpenAIJSONFormat  bool
	L1MinContentRunes int
	L1FilterRetweets  bool
	L1BlockKeywords   []string
	L1AllowKeywords   []string
	CLICommand        string
	CLIArgs           []string
	CLICooldown       time.Duration
}

// ConfigFromEnv reads AI_* variables from the process environment only.
// When AI_ENABLED is unset, the pipeline auto-enables if AI_API_KEY is present
// or AI_PROVIDER=cli; otherwise it degrades to L1-only. Invalid values fall
// back to safe defaults with a warning (never fatal, never blocks startup).
func ConfigFromEnv(logger *slog.Logger) Config {
	if logger == nil {
		logger = slog.Default()
	}
	cfg := Config{
		Provider:          strings.ToLower(envDefault("AI_PROVIDER", "openai")),
		BaseURL:           envDefault("AI_BASE_URL", defaultBaseURL),
		APIKey:            strings.TrimSpace(os.Getenv("AI_API_KEY")),
		Model:             envDefault("AI_MODEL", defaultModel),
		Topics:            envDefault("AI_TOPICS", defaultTopics),
		KeepMinScore:      defaultKeepMinScore,
		Timeout:           defaultTimeoutSeconds * time.Second,
		FallbackMode:      strings.ToLower(envDefault("AI_FALLBACK_MODE", "keep_l1")),
		OpenAIJSONFormat:  envBool("AI_OPENAI_JSON_FORMAT", false),
		L1MinContentRunes: defaultL1MinContentRunes,
		L1FilterRetweets:  envBool("AI_L1_FILTER_RETWEETS", false),
		L1BlockKeywords:   splitKeywords(os.Getenv("AI_L1_BLOCK_KEYWORDS")),
		L1AllowKeywords:   splitKeywords(os.Getenv("AI_L1_ALLOW_KEYWORDS")),
		CLICommand:        strings.TrimSpace(os.Getenv("AI_CLI_COMMAND")),
		CLIArgs:           splitCLIArgs(os.Getenv("AI_CLI_ARGS")),
	}

	switch cfg.Provider {
	case "openai", "cli", "l1":
	default:
		logger.Warn("unknown AI_PROVIDER; degrading to L1-only", "provider", cfg.Provider)
		cfg.Provider = "l1"
	}

	auto := cfg.APIKey != "" || cfg.Provider == "cli"
	cfg.Enabled = auto
	if raw := strings.TrimSpace(os.Getenv("AI_ENABLED")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			cfg.Enabled = parsed
		} else {
			logger.Warn("invalid AI_ENABLED; using auto-detect", "value", raw)
		}
	}

	if raw := strings.TrimSpace(os.Getenv("AI_KEEP_MIN_SCORE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			cfg.KeepMinScore = clampInt(n, 0, 100)
		} else {
			logger.Warn("invalid AI_KEEP_MIN_SCORE; using default", "value", raw, "default", defaultKeepMinScore)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AI_TIMEOUT_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			cfg.Timeout = time.Duration(n) * time.Second
		} else {
			logger.Warn("invalid AI_TIMEOUT_SECONDS; using default", "value", raw, "default", defaultTimeoutSeconds)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AI_CLI_COOLDOWN_SECONDS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			cfg.CLICooldown = time.Duration(n) * time.Second
		} else {
			logger.Warn("invalid AI_CLI_COOLDOWN_SECONDS; using 0", "value", raw)
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AI_L1_MIN_CONTENT_RUNES")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			cfg.L1MinContentRunes = n
		} else {
			logger.Warn("invalid AI_L1_MIN_CONTENT_RUNES; using default", "value", raw, "default", defaultL1MinContentRunes)
		}
	}
	switch cfg.FallbackMode {
	case "keep_l1", "fail":
	default:
		logger.Warn("unknown AI_FALLBACK_MODE; using keep_l1", "value", cfg.FallbackMode)
		cfg.FallbackMode = "keep_l1"
	}
	cfg.Decision = decisionConfigFromEnv(logger)
	return cfg
}

// HTTPWriteTimeout is read by cmd/fusion to widen http.Server.WriteTimeout so a
// synchronous multi-item ingest batch is not cut off by the 30s default.
func HTTPWriteTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AI_HTTP_WRITE_TIMEOUT_SECONDS"))
	if raw == "" {
		return defaultWriteTimeoutSeconds * time.Second
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return defaultWriteTimeoutSeconds * time.Second
}

func envDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	if parsed, err := strconv.ParseBool(raw); err == nil {
		return parsed
	}
	return def
}

func splitKeywords(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '，' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// splitCLIArgs accepts a comma-separated list, or whitespace-separated tokens
// when no comma is present. {{prompt}} placeholders are resolved at call time.
func splitCLIArgs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parts []string
	if strings.Contains(raw, ",") {
		parts = strings.Split(raw, ",")
	} else {
		parts = strings.Fields(raw)
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == `""` || p == `''` {
			out = append(out, "")
			continue
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// redact removes secret material from any string destined for logs or errors.
func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "[REDACTED]")
}

// When Authorization includes a scheme, a provider may echo the token alone.
func redactCredential(s, credential string) string {
	s = redact(s, credential)
	if _, token, ok := strings.Cut(credential, " "); ok {
		return redact(s, strings.TrimSpace(token))
	}
	return s
}
