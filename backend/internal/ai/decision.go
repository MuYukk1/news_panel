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

	"news_panel/internal/pkg/httpc"
	"news_panel/internal/triage"
)

type DecisionOutcome struct {
	Action   string `json:"action"`
	Priority string `json:"priority"`
	Domain   string `json:"domain"`
	Reason   string `json:"reason"`
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
	data, err := doDecisionPOST(ctx, p.client, p.cfg, p.cfg.BaseURL, p.cfg.AuthHeader, body)
	if err != nil {
		return DecisionOutcome{}, err
	}
	return parseDecision(data, l1.Domain, p.cfg.APIKey)
}

func doDecisionPOST(ctx context.Context, client *http.Client, cfg DecisionConfig, endpoint, header string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid decision endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	if key := cfg.APIKey; key != "" {
		if strings.EqualFold(header, "Authorization") && !strings.Contains(key, " ") {
			key = "Bearer " + key
		}
		req.Header.Set(header, key)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("decision HTTP request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("decision HTTP status %d", resp.StatusCode)
	}
	data, err := readModelResponse(resp.Body)
	if err != nil {
		return nil, errors.New("failed to read bounded decision response")
	}
	return data, nil
}

type decisionQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

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

// Refuse redirects so a custom authentication header stays at the configured endpoint.
func newDecisionProvider(cfg Config) DecisionProvider {
	c := cfg.Decision
	if !c.Enabled {
		return nil
	}
	client := httpc.NewNoRedirectClient(c.Timeout, c.AllowPrivateEndpoints)
	preset, _ := decisionPreset(c.Provider)
	if preset.dialect != dialectCompat {
		return &nativeDecisionProvider{preset: preset, cfg: c, topics: cfg.Topics, client: client}
	}
	switch c.Provider {
	case "openai":
		completion := NewOpenAIProvider(c.BaseURL, c.APIKey, c.Model, c.JSONFormat, c.Timeout)
		completion.client = client
		return &OpenAIDecisionProvider{completion, cfg.Topics}
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
