package ai

import "context"

// CompletionProvider is the L2 backend abstraction: one completion call per
// item. Implementations must be safe for concurrent use (the CLI provider
// serializes internally) and must never leak credentials in errors.
type CompletionProvider interface {
	Name() string
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}