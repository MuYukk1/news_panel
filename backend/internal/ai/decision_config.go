package ai

import (
	"log/slog"
	"math"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var headerNamePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9a-zA-Z-]+$")

type DecisionConfig struct {
	Enabled                                      bool
	Provider, BaseURL, APIKey, AuthHeader, Model string
	JSONFormat                                   bool
	Timeout                                      time.Duration
	L3Priorities                                 []string
	LowAction, FallbackMode                      string
}

func decisionConfigFromEnv(log *slog.Logger) DecisionConfig {
	c := DecisionConfig{
		Provider:     strings.ToLower(envDefault("AI_DECISION_PROVIDER", "openai")),
		BaseURL:      strings.TrimSpace(os.Getenv("AI_DECISION_BASE_URL")),
		APIKey:       strings.TrimSpace(os.Getenv("AI_DECISION_API_KEY")),
		AuthHeader:   envDefault("AI_DECISION_AUTH_HEADER", "Authorization"),
		Model:        envDefault("AI_DECISION_MODEL", defaultModel),
		Timeout:      6 * time.Second,
		LowAction:    envChoice(log, "AI_DECISION_LOW_ACTION", "keep_unscored", "filter"),
		FallbackMode: envChoice(log, "AI_DECISION_FALLBACK_MODE", "skip_to_l3", "keep_l1", "fail"),
	}
	c.JSONFormat = decisionBool(log, "AI_DECISION_JSON_FORMAT", false)
	if raw := strings.TrimSpace(os.Getenv("AI_DECISION_TIMEOUT_SECONDS")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 || n > math.MaxInt64/int64(time.Second) {
			log.Warn("invalid AI_DECISION_TIMEOUT_SECONDS; using 6")
		} else {
			c.Timeout = time.Duration(n) * time.Second
		}
	}
	c.L3Priorities = strings.Split(strings.ToLower(envDefault("AI_DECISION_L3_PRIORITIES", "critical,high,normal")), ",")
	for i, p := range c.L3Priorities {
		p = strings.TrimSpace(p)
		if !validPriority(p) {
			log.Warn("invalid AI_DECISION_L3_PRIORITIES; using critical,high,normal")
			c.L3Priorities = []string{"critical", "high", "normal"}
			break
		}
		c.L3Priorities[i] = p
	}
	switch c.Provider {
	case "openai":
		if c.BaseURL == "" {
			c.BaseURL = defaultBaseURL
		}
	case "http_json":
		if !headerNamePattern.MatchString(c.AuthHeader) {
			log.Warn("invalid AI_DECISION_AUTH_HEADER; using Authorization")
			c.AuthHeader = "Authorization"
		}
	case "off", "none":
		c.Provider = "off"
		return c
	default:
		log.Warn("unknown AI_DECISION_PROVIDER; disabling L2")
		c.Provider = "off"
		return c
	}
	enabled := decisionBool(log, "AI_DECISION_ENABLED", true)
	if c.BaseURL == "" || (c.Provider == "openai" && c.APIKey == "") {
		return c
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		log.Warn("invalid AI_DECISION_BASE_URL; disabling L2")
		return c
	}
	c.Enabled = enabled
	return c
}

func validPriority(p string) bool {
	switch p {
	case "critical", "high", "normal", "low":
		return true
	}
	return false
}

func envChoice(log *slog.Logger, key, def string, others ...string) string {
	v := strings.ToLower(envDefault(key, def))
	if v == def || slices.Contains(others, v) {
		return v
	}
	log.Warn("invalid AI option; using default", "option", key, "default", def)
	return def
}

func decisionBool(log *slog.Logger, key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	if v, err := strconv.ParseBool(raw); err == nil {
		return v
	}
	log.Warn("invalid AI option; using default", "option", key)
	return def
}

// RSSMaxItemsPerPull reads the per-source evaluation budget once at puller startup.
func RSSMaxItemsPerPull() int {
	n, err := strconv.Atoi(envDefault("AI_RSS_MAX_ITEMS_PER_PULL", "20"))
	if err != nil || n < 0 {
		slog.Warn("invalid AI_RSS_MAX_ITEMS_PER_PULL; using 20")
		return 20
	}
	return n
}
