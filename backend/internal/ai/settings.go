package ai

import (
	"errors"
	"news_panel/internal/model"
	"slices"
	"strings"
	"time"
)

// Merged overlays durable settings on the original startup config, never on the last reload.
func (cfg Config) Merged(row model.AISettings) Config {
	d := &cfg.Decision
	requested := d.requested || d.Enabled
	if row.DecisionEnabled != nil {
		requested = *row.DecisionEnabled
	}
	if row.DecisionProvider != nil && *row.DecisionProvider != d.Provider {
		d.Provider = *row.DecisionProvider
		d.BaseURL = d.envBaseURL
		d.Model = d.envModel
	}
	if row.DecisionBaseURL != nil {
		cfg.Decision.BaseURL = *row.DecisionBaseURL
	}
	if row.DecisionAPIKey != nil {
		cfg.Decision.APIKey = *row.DecisionAPIKey
	}
	if row.DecisionAccountID != nil {
		cfg.Decision.AccountID = *row.DecisionAccountID
	}
	if row.DecisionAuthHeader != nil {
		cfg.Decision.AuthHeader = *row.DecisionAuthHeader
	}
	if row.DecisionModel != nil {
		cfg.Decision.Model = *row.DecisionModel
	}
	if row.DecisionJSONFormat != nil {
		cfg.Decision.JSONFormat = *row.DecisionJSONFormat
	}
	if row.DecisionLowAction != nil {
		cfg.Decision.LowAction = *row.DecisionLowAction
	}
	if row.DecisionFallbackMode != nil {
		cfg.Decision.FallbackMode = *row.DecisionFallbackMode
	}
	if row.Topics != nil {
		cfg.Topics = *row.Topics
	}
	if row.RSSMaxItemsPerPull != nil {
		cfg.RSSLimit = *row.RSSMaxItemsPerPull
	}
	preset, _ := decisionPreset(d.Provider)
	if d.BaseURL == "" {
		d.BaseURL = preset.DefaultBaseURL
	}
	if d.Model == "" {
		d.Model = preset.DefaultModel
	}
	if row.DecisionTimeoutSeconds != nil {
		d.Timeout = time.Duration(*row.DecisionTimeoutSeconds) * time.Second
	}
	if row.DecisionL3Priorities != nil {
		d.L3Priorities = strings.Split(*row.DecisionL3Priorities, ",")
		for i, v := range d.L3Priorities {
			d.L3Priorities[i] = strings.TrimSpace(v)
		}
	}
	d.requested = requested
	d.Enabled = requested && d.ready()
	return cfg
}

func ValidateSettingsPatch(p model.AISettings) error {
	if p.DecisionProvider != nil {
		if _, ok := decisionPreset(*p.DecisionProvider); !ok {
			return errors.New("invalid decision provider")
		}
	}
	if p.DecisionTimeoutSeconds != nil && (*p.DecisionTimeoutSeconds < 1 || *p.DecisionTimeoutSeconds > 60) {
		return errors.New("timeout must be 1–60 seconds")
	}
	if p.RSSMaxItemsPerPull != nil && (*p.RSSMaxItemsPerPull < 0 || *p.RSSMaxItemsPerPull > 500) {
		return errors.New("RSS limit must be 0–500")
	}
	if p.DecisionL3Priorities != nil {
		for _, v := range strings.Split(*p.DecisionL3Priorities, ",") {
			if !validPriority(strings.TrimSpace(v)) {
				return errors.New("invalid L3 priority")
			}
		}
	}
	if p.DecisionLowAction != nil && !slices.Contains([]string{"filter", "keep_unscored"}, *p.DecisionLowAction) {
		return errors.New("invalid low action")
	}
	if p.DecisionFallbackMode != nil && !slices.Contains([]string{"keep_l1", "skip_to_l3", "fail"}, *p.DecisionFallbackMode) {
		return errors.New("invalid fallback mode")
	}
	if p.DecisionAPIKey != nil && strings.ContainsAny(*p.DecisionAPIKey, "\r\n") {
		return errors.New("invalid API key")
	}
	return nil
}
