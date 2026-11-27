package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"news_panel/internal/triage"
)

func TestDecisionConfig(t *testing.T) {
	clearAIEnv(t)
	cfg := func() DecisionConfig { return ConfigFromEnv(discardLogger()).Decision }
	c := cfg()
	if c.Enabled || c.Provider != "openai" || c.BaseURL != defaultBaseURL || c.Timeout != 6*time.Second || c.LowAction != "keep_unscored" || c.FallbackMode != "skip_to_l3" || !slices.Equal(c.L3Priorities, []string{"critical", "high", "normal"}) {
		t.Fatal("unexpected decision defaults")
	}
	t.Setenv("AI_DECISION_API_KEY", "example")
	if !cfg().Enabled {
		t.Fatal("missing openai auto-detect")
	}
	t.Setenv("AI_DECISION_ENABLED", "false")
	if cfg().Enabled {
		t.Fatal("ignored explicit false")
	}
	t.Setenv("AI_DECISION_ENABLED", "true")
	for _, provider := range []string{"off", "none", "invalid", "http_json"} {
		t.Setenv("AI_DECISION_PROVIDER", provider)
		if cfg().Enabled {
			t.Errorf("%s should be disabled without endpoint", provider)
		}
	}
	t.Setenv("AI_DECISION_PROVIDER", "http_json")
	t.Setenv("AI_DECISION_BASE_URL", "https://example.test/decide")
	t.Setenv("AI_DECISION_API_KEY", "")
	t.Setenv("AI_DECISION_ENABLED", "")
	t.Setenv("AI_DECISION_L3_PRIORITIES", " HIGH ,low")
	if c := cfg(); !c.Enabled || !slices.Equal(c.L3Priorities, []string{"high", "low"}) {
		t.Fatal("http_json detection or allowlist failed")
	}
	t.Setenv("AI_DECISION_AUTH_HEADER", "bad\nheader")
	if cfg().AuthHeader != "Authorization" {
		t.Fatal("invalid header did not fall back")
	}
	t.Setenv("AI_DECISION_BASE_URL", "not a URL")
	if cfg().Enabled {
		t.Fatal("invalid endpoint enabled")
	}
	for _, tc := range []struct {
		raw  string
		want int
	}{{"", 20}, {"0", 0}, {"5", 5}, {"-1", 20}, {"bad", 20}} {
		t.Setenv("AI_RSS_MAX_ITEMS_PER_PULL", tc.raw)
		if RSSMaxItemsPerPull() != tc.want {
			t.Errorf("RSS limit %q", tc.raw)
		}
	}
}

