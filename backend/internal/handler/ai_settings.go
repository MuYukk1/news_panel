package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"news_panel/internal/ai"
	"news_panel/internal/model"
)

type settingsPatch struct {
	model.AISettings
	ClearKey bool `json:"clear_decision_api_key"`
	Reset    bool `json:"reset_to_env"`
}

// In anonymous-reader mode the rest of the panel can stay public. Secrets and
// outbound probes require a reader login.
func (h *Handler) requireAIReader(c *gin.Context) {
	if h.allowAnonAPI {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "AI settings require reader authentication"})
		return
	}
	c.Next()
}

func readSettingsPatch(c *gin.Context, optional bool) (settingsPatch, bool) {
	var patch settingsPatch
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10))
	dec.DisallowUnknownFields()
	err := dec.Decode(&patch)
	if optional && err == io.EOF {
		return patch, true
	}
	if err != nil || dec.Decode(new(any)) != io.EOF {
		badRequestError(c, "invalid AI settings request")
		return patch, false
	}
	if err := ai.ValidateSettingsPatch(patch.AISettings); err != nil {
		badRequestError(c, err.Error())
		return patch, false
	}
	return patch, true
}

func (h *Handler) loadSettings(c *gin.Context) (*ai.Evaluator, model.AISettings, bool) {
	eval, ok := h.evaluator.(*ai.Evaluator)
	if !ok || h.store == nil {
		c.JSON(503, gin.H{"error": "AI settings unavailable"})
		return nil, model.AISettings{}, false
	}
	row, err := h.store.GetAISettings()
	if err != nil {
		internalError(c, errors.New("could not read AI settings"), "AI settings")
		return nil, row, false
	}
	return eval, row, true
}

func maskedKey(key string) string {
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) < 8 {
		return "••••"
	}
	return "••••" + string(runes[len(runes)-4:])
}

// A URL can contain credentials supplied by the operator. Omit its userinfo
// and query from the reader-facing representation.
func displayURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.User == nil && u.RawQuery == "" && u.Fragment == "" {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
func settingsView(eval *ai.Evaluator, row model.AISettings) gin.H {
	cfg := eval.EnvConfig().Merged(row)
	d := cfg.Decision
	sources := map[string]string{}
	source := func(field, env string, overridden bool) {
		switch {
		case overridden:
			sources[field] = "db"
		case strings.TrimSpace(os.Getenv(env)) != "":
			sources[field] = "env"
		default:
			sources[field] = "default"
		}
	}
	source("decision_enabled", "AI_DECISION_ENABLED", row.DecisionEnabled != nil)
	source("decision_provider", "AI_DECISION_PROVIDER", row.DecisionProvider != nil)
	source("decision_base_url", "AI_DECISION_BASE_URL", row.DecisionBaseURL != nil)
	source("decision_api_key", "AI_DECISION_API_KEY", row.DecisionAPIKey != nil)
	source("decision_account_id", "AI_DECISION_ACCOUNT_ID", row.DecisionAccountID != nil)
	source("decision_auth_header", "AI_DECISION_AUTH_HEADER", row.DecisionAuthHeader != nil)
	source("decision_model", "AI_DECISION_MODEL", row.DecisionModel != nil)
	source("decision_json_format", "AI_DECISION_JSON_FORMAT", row.DecisionJSONFormat != nil)
	source("decision_timeout_seconds", "AI_DECISION_TIMEOUT_SECONDS", row.DecisionTimeoutSeconds != nil)
	source("decision_l3_priorities", "AI_DECISION_L3_PRIORITIES", row.DecisionL3Priorities != nil)
	source("decision_low_action", "AI_DECISION_LOW_ACTION", row.DecisionLowAction != nil)
	source("decision_fallback_mode", "AI_DECISION_FALLBACK_MODE", row.DecisionFallbackMode != nil)
	source("topics", "AI_TOPICS", row.Topics != nil)
	source("rss_max_items_per_pull", "AI_RSS_MAX_ITEMS_PER_PULL", row.RSSMaxItemsPerPull != nil)
	keySource := "none"
	if d.APIKey != "" {
		keySource = "env"
		if row.DecisionAPIKey != nil {
			keySource = "db"
		}
	}
	sources["decision_api_key"] = keySource
	overrides := map[string]bool{}
	for name, src := range sources {
		if src == "db" {
			overrides[name] = true
		}
	}
	effective := gin.H{
		"decision_enabled":         cfg.Decision.Enabled,
		"decision_provider":        cfg.Decision.Provider,
		"decision_base_url":        displayURL(d.BaseURL),
		"decision_account_id":      cfg.Decision.AccountID,
		"decision_auth_header":     cfg.Decision.AuthHeader,
		"decision_model":           cfg.Decision.Model,
		"decision_json_format":     cfg.Decision.JSONFormat,
		"decision_timeout_seconds": int(d.Timeout / time.Second),
		"decision_l3_priorities":   strings.Join(d.L3Priorities, ","),
		"decision_low_action":      cfg.Decision.LowAction,
		"decision_fallback_mode":   cfg.Decision.FallbackMode,
		"topics":                   cfg.Topics,
		"rss_max_items_per_pull":   cfg.RSSLimit,
	}
	stats := eval.Stats()
	return gin.H{"decision_requested": d.Requested(), "effective": effective, "sources": sources, "overrides": overrides, "presets": ai.DecisionPresets(),
		"decision_api_key_configured": d.APIKey != "", "decision_api_key_masked": maskedKey(d.APIKey), "decision_api_key_source": keySource,
		"l3_enabled": stats.Enabled, "l3_model": cfg.Model, "l3_provider": cfg.Provider}
}

func (h *Handler) getAISettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	h.aiSettingsMu.Lock()
	defer h.aiSettingsMu.Unlock()
	eval, row, ok := h.loadSettings(c)
	if ok {
		dataResponse(c, settingsView(eval, row))
	}
}

