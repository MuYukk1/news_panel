package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"news_panel/internal/triage"
)

type DecisionOutcome struct {
	Action   string
	Priority string
	Domain   string
	Reason   string
}

type DecisionProvider interface {
	Name() string
	Decide(context.Context, triage.Input, triage.Result) (DecisionOutcome, error)
}

type OpenAIDecisionProvider struct {
	completion *OpenAIProvider
	topics     string
}

func (p *OpenAIDecisionProvider) Name() string { return "openai" }
func (p *OpenAIDecisionProvider) Decide(ctx context.Context, in triage.Input, l1 triage.Result) (DecisionOutcome, error) {
	system, user := decisionPrompts(p.topics, in, l1)
	content, err := p.completion.Complete(ctx, system, user)
	if err != nil {
		return DecisionOutcome{}, errors.New(redact(err.Error(), p.completion.apiKey))
	}
	if json.Valid([]byte(content)) {
		return parseDecision([]byte(content), l1.Domain, p.completion.apiKey)
	}
	// The gate asks for a short verdict. Bound the tolerant prose/fence search too.
	if len(content) > 16<<10 {
		return DecisionOutcome{}, errors.New("decision completion too long")
	}
	obj, err := extractPreferredObject(content, `"action"`, `"keep"`)
	if err != nil {
		return DecisionOutcome{}, err
	}
	return parseDecision([]byte(obj), l1.Domain, p.completion.apiKey)
}

func decisionPrompts(topics string, in triage.Input, l1 triage.Result) (string, string) {
	system := "你是新闻门控分拣器。关注主题：" + topics + "。广告、卖货、水帖、闲聊判 filter/low；突发催化剂、重要发布判 pass/critical 或 high；一般相关判 pass/normal。只输出单个 JSON 对象，字段如下：" + `{"action":"pass|filter","priority":"critical|high|normal|low","domain":"investing|ai|deals|other","reason":"15字内简短理由"}`
	user := fmt.Sprintf("来源：%s 作者：%s\n标题：%s\n正文：%s\n发布时间：%s\nL1：%s/%s", in.SourceType, in.Author, in.Title, boundedText(in.Content, promptContentLimit), in.PublishedAt.UTC().Format(time.RFC3339), l1.Domain, l1.Priority)
	return system, user
}

type HTTPJSONDecisionProvider struct {
	cfg    DecisionConfig
	topics string
	client *http.Client
}

