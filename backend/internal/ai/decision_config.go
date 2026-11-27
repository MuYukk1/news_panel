package ai

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/url"
	"news_panel/internal/pkg/httpc"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTypeSafeBaseURL = "https://api.typesafe.ai/v1/systemone"
	defaultTypeSafeModel   = "jev-latest"
)

var headerNamePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9a-zA-Z-]+$")

type DecisionConfig struct {
	AccountID                                    string
	AllowPrivateEndpoints                        bool
	requested                                    bool
	envBaseURL, envModel                         string
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
	c.AccountID = strings.TrimSpace(os.Getenv("AI_DECISION_ACCOUNT_ID"))
	c.AllowPrivateEndpoints = decisionBool(log, "AI_ALLOW_PRIVATE_ENDPOINTS", false)
	c.requested = decisionBool(log, "AI_DECISION_ENABLED", true)
	c.envBaseURL = c.BaseURL
	c.envModel = strings.TrimSpace(os.Getenv("AI_DECISION_MODEL"))
	if c.Provider == "none" {
		c.Provider = "off"
	}
	preset, ok := decisionPreset(c.Provider)
	if !ok {
		log.Warn("unknown AI_DECISION_PROVIDER; disabling L2")
		c.Provider = "off"
		return c
	}
	if c.BaseURL == "" {
		c.BaseURL = preset.DefaultBaseURL
	}
	if c.envModel == "" {
		c.Model = preset.DefaultModel
	}
	if !validDecisionHeader(c.AuthHeader) {
		log.Warn("invalid AI_DECISION_AUTH_HEADER; using Authorization")
		c.AuthHeader = "Authorization"
	}
	c.Enabled = c.requested && c.ready()
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

type decisionDialect int

const (
	dialectCompat decisionDialect = iota
	dialectSystemOneMap
	dialectOpenAIArray
)

type DecisionPreset struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	DefaultBaseURL    string `json:"default_base_url"`
	DefaultModel      string `json:"default_model"`
	RequiresAPIKey    bool   `json:"requires_api_key"`
	RequiresAccountID bool   `json:"requires_account_id"`
	AuthHeaderHint    string `json:"auth_header_hint"`
	dialect           decisionDialect
	inputField        string
}

const cloudflareBase = "https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/run/{model}"

var decisionPresets = []DecisionPreset{
	{"gemini_decision", "Gemini Decision 1", "https://generativelanguage.googleapis.com/v1beta/decisions", "gemini-decision-1", true, false, "x-goog-api-key", dialectSystemOneMap, "input"},
	{"openai_decision", "OpenAI Decisions", "https://api.openai.com/v1/decisions", "gpt-6-luna", true, false, "Authorization", dialectOpenAIArray, "input"},
	{"typesafe", "TypeSafe Jev System One", defaultTypeSafeBaseURL, defaultTypeSafeModel, true, false, "Authorization", dialectSystemOneMap, "state"},
	{"cloudflare_clef", "Cloudflare Workers AI Clef", cloudflareBase, "@cf/cloudflare/clef-flash", true, true, "Authorization", dialectSystemOneMap, "state"},
	{"openai", "OpenAI-compatible", defaultBaseURL, defaultModel, true, false, "Authorization", dialectCompat, ""},
	{"http_json", "Custom HTTP JSON", "", defaultModel, false, false, "Authorization", dialectCompat, ""},
	{"off", "Off", "", "", false, false, "", dialectCompat, ""},
}

func DecisionPresets() []DecisionPreset { return slices.Clone(decisionPresets) }
func decisionPreset(id string) (DecisionPreset, bool) {
	for _, p := range decisionPresets {
		if p.ID == id {
			return p, true
		}
	}
	return DecisionPreset{}, false
}
func validDecisionHeader(h string) bool {
	return headerNamePattern.MatchString(h) && !slices.Contains([]string{"host", "cookie", "content-length", "transfer-encoding"}, strings.ToLower(h))
}

var cfAccount = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var cfModel = regexp.MustCompile(`^[A-Za-z0-9@._/-]{1,128}$`)

func (d DecisionConfig) endpoint() (string, error) {
	endpoint := d.BaseURL
	switch d.Provider {
	case "typesafe":
		endpoint = typeSafeEndpoint(endpoint)
	case "cloudflare_clef":
		if !cfModel.MatchString(d.Model) || strings.Contains(d.Model, "..") {
			return "", errors.New("invalid Cloudflare model")
		}
		if d.AccountID != "" && !cfAccount.MatchString(d.AccountID) {
			return "", errors.New("invalid Cloudflare account ID")
		}
		if endpoint != cloudflareBase && strings.Contains(endpoint, "/ai/run/") {
			u, err := url.Parse(endpoint)
			if err != nil {
				return "", errors.New("invalid Cloudflare run URL")
			}
			_, tail, ok := strings.Cut(u.Path, "/accounts/")
			account, model, run := strings.Cut(tail, "/ai/run/")
			if !ok || !run || !cfAccount.MatchString(account) || !cfModel.MatchString(model) || strings.Contains(model, "..") {
				return "", errors.New("invalid Cloudflare run URL")
			}
		} else {
			if !cfAccount.MatchString(d.AccountID) {
				return "", errors.New("Cloudflare account ID required")
			}
			if endpoint == cloudflareBase {
				endpoint = "https://api.cloudflare.com/client/v4"
			}
			endpoint = strings.TrimRight(endpoint, "/") + "/accounts/" + d.AccountID + "/ai/run/" + d.Model
		}
	}
	return endpoint, nil
}
func (d DecisionConfig) Requested() bool { return d.requested }

func (d DecisionConfig) ready() bool {
	p, ok := decisionPreset(d.Provider)
	if !ok || d.Provider == "off" || d.BaseURL == "" || (p.RequiresAPIKey && d.APIKey == "") {
		return false
	}
	endpoint, err := d.endpoint()
	if err != nil {
		return false
	}
	u, err := url.Parse(endpoint)
	return err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https")
}
func ValidateDecisionConfig(ctx context.Context, d DecisionConfig) error {
	preset, ok := decisionPreset(d.Provider)
	if !ok {
		return errors.New("invalid decision provider")
	}
	if !validDecisionHeader(d.AuthHeader) {
		return errors.New("invalid decision auth header")
	}
	endpoint, err := d.endpoint()
	if err != nil {
		return err
	}
	if d.BaseURL != "" && d.BaseURL != preset.DefaultBaseURL {
		u, err := url.Parse(endpoint)
		if err != nil || u.User != nil || u.Fragment != "" {
			return errors.New("invalid decision URL")
		}
		if err = httpc.ValidateRequestURL(ctx, endpoint, d.AllowPrivateEndpoints); err != nil {
			return errors.New("invalid or disallowed decision URL")
		}
	}
	return nil
}
