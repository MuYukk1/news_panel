package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"news_panel/internal/ai"
	"news_panel/internal/config"
	"news_panel/internal/pull"
	"news_panel/internal/store"
)

const aiSettingsPath = "/api/v1/ai/settings"

type aiFixture struct {
	handler http.Handler
	eval    *ai.Evaluator
	puller  *pull.Puller
	store   *store.Store
	cookie  *http.Cookie
}

func newAIFixture(t *testing.T, private bool) aiFixture {
	t.Helper()
	t.Setenv("AI_PROVIDER", "cli")
	t.Setenv("AI_CLI_COMMAND", "private-cli-command")
	t.Setenv("AI_ENABLED", "true")
	t.Setenv("AI_DECISION_ENABLED", "true")
	t.Setenv("AI_DECISION_PROVIDER", "openai")
	t.Setenv("AI_DECISION_API_KEY", "secret-env-token")
	t.Setenv("AI_DECISION_BASE_URL", "")
	t.Setenv("AI_ALLOW_PRIVATE_ENDPOINTS", fmt.Sprint(private))
	t.Setenv("AI_RSS_MAX_ITEMS_PER_PULL", "13")
	db, err := store.New(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := &config.Config{Password: "reader", IngestToken: "plugin"}
	eval := ai.NewFromEnv()
	puller := pull.New(db, cfg)
	puller.SetEvaluator(eval)
	h, err := New(db, cfg, puller, eval)
	if err != nil {
		t.Fatal(err)
	}
	h.sessions["reader-cookie"] = time.Now().Add(time.Hour).Unix()
	return aiFixture{h.SetupRouter(), eval, puller, db, &http.Cookie{Name: "session", Value: "reader-cookie"}}
}
func (f aiFixture) request(t *testing.T, method, path, body string, want int) string {
	t.Helper()
	w := performRequest(f.handler, method, path, strings.NewReader(body), nil, f.cookie)
	if w.Code != want {
		t.Fatalf("%s %s: status %d, expected %d: %s", method, path, w.Code, want, w.Body.String())
	}
	for _, secret := range []string{"secret-env-token", "secret-db-token", "private-cli-command", "private-cli-arg"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("response exposed credentials or command")
		}
	}
	return w.Body.String()
}
func TestAISettingsReaderAndReload(t *testing.T) {
	f := newAIFixture(t, false)
	for _, route := range []struct{ method, path string }{{"GET", aiSettingsPath}, {"PATCH", aiSettingsPath}, {"POST", aiSettingsPath + "/test"}} {
		w := performRequest(f.handler, route.method, route.path, nil, map[string]string{"Authorization": "Bearer plugin"})
		if w.Code != 401 {
			t.Fatalf("plugin accessed %s", route.path)
		}
	}
	view := f.request(t, "GET", aiSettingsPath, "", 200)
	if !strings.Contains(view, `"decision_api_key_source":"env"`) || !strings.Contains(view, "••••oken") {
		t.Fatal("incorrect ENV key status")
	}
	view = f.request(t, "PATCH", aiSettingsPath, `{"decision_provider":"gemini_decision","decision_api_key":"secret-db-token","rss_max_items_per_pull":0,"topics":"breaking news"}`, 200)
	if f.puller.RSSLimit() != 0 || f.eval.Stats().L2Provider != "gemini_decision" || !strings.Contains(view, `"decision_api_key_source":"db"`) || !strings.Contains(view, `"decision_model":"gemini-decision-1"`) {
		t.Fatal("save did not update pipeline or select preset defaults")
	}
	f.request(t, "PATCH", aiSettingsPath, `{"decision_api_key":"","decision_enabled":false}`, 200)
	row, err := f.store.GetAISettings()
	if err != nil || row.DecisionAPIKey == nil || *row.DecisionAPIKey != "secret-db-token" || f.eval.Stats().L2Enabled {
		t.Fatal("blank key or explicit false mishandled")
	}
	view = f.request(t, "PATCH", aiSettingsPath, `{"clear_decision_api_key":true}`, 200)
	if !strings.Contains(view, `"decision_api_key_source":"env"`) {
		t.Fatal("clear did not inherit ENV")
	}
	f.request(t, "PATCH", aiSettingsPath, `{"reset_to_env":true}`, 200)
	if f.puller.RSSLimit() != 13 || f.eval.Stats().L2Provider != "openai" {
		t.Fatal("reset did not restore environment")
	}
}
func TestAISettingsValidation(t *testing.T) {
	f := newAIFixture(t, false)
	invalid := []string{`{"decision_provider":"unknown"}`, `{"decision_timeout_seconds":61}`, `{"rss_max_items_per_pull":-1}`, `{"decision_l3_priorities":"middle"}`, `{"decision_low_action":"bad"}`, `{"decision_fallback_mode":"bad"}`, `{"AI_CLI_COMMAND":"private-cli-command"}`, `{"AI_CLI_ARGS":"private-cli-arg"}`, `{"decision_auth_header":"HoSt"}`, `{"decision_auth_header":"Cookie"}`, `{"decision_auth_header":"X-Test\r\nOther"}`, `{"decision_provider":"cloudflare_clef","decision_account_id":"invalid/id"}`}
	for _, base := range []string{"http://127.0.0.1", "http://localhost", "http://169.254.169.254", "file:///invalid"} {
		b, _ := json.Marshal(map[string]string{"decision_base_url": base})
		invalid = append(invalid, string(b))
	}
	for _, body := range invalid {
		f.request(t, "PATCH", aiSettingsPath, body, 400)
		f.request(t, "POST", aiSettingsPath+"/test", body, 400)
	}
	if f.puller.RSSLimit() != 13 || f.eval.Stats().L2Provider != "openai" {
		t.Fatal("invalid draft changed live settings")
	}
}
func TestAISettingsDraftProbe(t *testing.T) {
	f := newAIFixture(t, true)
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Test") != "secret-db-token" {
			t.Error("probe did not use saved key")
		}
		fmt.Fprint(w, `{"action":"pass","priority":"high","domain":"ai","reason":"secret-db-token"}`)
	}))
	defer s.Close()
	b, _ := json.Marshal(map[string]string{"decision_base_url": s.URL, "decision_provider": "http_json", "decision_api_key": "secret-db-token", "decision_auth_header": "X-Test"})
	f.request(t, "PATCH", aiSettingsPath, string(b), 200)
	response := f.request(t, "POST", aiSettingsPath+"/test", `{"topics":"probe only"}`, 200)
	if !strings.Contains(response, `"ok":true`) || !strings.Contains(response, `"action":"pass"`) {
		t.Fatal("missing probe outcome")
	}
	f.request(t, "POST", aiSettingsPath+"/test", "", 429)
	row, err := f.store.GetAISettings()
	if err != nil || calls != 1 || row.Topics != nil || f.eval.Stats().L2Passed != 0 {
		t.Fatal("probe persisted draft, bypassed cooldown or changed counters")
	}
}
