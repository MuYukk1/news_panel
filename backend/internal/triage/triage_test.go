package triage

import (
	"context"
	"testing"
)

func TestPassThrough(t *testing.T) {
	result, err := (PassThrough{}).Evaluate(context.Background(), Input{Content: "news"})
	if err != nil || result.Validate() != nil || result.TriageStatus != "kept" || result.Domain != "other" || result.Priority != "normal" || result.AIScore != nil || result.AITags == nil {
		t.Fatalf("unexpected result: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (PassThrough{}).Evaluate(ctx, Input{}); err == nil {
		t.Fatal("cancellation ignored")
	}
}