func sameOrigin(a, b string) bool {
	x, e := url.Parse(a)
	y, f := url.Parse(b)
	return e == nil && f == nil && x.Host != "" && strings.EqualFold(x.Scheme, y.Scheme) && strings.EqualFold(x.Host, y.Host)
}
func prepareSettings(ctx context.Context, env ai.Config, old model.AISettings, p settingsPatch) (model.AISettings, ai.Config, error) {
	if p.Reset {
		return model.AISettings{}, env, nil
	}
	next := old.Apply(p.AISettings, p.ClearKey)
	merged := env.Merged(next)
	d := merged.Decision
	if err := ai.ValidateDecisionConfig(ctx, d); err != nil {
		return old, env, err
	}
	// An inherited credential may be used at its original origin or a trusted
	// preset. A custom origin change needs a freshly supplied credential.
	if d.APIKey != "" && !(d.Provider == "off" && d.BaseURL == "") && (p.DecisionAPIKey == nil || *p.DecisionAPIKey == "" || p.ClearKey) {
		origin := env.Decision.BaseURL
		if next.DecisionAPIKey != nil {
			origin = env.Merged(old).Decision.BaseURL
		}
		trusted := sameOrigin(d.BaseURL, origin)
		for _, preset := range ai.DecisionPresets() {
			if d.Provider == preset.ID && d.BaseURL == preset.DefaultBaseURL {
				trusted = true
			}
		}
		if !trusted {
			return old, env, errors.New("enter an API key for the new endpoint")
		}
	}
	return next, merged, nil
}
func (h *Handler) patchAISettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	patch, ok := readSettingsPatch(c, false)
	if !ok {
		return
	}
	h.aiSettingsMu.Lock()
	defer h.aiSettingsMu.Unlock()
	eval, old, ok := h.loadSettings(c)
	if !ok {
		return
	}
	next, cfg, err := prepareSettings(c.Request.Context(), eval.EnvConfig(), old, patch)
	if err != nil {
		badRequestError(c, err.Error())
		return
	}
	if patch.Reset {
		err = h.store.ResetAISettings()
	} else {
		err = h.store.UpdateAISettings(patch.AISettings, patch.ClearKey)
	}
	if err != nil {
		internalError(c, errors.New("could not save AI settings"), "AI settings")
		return
	}
	eval.Reload(cfg)
	if setter, ok := h.puller.(interface{ SetRSSLimit(int) }); ok {
		setter.SetRSSLimit(cfg.RSSLimit)
	}
	dataResponse(c, settingsView(eval, next))
}

func (h *Handler) testAISettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	patch, ok := readSettingsPatch(c, true)
	if !ok {
		return
	}
	cfg, ok := h.reserveDecisionTest(c, patch)
	if ok {
		dataResponse(c, ai.TestDecisionConfig(c.Request.Context(), cfg))
	}
}
func (h *Handler) reserveDecisionTest(c *gin.Context, patch settingsPatch) (ai.Config, bool) {
	h.aiSettingsMu.Lock()
	defer h.aiSettingsMu.Unlock()
	eval, old, ok := h.loadSettings(c)
	if !ok {
		return ai.Config{}, false
	}
	_, cfg, err := prepareSettings(c.Request.Context(), eval.EnvConfig(), old, patch)
	if err != nil {
		badRequestError(c, err.Error())
		return cfg, false
	}
	if time.Since(h.lastDecisionTest) < 2*time.Second {
		c.Header("Retry-After", "2")
		c.JSON(429, gin.H{"error": "wait two seconds between connection tests"})
		return cfg, false
	}
	h.lastDecisionTest = time.Now()
	return cfg, true
}
