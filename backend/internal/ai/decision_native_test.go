package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"news_panel/internal/triage"
	"strings"
	"testing"
	"time"
)

func TestNativeDecisionWire(t *testing.T) {
	for _, provider := range []string{"gemini_decision", "openai_decision", "cloudflare_clef", "typesafe"} {
		t.Run(provider, func(t *testing.T) {
			p, _ := decisionPreset(provider)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := "Bearer fixture-key"
				if provider == "gemini_decision" {
					key = "fixture-key"
					if r.Header.Get("Authorization") != "" {
						t.Error("unexpected bearer header")
					}
				}
				if r.Method != "POST" || r.Header.Get(p.AuthHeaderHint) != key || r.URL.RawQuery != "" {
					t.Error("invalid native authentication or method")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				state := "state"
				if provider == "gemini_decision" || provider == "openai_decision" {
					state = "input"
				}
				if !strings.Contains(string(body[state]), "OpenAI") || string(body["model"]) != fmt.Sprintf("%q", p.DefaultModel) {
					t.Error("missing model/input")
				}
				if provider == "openai_decision" {
					var qs []namedQuestion
					if json.Unmarshal(body["questions"], &qs) != nil || len(qs) != 3 || qs[0].Name != "action" || len(qs[0].Choices) != 2 {
						t.Error("invalid array questions")
					}
					fmt.Fprint(w, `{"answers":[{"name":"domain","choice":"ai","probabilities":[1]},{"name":"action","value":"pass","confidence":0.96,"probabilities":{"pass":1}},{"name":"priority","choice":"high"}]}`)
					return
				}
				var qs map[string]decisionQuestion
				if json.Unmarshal(body["questions"], &qs) != nil || len(qs) != 3 || qs["priority"].Criteria["high"] == "" || !strings.Contains(qs["action"].Instructions, "custom topic") {
					t.Error("invalid map questions")
				}
				answers := `{"answers":{"action":{"choice":"pass","confidence":0.96},"priority":{"value":"high"},"domain":{"selected":"ai"}}}`
				if provider == "cloudflare_clef" {
					if r.URL.Path != "/accounts/account_1/ai/run/"+p.DefaultModel {
						t.Error("unexpected Cloudflare run path")
					}
					fmt.Fprintf(w, `{"success":true,"result":%s}`, answers)
				} else {
					fmt.Fprint(w, answers)
				}
			}))
			defer srv.Close()
			c := Config{Topics: "custom topic", Decision: DecisionConfig{Provider: provider, Model: p.DefaultModel, BaseURL: srv.URL, APIKey: "fixture-key", AccountID: "account_1", AuthHeader: "Authorization", AllowPrivateEndpoints: true, Timeout: time.Second}}
			res := TestDecisionConfig(context.Background(), c)
			if !res.OK || res.Outcome.Action != "pass" || res.Outcome.Priority != "high" || res.Outcome.Domain != "ai" || !strings.Contains(res.Outcome.Reason, "0.96") {
				t.Errorf("unexpected decision %+v", res)
			}
		})
	}
}

func TestCloudflareValidationAndFailure(t *testing.T) {
	d := DecisionConfig{Provider: "cloudflare_clef", BaseURL: cloudflareBase, Model: "@cf/cloudflare/clef-flash", AccountID: "a/b", AuthHeader: "Authorization"}
	if _, err := d.endpoint(); err == nil {
		t.Fatal("invalid account accepted")
	}
	d.AccountID = "abc"
	d.Model = "../model"
	if _, err := d.endpoint(); err == nil {
		t.Fatal("invalid model accepted")
	}
	d.BaseURL = "https://example.test/accounts/abc/ai/run/custom"
	d.AccountID = ""
	d.Model = "custom"
	if url, err := d.endpoint(); err != nil || url != d.BaseURL {
		t.Fatal("full run URL rejected")
	}
	preset, _ := decisionPreset("cloudflare_clef")
	if _, err := parseNativeAnswers([]byte(`{"success":false,"errors":["private credential"]}`), preset, "other", ""); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unexpected provider error")
	}
}
func TestL2NoRedirect(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/second" {
			calls++
			return
		}
		http.Redirect(w, r, "/second", http.StatusFound)
	}))
	defer s.Close()
	c := Config{Decision: DecisionConfig{Enabled: true, Provider: "http_json", BaseURL: s.URL, AuthHeader: "X-Test", APIKey: "fixture", Timeout: time.Second, AllowPrivateEndpoints: true}}
	if _, err := newDecisionProvider(c).Decide(context.Background(), triage.Input{}, triage.Result{}); err == nil || calls != 0 {
		t.Fatal("followed redirect or accepted response")
	}
}
func TestReloadKeepsCLIAndCounters(t *testing.T) {
	c := l2Config()
	c.Provider = "cli"
	c.CLICommand = "unused"
	c.CLICooldown = time.Minute
	e := New(c, discardLogger())
	before := e.state.Load()
	_, err := e.Evaluate(context.Background(), l1Input("互粉互关回关必回", ""))
	if err != nil {
		t.Fatal(err)
	}
	c.Topics = "updated"
	e.Reload(c)
	after := e.state.Load()
	if before == after || before.provider != after.provider || after.cfg.Topics != c.Topics || e.EnvConfig().Topics == c.Topics || e.Stats().L1Filtered != 1 {
		t.Fatal("reload lost runtime state or changed baseline")
	}
}
