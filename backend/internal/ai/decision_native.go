package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"news_panel/internal/triage"
	"sort"
	"strings"
	"time"
)

func buildDecisionSpec(topics string, in triage.Input, l1 triage.Result) (string, map[string]decisionQuestion) {
	state := fmt.Sprintf("Source: %s\nAuthor: %s\nTitle: %s\nContent: %s\nPublished: %s\nL1 Hint: domain=%s priority=%s",
		in.SourceType, in.Author, in.Title, boundedText(in.Content, promptContentLimit),
		in.PublishedAt.UTC().Format(time.RFC3339), l1.Domain, l1.Priority)
	return state, map[string]decisionQuestion{
		"action": {
			Type:         "choice",
			Instructions: "Decide whether this item is worth forwarding to detailed AI summarization for user focus topics: " + topics + ". Filter out casual chatter, personal venting, job ads, self-promotion, spam, and off-topic posts.",
			Criteria: map[string]string{
				"pass":   "Relevant news, AI/tech release, high-signal engineering insight, investing catalyst, or notable deal/discount",
				"filter": "Casual chatter, personal venting, basic Q&A troubleshooting, ads, spam, or off-topic low-value post",
			},
		},
		"priority": {
			Type:         "choice",
			Instructions: "Assign reader priority level for this item.",
			Criteria: map[string]string{
				"critical": "Breaking major release, urgent market catalyst, critical outage, or time-sensitive pricing bug",
				"high":     "Notable product launch, strong technical deep-dive, important market signal, or solid discount",
				"normal":   "Moderately useful industry update, tool share, or relevant discussion",
				"low":      "Minor, routine, low-signal, or mostly noise",
			},
		},
		"domain": {
			Type:         "choice",
			Instructions: "Classify the primary domain of this item.",
			Criteria: map[string]string{
				"ai":        "AI, LLMs, developer tools, software engineering, tech releases",
				"investing": "Stocks, macroeconomics, markets, finance, crypto catalysts",
				"deals":     "Discounts, promotions, pricing bugs, coupons, freebies, VPS deals",
				"other":     "General topics not matching the above",
			},
		},
	}
}

type nativeDecisionProvider struct {
	preset DecisionPreset
	cfg    DecisionConfig
	topics string
	client *http.Client
}

func (p *nativeDecisionProvider) Name() string { return p.cfg.Provider }

type namedChoice struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}
type namedQuestion struct {
	Type         string        `json:"type"`
	Name         string        `json:"name"`
	Instructions string        `json:"instructions"`
	Choices      []namedChoice `json:"choices"`
}

func arrayQuestions(spec map[string]decisionQuestion) []namedQuestion {
	questions := make([]namedQuestion, 0, len(spec))
	for _, name := range []string{"action", "priority", "domain"} {
		q := spec[name]
		keys := make([]string, 0, len(q.Criteria))
		for k := range q.Criteria {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		choices := make([]namedChoice, 0, len(keys))
		for _, k := range keys {
			choices = append(choices, namedChoice{k, q.Criteria[k]})
		}
		questions = append(questions, namedQuestion{q.Type, name, q.Instructions, choices})
	}
	return questions
}

func (p *nativeDecisionProvider) Decide(ctx context.Context, in triage.Input, l1 triage.Result) (DecisionOutcome, error) {
	state, questions := buildDecisionSpec(p.topics, in, l1)
	preset := p.preset
	payload := map[string]any{"model": p.cfg.Model, preset.inputField: state, "questions": questions}
	if preset.dialect == dialectOpenAIArray {
		payload["questions"] = arrayQuestions(questions)
	}
	endpoint, err := p.cfg.endpoint()
	if err != nil {
		return DecisionOutcome{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DecisionOutcome{}, err
	}
	data, err := doDecisionPOST(ctx, p.client, p.cfg, endpoint, preset.AuthHeaderHint, body)
	if err != nil {
		return DecisionOutcome{}, err
	}
	return parseNativeAnswers(data, preset, l1.Domain, p.cfg.APIKey)
}

type nativeAnswer struct {
	Name, Choice, Selected, Value string
	Confidence                    float64
}

func (a nativeAnswer) choice() string {
	for _, value := range []string{a.Choice, a.Selected, a.Value} {
		if s := strings.TrimSpace(value); s != "" {
			return s
		}
	}
	return ""
}
func parseNativeAnswers(data []byte, preset DecisionPreset, domain, key string) (DecisionOutcome, error) {
	var resp struct {
		Success *bool
		Result  struct{ Answers json.RawMessage }
		Answers json.RawMessage
	}
	invalid := errors.New("invalid decision answers")
	if json.Unmarshal(data, &resp) != nil {
		return DecisionOutcome{}, invalid
	}
	if resp.Success != nil && !*resp.Success {
		return DecisionOutcome{}, errors.New("decision provider reported failure")
	}
	answers := resp.Answers
	if len(resp.Result.Answers) > 0 {
		answers = resp.Result.Answers
	}
	byName := map[string]nativeAnswer{}
	if preset.dialect == dialectOpenAIArray {
		var arr []nativeAnswer
		if json.Unmarshal(answers, &arr) != nil {
			return DecisionOutcome{}, invalid
		}
		for _, a := range arr {
			byName[a.Name] = a
		}
	} else if json.Unmarshal(answers, &byName) != nil {
		return DecisionOutcome{}, invalid
	}
	a, p := byName["action"], byName["priority"]
	provider := preset.ID
	if provider == "typesafe" {
		provider = "jev"
	}
	reason := fmt.Sprintf("%s:%s(%.2f)/%s(%.2f)", provider, a.choice(), a.Confidence, p.choice(), p.Confidence)
	return normalizeDecision(a.choice(), p.choice(), byName["domain"].choice(), reason, nil, domain, key)
}

type DecisionTestResult struct {
	OK        bool            `json:"ok"`
	Provider  string          `json:"provider"`
	Model     string          `json:"model"`
	LatencyMS int64           `json:"latency_ms"`
	Outcome   DecisionOutcome `json:"outcome"`
	Error     string          `json:"error"`
}

func TestDecisionConfig(ctx context.Context, cfg Config) DecisionTestResult {
	d := cfg.Decision
	result := DecisionTestResult{Provider: d.Provider, Model: d.Model}
	if err := ValidateDecisionConfig(ctx, d); err != nil {
		result.Error = err.Error()
		return result
	}
	if !d.ready() {
		result.Error = "decision provider is off or unconfigured"
		return result
	}
	cfg.Decision.Enabled = true
	start := time.Now()
	outcome, err := newDecisionProvider(cfg).Decide(ctx, triage.Input{SourceType: "rss", Title: "OpenAI released a new model update with lower latency.", PublishedAt: start}, triage.Result{Domain: "ai", Priority: "normal"})
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = redactCredential(err.Error(), d.APIKey)
		return result
	}
	result.OK = true
	result.Outcome = outcome
	return result
}
