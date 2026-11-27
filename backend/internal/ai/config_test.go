package ai

import (
	"log/slog"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&nullWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }

func clearAIEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AI_DECISION_ACCOUNT_ID", "AI_ALLOW_PRIVATE_ENDPOINTS", "AI_DECISION_PROVIDER", "AI_DECISION_ENABLED", "AI_DECISION_BASE_URL", "AI_DECISION_API_KEY", "AI_DECISION_AUTH_HEADER", "AI_DECISION_MODEL", "AI_DECISION_TIMEOUT_SECONDS", "AI_DECISION_L3_PRIORITIES", "AI_DECISION_LOW_ACTION", "AI_DECISION_JSON_FORMAT", "AI_DECISION_FALLBACK_MODE", "AI_RSS_MAX_ITEMS_PER_PULL",
		"AI_ENABLED", "AI_PROVIDER", "AI_BASE_URL", "AI_API_KEY", "AI_MODEL", "AI_TOPICS",
		"AI_KEEP_MIN_SCORE", "AI_TIMEOUT_SECONDS", "AI_FALLBACK_MODE", "AI_OPENAI_JSON_FORMAT",
		"AI_L1_MIN_CONTENT_RUNES", "AI_L1_FILTER_RETWEETS", "AI_L1_BLOCK_KEYWORDS",
		"AI_L1_ALLOW_KEYWORDS", "AI_CLI_COMMAND", "AI_CLI_ARGS", "AI_CLI_COOLDOWN_SECONDS", "AI_HTTP_WRITE_TIMEOUT_SECONDS",
	} {
		t.Setenv(k, "")
	}
}

func TestConfigDefaults(t *testing.T) {
	clearAIEnv(t)
	cfg := ConfigFromEnv(discardLogger())
	if cfg.Enabled {
		t.Errorf("expected auto L1-only (disabled) with empty env, got Enabled=true")
	}
	if cfg.Provider != "openai" || cfg.BaseURL != defaultBaseURL || cfg.Model != defaultModel {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.KeepMinScore != 45 || cfg.Timeout != 8*time.Second || cfg.FallbackMode != "keep_l1" {
		t.Errorf("unexpected numeric/fallback defaults: %+v", cfg)
	}
	if cfg.OpenAIJSONFormat || cfg.L1FilterRetweets || cfg.L1MinContentRunes != 10 {
		t.Errorf("unexpected L1 defaults: %+v", cfg)
	}
	if cfg.Topics != defaultTopics {
		t.Errorf("unexpected topics default: %q", cfg.Topics)
	}
}

func TestConfigAutoEnableRules(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		enabled bool
	}{
		{"empty env -> L1-only", map[string]string{}, false},
		{"key present -> enabled", map[string]string{"AI_API_KEY": "sk-test"}, true},
		{"provider cli -> enabled", map[string]string{"AI_PROVIDER": "cli"}, true},
		{"provider l1 no key -> disabled", map[string]string{"AI_PROVIDER": "l1"}, false},
		{"explicit false overrides key", map[string]string{"AI_API_KEY": "sk-test", "AI_ENABLED": "false"}, false},
		{"explicit true no key", map[string]string{"AI_ENABLED": "true"}, true},
		{"invalid enabled falls back to auto (key)", map[string]string{"AI_API_KEY": "sk-test", "AI_ENABLED": "maybe"}, true},
		{"invalid enabled falls back to auto (empty)", map[string]string{"AI_ENABLED": "maybe"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAIEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg := ConfigFromEnv(discardLogger())
			if cfg.Enabled != tc.enabled {
				t.Errorf("Enabled=%v, want %v", cfg.Enabled, tc.enabled)
			}
		})
	}
}

func TestConfigParsingAndClamping(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("AI_KEEP_MIN_SCORE", "150")
	t.Setenv("AI_TIMEOUT_SECONDS", "20")
	t.Setenv("AI_L1_MIN_CONTENT_RUNES", "0")
	t.Setenv("AI_FALLBACK_MODE", "FAIL")
	t.Setenv("AI_OPENAI_JSON_FORMAT", "true")
	t.Setenv("AI_L1_FILTER_RETWEETS", "1")
	t.Setenv("AI_L1_BLOCK_KEYWORDS", " spam ，广告, , ")
	t.Setenv("AI_L1_ALLOW_KEYWORDS", "薅羊毛,限免")
	t.Setenv("AI_PROVIDER", "OPENAI")
	cfg := ConfigFromEnv(discardLogger())
	if cfg.KeepMinScore != 100 {
		t.Errorf("KeepMinScore clamp: got %d, want 100", cfg.KeepMinScore)
	}
	if cfg.Timeout != 20*time.Second {
		t.Errorf("Timeout: got %v", cfg.Timeout)
	}
	if cfg.L1MinContentRunes != 0 {
		t.Errorf("L1MinContentRunes: got %d, want 0", cfg.L1MinContentRunes)
	}
	if cfg.FallbackMode != "fail" {
		t.Errorf("FallbackMode normalize: got %q", cfg.FallbackMode)
	}
	if !cfg.OpenAIJSONFormat || !cfg.L1FilterRetweets {
		t.Errorf("bool parsing failed: %+v", cfg)
	}
	if len(cfg.L1BlockKeywords) != 2 || cfg.L1BlockKeywords[0] != "spam" || cfg.L1BlockKeywords[1] != "广告" {
		t.Errorf("block keywords split: %#v", cfg.L1BlockKeywords)
	}
	if len(cfg.L1AllowKeywords) != 2 {
		t.Errorf("allow keywords split: %#v", cfg.L1AllowKeywords)
	}
	if cfg.Provider != "openai" {
		t.Errorf("provider normalize: got %q", cfg.Provider)
	}
}