func TestDecisionAdapters(t *testing.T) {
	const key = "decision-test-key"
	in := l1Input(strings.Repeat("测", 2001), "news")
	in.Author = "reporter"
	l1 := triage.Result{Domain: "deals", Priority: "high"}
	for _, tc := range []struct{ provider, header, key, wantAuth string }{
		{"http_json", "X-API-Key", key, key}, {"http_json", "Authorization", key, "Bearer " + key}, {"http_json", "Authorization", "Token " + key, "Token " + key}, {"openai", "Authorization", key, "Bearer " + key},
	} {
		t.Run(tc.provider+"/"+tc.header, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get(tc.header) != tc.wantAuth || r.Header.Get("Content-Type") != "application/json" {
					t.Error("bad method or headers")
				}
				if tc.provider == "openai" {
					var body struct {
						Model          string
						Messages       []struct{ Content string }
						ResponseFormat map[string]string `json:"response_format"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if r.URL.Path != "/v1/chat/completions" || body.Model != "light" || len(body.Messages) != 2 || body.ResponseFormat["type"] != "json_object" || !strings.Contains(body.Messages[0].Content, "action") {
						t.Error("bad chat request")
					}
					fmt.Fprint(w, okCompletionBody("verdict: "+`{"action":"allow","reason":"`+key+strings.Repeat("密", 80)+`"}`))
					return
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				for k, v := range map[string]string{"model": "light", "topics": "topics", "source_id": in.SourceID, "source_type": in.SourceType, "author": in.Author, "title": in.Title, "content": strings.Repeat("测", 2000), "url": in.URL, "published_at": in.PublishedAt.UTC().Format(time.RFC3339), "l1_priority": l1.Priority, "l1_domain": l1.Domain} {
					if body[k] != v {
						t.Errorf("incorrect field %s", k)
					}
				}
				fmt.Fprint(w, `{"decision":{"keep":true,"reason":"`+key+strings.Repeat("密", 80)+`"}}`)
			}))
			defer srv.Close()
			cfg := Config{Topics: "topics", Decision: DecisionConfig{Enabled: true, Provider: tc.provider, BaseURL: srv.URL, APIKey: tc.key, AuthHeader: tc.header, Model: "light", JSONFormat: true, Timeout: time.Second}}
			got, err := newDecisionProvider(cfg).Decide(context.Background(), in, l1)
			if err != nil || got.Action != "pass" || got.Priority != "normal" || got.Domain != l1.Domain || utf8.RuneCountInString(got.Reason) != 64 || strings.Contains(got.Reason, key) {
				t.Fatalf("bad normalized decision: %+v (%v)", got, err)
			}
		})
	}
}

func TestDecisionRejectsOversizeAndErrors(t *testing.T) {
	const key = "example-secret"
	for _, provider := range []string{"openai", "http_json"} {
		for _, mode := range []string{"oversize", "status", "invalid", "timeout"} {
			t.Run(provider+"/"+mode, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch mode {
					case "oversize":
						fmt.Fprint(w, strings.Repeat(" ", 1<<20)+`{"keep":true}`)
					case "status":
						w.WriteHeader(403)
						fmt.Fprint(w, key)
					case "invalid":
						fmt.Fprint(w, `{"action":"`+key+`"}`)
					case "timeout":
						time.Sleep(100 * time.Millisecond)
					}
				}))
				defer srv.Close()
				timeout := time.Second
				if mode == "timeout" {
					timeout = 20 * time.Millisecond
				}
				p := newDecisionProvider(Config{Decision: DecisionConfig{Enabled: true, Provider: provider, BaseURL: srv.URL, APIKey: key, AuthHeader: "Authorization", Timeout: timeout}})
				_, err := p.Decide(context.Background(), triage.Input{}, triage.Result{})
				if err == nil || strings.Contains(err.Error(), key) {
					t.Fatal("missing error or leaked key")
				}
			})
		}
	}
}

func TestDecisionPipeline(t *testing.T) {
	for _, tc := range []struct {
		name, body, low, mode, tag, status string
		calls                              int64
	}{
		{"noise", "", "", "", "", "filtered", 0},
		{"filter", `{"data":{"action":"drop"}}`, "", "", "l2-decision-filtered", "filtered", 0},
		{"low", `{"result":{"action":"keep","priority":"low"}}`, "", "", "l2-decision-bypass", "kept", 0},
		{"low-filter", `{"action":"pass","priority":"low"}`, "filter", "", "l2-decision-low-filtered", "filtered", 0},
		{"pass", `{"action":"pass","priority":"high","domain":"deals"}`, "", "", "", "kept", 1},
		{"fail-open", "{", "", "skip_to_l3", "", "kept", 1},
		{"keep-l1", "{", "", "keep_l1", "l2-decision-fallback", "kept", 0},
		{"fail", "{", "", "fail", "", "", 0},
		{"disabled", "", "", "", "", "kept", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); fmt.Fprint(w, tc.body) }))
			defer srv.Close()
			cfg := l2Config()
			cfg.Decision = DecisionConfig{Provider: "http_json", Enabled: tc.name != "disabled", BaseURL: srv.URL, Timeout: time.Second, L3Priorities: []string{"high", "normal", "critical"}, LowAction: tc.low, FallbackMode: tc.mode}
			e := New(cfg, discardLogger())
			main := &stubProvider{out: goodVerdict}
			e.provider = main
			in := l1Input("OpenAI 发布模型", "发布")
			if tc.name == "noise" {
				in = l1Input("互粉互关回关必回", "")
			}
			got, err := e.Evaluate(context.Background(), in)
			st := e.Stats()
			if (err != nil) != (tc.name == "fail") || got.TriageStatus != tc.status || int64(main.calls) != tc.calls || st.L3Called != tc.calls || st.L3Kept != tc.calls {
				t.Fatalf("got %+v err=%v stats=%+v", got, err, st)
			}
			if tc.tag != "" && !slices.Contains(got.AITags, tc.tag) {
				t.Fatalf("missing tag %s", tc.tag)
			}
			if (got.AIScore != nil) != (tc.calls == 1) {
				t.Fatal("score presence mismatch")
			}
			if strings.Contains(main.lastUser, "L2 门控初判：priority=high domain=deals") != (tc.name == "pass") {
				t.Fatal("decision hint mismatch")
			}
			var expected PipelineStats
			expected.Enabled = true
			expected.L3Provider = "stub"
			expected.L2Enabled = cfg.Decision.Enabled
			if expected.L2Enabled {
				expected.L2Provider = "http_json"
			}
			expected.L3Called = tc.calls
			expected.L3Kept = tc.calls
			calls := int64(1)
			switch tc.name {
			case "noise":
				expected.L1Filtered = 1
				calls = 0
			case "disabled":
				expected.L2Skipped = 1
				calls = 0
			case "filter":
				expected.L2Filtered = 1
			case "pass":
				expected.L2Passed = 1
			case "low", "low-filter":
				expected.L2Passed = 1
				expected.L2BypassedL3 = 1
			default:
				expected.L2Errors = 1
			}
			if st != expected || hits.Load() != calls {
				t.Fatalf("got stats %+v L2 calls %d, want %+v / %d", st, hits.Load(), expected, calls)
			}
		})
	}
}

func TestDecisionCancellationAndL1Only(t *testing.T) {
	entered := make(chan struct{})
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-done }))
	defer srv.Close()
	defer close(done)
	cfg := l2Config()
	cfg.Decision = DecisionConfig{Enabled: true, Provider: "http_json", BaseURL: srv.URL, Timeout: time.Second}
	in := l1Input("OpenAI 发布模型", "发布")
	for _, mode := range []string{"l1", "disabled", "missing-main-key"} {
		only := cfg
		switch mode {
		case "l1":
			only.Provider = "l1"
		case "disabled":
			only.Enabled = false
		case "missing-main-key":
			only.APIKey = ""
		}
		e := New(only, discardLogger())
		got, err := e.Evaluate(context.Background(), in)
		if err != nil || !slices.Contains(got.AITags, "l1-only") || e.Stats() != (PipelineStats{}) {
			t.Fatalf("bad L1-only state for %s", mode)
		}
	}
	e := New(cfg, discardLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { _, err := e.Evaluate(ctx, in); errCh <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("decision request did not start")
	}
	cancel()
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled gate did not return")
	}
	if s := e.Stats(); s.L2Errors != 0 || s.L3Called != 0 {
		t.Fatal("cancelled gate was counted as failure or called L3")
	}
}

func TestSharedEvaluatorCountsConcurrentCalls(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch hits.Add(1) % 3 {
		case 0:
			fmt.Fprint(w, okCompletionBody(goodVerdict))
		case 1:
			fmt.Fprint(w, okCompletionBody(`{"score":10,"keep":false}`))
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer server.Close()
	cfg := l2Config()
	cfg.BaseURL = server.URL
	e := New(cfg, discardLogger())
	in := l1Input("OpenAI 发布模型", "发布")
	const calls = 18
	errs := make(chan error, calls)
	for range calls {
		go func() { _, err := e.Evaluate(context.Background(), in); errs <- err }()
	}
	for range calls {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	s := e.Stats()
	if hits.Load() != calls || s.L3Called != calls || s.L2Skipped != calls || s.L3Kept != 6 || s.L3Filtered != 6 || s.L3Fallback != 6 {
		t.Fatalf("incorrect concurrent counts: %+v", s)
	}
}
