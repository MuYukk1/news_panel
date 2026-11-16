package ai

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"news_panel/internal/triage"
)

type stubProvider struct {
	out        string
	err        error
	calls      int32
	lastSystem string
	lastUser   string
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) Complete(ctx context.Context, system, user string) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	s.lastSystem, s.lastUser = system, user
	return s.out, s.err
}

func bufLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, nil)), &buf
}

func l2Config() Config {
	return Config{Enabled: true, Provider: "openai", APIKey: "sk-test", KeepMinScore: 45,
		Timeout: 2 * time.Second, FallbackMode: "keep_l1", L1MinContentRunes: 10, Topics: defaultTopics}
}

func evalWith(cfg Config, p CompletionProvider, logger *slog.Logger) *Evaluator {
	if logger == nil {
		logger = discardLogger()
	}
	return &Evaluator{cfg: cfg, l1: newL1Rules(cfg), logger: logger, provider: p}
}

const goodVerdict = `{"domain":"ai","score":80,"tags":["OpenAI"],"summary":"一句话摘要","priority":"normal","keep":true}`

func TestEvaluatorL1OnlyDegradation(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"disabled", Config{Enabled: false, Provider: "openai", APIKey: "sk-test", L1MinContentRunes: 10}},
		{"provider l1", Config{Enabled: true, Provider: "l1", L1MinContentRunes: 10}},
		{"openai no key", Config{Enabled: true, Provider: "openai", APIKey: "", L1MinContentRunes: 10}},
		{"cli no command", Config{Enabled: true, Provider: "cli", CLICommand: "", L1MinContentRunes: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := New(tc.cfg, discardLogger())
			if e.provider != nil {
				t.Fatalf("expected L1-only (nil provider)")
			}
			got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
			if err != nil {
				t.Fatalf("L1-only must not error: %v", err)
			}
			assertValid(t, got)
			if got.TriageStatus != "kept" || got.AIScore != nil {
				t.Errorf("L1-only keep with nil score: %+v", got)
			}
			if !containsTag(got.AITags, "l1-only") {
				t.Errorf("expected l1-only tag: %#v", got.AITags)
			}
			noise, err := e.Evaluate(context.Background(), l1Input("互粉互关回关", ""))
			if err != nil || noise.TriageStatus != "filtered" {
				t.Errorf("L1-only noise should filter without error: %+v %v", noise, err)
			}
		})
	}
}

func TestEvaluatorL2Success(t *testing.T) {
	stub := &stubProvider{out: goodVerdict}
	e := evalWith(l2Config(), stub, nil)
	got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	assertValid(t, got)
	if got.TriageStatus != "kept" || got.Domain != "ai" || got.AIScore == nil || *got.AIScore != 80 {
		t.Errorf("L2 success mapping: %+v", got)
	}
	if atomic.LoadInt32(&stub.calls) != 1 {
		t.Errorf("provider should be called once")
	}
}

func TestEvaluatorL2Filtered(t *testing.T) {
	stub := &stubProvider{out: `{"domain":"other","score":20,"tags":[],"summary":"水帖","priority":"low","keep":false}`}
	e := evalWith(l2Config(), stub, nil)
	got, err := e.Evaluate(context.Background(), l1Input("今天随便说点什么没有信息量", ""))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.TriageStatus != "filtered" {
		t.Errorf("keep=false should filter: %+v", got)
	}
}

func TestEvaluatorFallbackKeepL1(t *testing.T) {
	stub := &stubProvider{err: errors.New("network unreachable")}
	e := evalWith(l2Config(), stub, nil)
	got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil {
		t.Fatalf("keep_l1 must not error: %v", err)
	}
	assertValid(t, got)
	if got.TriageStatus != "kept" || got.AIScore != nil {
		t.Errorf("keep_l1 should preserve L1 keep with nil score: %+v", got)
	}
	if !containsTag(got.AITags, "l2-fallback") {
		t.Errorf("expected l2-fallback tag: %#v", got.AITags)
	}
}

func TestEvaluatorFallbackFail(t *testing.T) {
	cfg := l2Config()
	cfg.FallbackMode = "fail"
	stub := &stubProvider{err: errors.New("network unreachable")}
	e := evalWith(cfg, stub, nil)
	_, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err == nil {
		t.Fatalf("fail mode must return error so handler stores failed")
	}
}

func TestEvaluatorUnparseableFallsBack(t *testing.T) {
	stub := &stubProvider{out: "抱歉我无法输出 JSON"}
	e := evalWith(l2Config(), stub, nil)
	got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil {
		t.Fatalf("unparseable should fall back, not error: %v", err)
	}
	if got.TriageStatus != "kept" || !containsTag(got.AITags, "l2-fallback") {
		t.Errorf("expected L1 fallback: %+v", got)
	}
}

func TestEvaluatorContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, withProvider := range []bool{false, true} {
		var p CompletionProvider
		stub := &stubProvider{out: goodVerdict}
		if withProvider {
			p = stub
		}
		e := evalWith(l2Config(), p, nil)
		_, err := e.Evaluate(ctx, l1Input("OpenAI 发布新模型", ""))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("withProvider=%v: expected context.Canceled, got %v", withProvider, err)
		}
		if withProvider && atomic.LoadInt32(&stub.calls) != 0 {
			t.Errorf("cancelled ctx must short-circuit before L2")
		}
	}
}

