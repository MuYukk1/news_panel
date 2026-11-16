// Package triage owns the small, implementation-independent processor contract.
// P1-03 can implement TriageEvaluator without changing storage or HTTP envelopes.
package triage

import (
	"context"
	"fmt"
	"time"
)

// Input is the normalized envelope shared by HTTP and in-process sources.
type Input struct {
	SourceID    string
	SourceType  string
	ExternalID  string
	URL         string
	Title       string
	Content     string
	Author      string
	PublishedAt time.Time
	RawMeta     map[string]any
}

type Result struct {
	Domain       string   `json:"domain"`
	AIScore      *int     `json:"ai_score"`
	AITags       []string `json:"ai_tags"`
	AISummary    string   `json:"ai_summary"`
	Priority     string   `json:"priority"`
	TriageStatus string   `json:"triage_status"`
}

type TriageEvaluator interface {
	Evaluate(context.Context, Input) (Result, error)
}

// PassThrough keeps unscored items visible until the AI evaluator is installed.
type PassThrough struct{}

func (PassThrough) Evaluate(ctx context.Context, _ Input) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return Result{Domain: "other", AITags: []string{}, Priority: "normal", TriageStatus: "kept"}, nil
}

func (r Result) Validate() error {
	if r.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if r.AIScore != nil && (*r.AIScore < 0 || *r.AIScore > 100) {
		return fmt.Errorf("score outside 0..100")
	}
	switch r.Priority {
	case "critical", "high", "normal", "low":
	default:
		return fmt.Errorf("invalid priority")
	}
	switch r.TriageStatus {
	case "pending", "kept", "filtered", "failed":
	default:
		return fmt.Errorf("invalid triage status")
	}
	return nil
}