func (p *HTTPJSONDecisionProvider) Name() string { return "http_json" }
func (p *HTTPJSONDecisionProvider) Decide(ctx context.Context, in triage.Input, l1 triage.Result) (DecisionOutcome, error) {
	body, err := json.Marshal(map[string]string{
		"model": p.cfg.Model, "topics": p.topics, "source_type": in.SourceType, "source_id": in.SourceID,
		"author": in.Author, "title": in.Title, "content": boundedText(in.Content, promptContentLimit),
		"url": in.URL, "published_at": in.PublishedAt.UTC().Format(time.RFC3339),
		"l1_domain": l1.Domain, "l1_priority": l1.Priority,
	})
	if err != nil {
		return DecisionOutcome{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return DecisionOutcome{}, errors.New("invalid decision endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	if key := p.cfg.APIKey; key != "" {
		if strings.EqualFold(p.cfg.AuthHeader, "Authorization") && !strings.Contains(key, " ") {
			key = "Bearer " + key
		}
		req.Header.Set(p.cfg.AuthHeader, key)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return DecisionOutcome{}, ctx.Err()
		}
		return DecisionOutcome{}, errors.New("decision HTTP request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DecisionOutcome{}, fmt.Errorf("decision HTTP status %d", resp.StatusCode)
	}
	data, err := readModelResponse(resp.Body)
	if err != nil {
		return DecisionOutcome{}, errors.New("failed to read bounded decision response")
	}
	return parseDecision(data, l1.Domain, p.cfg.APIKey)
}

type TypeSafeDecisionProvider struct {
	cfg    DecisionConfig
	topics string
	client *http.Client
}

type typeSafeQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

func (p *TypeSafeDecisionProvider) Name() string { return "typesafe" }

func typeSafeEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(trimmed, "/systemone") {
		return trimmed
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed + "/systemone"
	}
	return trimmed + "/v1/systemone"
}

func (p *TypeSafeDecisionProvider) Decide(ctx context.Context, in triage.Input, l1 triage.Result) (DecisionOutcome, error) {
	state := fmt.Sprintf("Source: %s\nAuthor: %s\nTitle: %s\nContent: %s\nPublished: %s\nL1 Hint: domain=%s priority=%s",
		in.SourceType, in.Author, in.Title, boundedText(in.Content, promptContentLimit),
		in.PublishedAt.UTC().Format(time.RFC3339), l1.Domain, l1.Priority)
	payload := map[string]any{
		"model": p.cfg.Model,
		"state": state,
		"questions": map[string]typeSafeQuestion{
			"action": {
				Type:         "choice",
				Instructions: "Decide whether this item is worth forwarding to detailed AI summarization for user focus topics: " + p.topics + ". Filter out casual chatter, personal venting, job ads, self-promotion, spam, and off-topic posts.",
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
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DecisionOutcome{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, typeSafeEndpoint(p.cfg.BaseURL), bytes.NewReader(body))
	if err != nil {
		return DecisionOutcome{}, errors.New("invalid typesafe decision endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	if key := p.cfg.APIKey; key != "" {
		if !strings.Contains(key, " ") {
			key = "Bearer " + key
		}
		req.Header.Set("Authorization", key)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return DecisionOutcome{}, ctx.Err()
		}
		return DecisionOutcome{}, errors.New("typesafe decision HTTP request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DecisionOutcome{}, fmt.Errorf("typesafe decision HTTP status %d", resp.StatusCode)
	}
	data, err := readModelResponse(resp.Body)
	if err != nil {
		return DecisionOutcome{}, errors.New("failed to read bounded typesafe decision response")
	}
	return parseTypeSafeDecision(data, l1.Domain, p.cfg.APIKey)
}

func parseTypeSafeDecision(data []byte, l1Domain, key string) (DecisionOutcome, error) {
	var resp struct {
		Answers map[string]struct {
			Choice     string  `json:"choice"`
			Confidence float64 `json:"confidence"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return DecisionOutcome{}, errors.New("invalid typesafe decision JSON")
	}
	act := resp.Answers["action"]
	if strings.TrimSpace(act.Choice) == "" {
		return DecisionOutcome{}, errors.New("typesafe decision missing action choice")
	}
	prio := resp.Answers["priority"]
	dom := resp.Answers["domain"]
	reason := fmt.Sprintf("jev:%s(%.2f)/%s(%.2f)", strings.TrimSpace(act.Choice), act.Confidence, strings.TrimSpace(prio.Choice), prio.Confidence)
	return normalizeDecision(act.Choice, prio.Choice, dom.Choice, reason, nil, l1Domain, key)
}

// Refuse redirects so a custom authentication header stays at the configured endpoint.
func newDecisionProvider(cfg Config) DecisionProvider {
	c := cfg.Decision
	if !c.Enabled {
		return nil
	}
	client := &http.Client{Timeout: c.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	switch c.Provider {
	case "openai":
		completion := NewOpenAIProvider(c.BaseURL, c.APIKey, c.Model, c.JSONFormat, c.Timeout)
		completion.client = client
		return &OpenAIDecisionProvider{completion, cfg.Topics}
	case "typesafe":
		return &TypeSafeDecisionProvider{c, cfg.Topics, client}
	case "http_json":
		return &HTTPJSONDecisionProvider{c, cfg.Topics, client}
	}
	return nil
}

func readModelResponse(r io.Reader) ([]byte, error) {
	const limit = 1 << 20
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if len(b) > limit {
		return nil, errors.New("model response exceeds 1 MiB")
	}
	return b, err
}

// boundedText cuts at a UTF-8 boundary without allocating a full rune slice.
func boundedText(s string, limit int) string {
	n := 0
	for i := range s {
		if n == limit {
			return s[:i]
		}
		n++
	}
	return s
}

func parseDecision(data []byte, l1Domain, key string) (DecisionOutcome, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return DecisionOutcome{}, errors.New("invalid decision JSON")
	}
	for _, wrapper := range []string{"data", "decision", "result"} {
		if obj["action"] != nil || obj["keep"] != nil {
			break
		}
		if nested := obj[wrapper]; nested != nil {
			data = nested
			break
		}
	}
	var raw struct {
		Action, Priority, Domain, Reason string
		Keep                             *bool
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return DecisionOutcome{}, errors.New("invalid decision fields")
	}
	return normalizeDecision(raw.Action, raw.Priority, raw.Domain, raw.Reason, raw.Keep, l1Domain, key)
}

func normalizeDecision(rawAction, rawPriority, rawDomain, rawReason string, rawKeep *bool, l1Domain, key string) (DecisionOutcome, error) {
	action := strings.ToLower(strings.TrimSpace(rawAction))
	switch action {
	case "pass", "keep", "allow":
		action = "pass"
	case "filter", "drop", "reject", "block":
		action = "filter"
	case "":
		if rawKeep == nil {
			return DecisionOutcome{}, errors.New("decision missing action or keep")
		}
		action = "filter"
		if *rawKeep {
			action = "pass"
		}
	default:
		return DecisionOutcome{}, errors.New("invalid decision action")
	}
	priority := strings.ToLower(strings.TrimSpace(rawPriority))
	if priority == "" {
		priority = "normal"
		if action == "filter" {
			priority = "low"
		}
	}
	if !validPriority(priority) {
		return DecisionOutcome{}, errors.New("invalid decision priority")
	}
	domain := strings.ToLower(strings.TrimSpace(rawDomain))
	switch domain {
	case "ai", "other", "investing", "deals":
	default:
		domain = l1Domain
	}
	return DecisionOutcome{Action: action, Domain: domain, Priority: priority, Reason: boundedText(redactCredential(strings.TrimSpace(rawReason), key), 64)}, nil
}