func TestEvaluatorNoiseSkipsL2(t *testing.T) {
	stub := &stubProvider{out: goodVerdict}
	e := evalWith(l2Config(), stub, nil)
	got, err := e.Evaluate(context.Background(), l1Input("互粉互关回关必回", ""))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.TriageStatus != "filtered" {
		t.Errorf("noise should filter: %+v", got)
	}
	if atomic.LoadInt32(&stub.calls) != 0 {
		t.Errorf("noise must not consume L2 tokens")
	}
}

func TestEvaluatorAllowKeywordReachesL2(t *testing.T) {
	cfg := l2Config()
	cfg.L1AllowKeywords = []string{"薅羊毛"}
	stub := &stubProvider{out: goodVerdict}
	e := evalWith(cfg, stub, nil)
	if _, err := e.Evaluate(context.Background(), l1Input("薅羊毛互粉抽奖线报", "")); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if atomic.LoadInt32(&stub.calls) != 1 {
		t.Errorf("allow keyword should bypass L1 noise and reach L2")
	}
}

func TestEvaluatorThresholdViaL2(t *testing.T) {
	cases := []struct {
		out        string
		wantStatus string
	}{
		{`{"domain":"ai","score":30,"tags":[],"summary":"s","priority":"normal","keep":true}`, "filtered"},
		{`{"domain":"ai","score":30,"tags":[],"summary":"s","priority":"high","keep":true}`, "kept"},
		{`{"domain":"ai","score":90,"tags":[],"summary":"s","priority":"normal","keep":false}`, "filtered"},
	}
	for _, tc := range cases {
		e := evalWith(l2Config(), &stubProvider{out: tc.out}, nil)
		got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got.TriageStatus != tc.wantStatus {
			t.Errorf("out %s: status=%q want %q", tc.out, got.TriageStatus, tc.wantStatus)
		}
	}
}

func TestEvaluatorLogRedaction(t *testing.T) {
	const key = "sk-secret-log-key-42"
	cfg := l2Config()
	cfg.APIKey = key
	logger, buf := bufLogger()
	stub := &stubProvider{err: errors.New("dial failed using " + key)}
	e := evalWith(cfg, stub, logger)
	if _, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型", "")); err != nil {
		t.Fatalf("keep_l1 should not error: %v", err)
	}
	logged := buf.String()
	if strings.Contains(logged, key) {
		t.Errorf("API key leaked into logs: %s", logged)
	}
	if !strings.Contains(logged, "[REDACTED]") {
		t.Errorf("expected redaction marker in logs: %s", logged)
	}
}

func TestEvaluatorFailModeErrorRedacted(t *testing.T) {
	const key = "sk-secret-err-key-77"
	cfg := l2Config()
	cfg.APIKey = key
	cfg.FallbackMode = "fail"
	stub := &stubProvider{err: errors.New("upstream rejected " + key)}
	e := evalWith(cfg, stub, nil)
	_, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型", ""))
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("API key leaked into returned error: %v", err)
	}
}

func TestEvaluatorPromptsCarryContext(t *testing.T) {
	cfg := l2Config()
	cfg.Topics = "自定义主题XYZ"
	stub := &stubProvider{out: goodVerdict}
	e := evalWith(cfg, stub, nil)
	in := l1Input("OpenAI 发布新模型正文内容", "标题ABC")
	in.Author = "@OpenAI"
	if _, err := e.Evaluate(context.Background(), in); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !strings.Contains(stub.lastSystem, "自定义主题XYZ") || !strings.Contains(stub.lastSystem, "45") {
		t.Errorf("system prompt missing topics/threshold: %s", stub.lastSystem)
	}
	if !strings.Contains(stub.lastUser, "标题ABC") || !strings.Contains(stub.lastUser, "@OpenAI") || !strings.Contains(stub.lastUser, "正文内容") {
		t.Errorf("user prompt missing item fields: %s", stub.lastUser)
	}
}

func TestEvaluatorOpenAIIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(okCompletionBody(goodVerdict)))
	}))
	defer srv.Close()
	cfg := l2Config()
	cfg.BaseURL = srv.URL
	e := New(cfg, discardLogger())
	got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil {
		t.Fatalf("integration Evaluate: %v", err)
	}
	assertValid(t, got)
	if got.TriageStatus != "kept" || got.AIScore == nil || *got.AIScore != 80 {
		t.Errorf("integration result: %+v", got)
	}
}

func TestNewFromEnvWiring(t *testing.T) {
	clearAIEnv(t)
	t.Setenv("AI_PROVIDER", "l1")
	e := NewFromEnv()
	if e == nil {
		t.Fatalf("NewFromEnv must never return nil")
	}
	got, err := e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil || got.TriageStatus != "kept" {
		t.Errorf("env L1-only wiring: %+v %v", got, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(okCompletionBody(goodVerdict)))
	}))
	defer srv.Close()
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_PROVIDER", "openai")
	t.Setenv("AI_API_KEY", "sk-env")
	t.Setenv("AI_BASE_URL", srv.URL)
	e = NewFromEnv()
	got, err = e.Evaluate(context.Background(), l1Input("OpenAI 发布新模型值得关注", ""))
	if err != nil {
		t.Fatalf("env openai wiring: %v", err)
	}
	if got.AIScore == nil || *got.AIScore != 80 {
		t.Errorf("env openai should score via L2: %+v", got)
	}
}

func TestEvaluatorImplementsContract(t *testing.T) {
	var _ triage.TriageEvaluator = New(Config{}, discardLogger())
}