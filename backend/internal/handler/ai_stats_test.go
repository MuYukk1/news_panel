package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"news_panel/internal/ai"
	"news_panel/internal/config"
	"news_panel/internal/triage"
	"strings"
	"testing"
	"time"
)

func TestAIStatsReaderSession(t *testing.T) {
	eval := ai.New(ai.Config{Provider: "l1", APIKey: "secret-main", Decision: ai.DecisionConfig{APIKey: "secret-gate", BaseURL: "https://secret-url.test"}}, nil)
	if _, err := eval.Evaluate(context.Background(), triage.Input{Content: "互粉互关回关必回"}); err != nil {
		t.Fatal(err)
	}
	h, err := New(nil, &config.Config{Password: "reader", IngestToken: "plugin-token"}, nil, eval)
	if err != nil {
		t.Fatal(err)
	}
	h.sessions["reader-session"] = time.Now().Add(time.Hour).Unix()
	router := h.SetupRouter()
	const path = "/api/v1/ai/stats"
	for _, headers := range []map[string]string{nil, {"Authorization": "Bearer plugin-token"}} {
		if w := performRequest(router, "GET", path, nil, headers); w.Code != 401 {
			t.Fatalf("missing reader session: %d", w.Code)
		}
	}
	w := performRequest(router, "GET", path, nil, nil, &http.Cookie{Name: "session", Value: "reader-session"})
	var body struct {
		Data ai.PipelineStats `json:"data"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Data != eval.Stats() || body.Data.L1Filtered != 1 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("incorrect stats: %s", w.Body.String())
	}
	h.evaluator = triage.PassThrough{}
	w = performRequest(router, "GET", path, nil, nil, &http.Cookie{Name: "session", Value: "reader-session"})
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Data != (ai.PipelineStats{}) {
		t.Fatal("no stats getter should return zero snapshot")
	}
}