func TestConfigInvalidValuesUseDefaults(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("AI_KEEP_MIN_SCORE", "abc")
	t.Setenv("AI_TIMEOUT_SECONDS", "-3")
	t.Setenv("AI_L1_MIN_CONTENT_RUNES", "-1")
	t.Setenv("AI_FALLBACK_MODE", "explode")
	t.Setenv("AI_PROVIDER", "gemini-native")
	cfg := ConfigFromEnv(discardLogger())
	if cfg.KeepMinScore != 45 || cfg.Timeout != 8*time.Second || cfg.L1MinContentRunes != 10 {
		t.Errorf("invalid values should use defaults: %+v", cfg)
	}
	if cfg.FallbackMode != "keep_l1" {
		t.Errorf("unknown fallback should default keep_l1: %q", cfg.FallbackMode)
	}
	if cfg.Provider != "l1" {
		t.Errorf("unknown provider should degrade to l1: %q", cfg.Provider)
	}
}

func TestSplitCLIArgs(t *testing.T) {
	got := splitCLIArgs("-p,{{prompt}},--output-format,json")
	want := []string{"-p", "{{prompt}}", "--output-format", "json"}
	if len(got) != len(want) {
		t.Fatalf("comma split: %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("comma split[%d]=%q want %q", i, got[i], want[i])
		}
	}
	got = splitCLIArgs("-p {{prompt}} --output-format json")
	if len(got) != 4 || got[1] != "{{prompt}}" {
		t.Errorf("whitespace split: %#v", got)
	}
	got = splitCLIArgs(`-p,{{prompt}},--tools,"",--model,fast-model`)
	if len(got) != 6 || got[2] != "--tools" || got[3] != "" || got[5] != "fast-model" {
		t.Errorf("explicit empty string split failed: %#v", got)
	}
	if splitCLIArgs("   ") != nil {
		t.Errorf("empty args should be nil")
	}
}

func TestHTTPWriteTimeout(t *testing.T) {
	clearAIEnv(t)
	if got := HTTPWriteTimeout(); got != 120*time.Second {
		t.Errorf("default write timeout: got %v, want 120s", got)
	}
	t.Setenv("AI_HTTP_WRITE_TIMEOUT_SECONDS", "45")
	if got := HTTPWriteTimeout(); got != 45*time.Second {
		t.Errorf("custom write timeout: got %v", got)
	}
	t.Setenv("AI_HTTP_WRITE_TIMEOUT_SECONDS", "not-a-number")
	if got := HTTPWriteTimeout(); got != 120*time.Second {
		t.Errorf("invalid write timeout should default: got %v", got)
	}
	t.Setenv("AI_HTTP_WRITE_TIMEOUT_SECONDS", "0")
	if got := HTTPWriteTimeout(); got != 120*time.Second {
		t.Errorf("zero write timeout should default: got %v", got)
	}
}

func TestCLICooldownParsing(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("AI_CLI_COOLDOWN_SECONDS", "15")
	cfg := ConfigFromEnv(discardLogger())
	if cfg.CLICooldown != 15*time.Second {
		t.Errorf("cooldown=%v want 15s", cfg.CLICooldown)
	}
	t.Setenv("AI_CLI_COOLDOWN_SECONDS", "-5")
	cfg = ConfigFromEnv(discardLogger())
	if cfg.CLICooldown != 0 {
		t.Errorf("negative cooldown should clamp/ignore to 0, got %v", cfg.CLICooldown)
	}
}

func TestRedact(t *testing.T) {
	if got := redact("error with sk-secret-123 inside", "sk-secret-123"); got != "error with [REDACTED] inside" {
		t.Errorf("redact: %q", got)
	}
	if got := redact("no secret", ""); got != "no secret" {
		t.Errorf("redact empty secret: %q", got)
	}
}
