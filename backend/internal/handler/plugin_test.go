package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"news_panel/internal/config"
	"news_panel/internal/model"
	"news_panel/internal/store"
	"news_panel/internal/triage"
)

type evaluatorFunc func(context.Context, triage.Input) (triage.Result, error)

func (f evaluatorFunc) Evaluate(ctx context.Context, input triage.Input) (triage.Result, error) {
	return f(ctx, input)
}

func pluginRouter(t *testing.T, token string, evaluators ...triage.TriageEvaluator) (http.Handler, *store.Store) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := st.Close(); err != nil {
			t.Error(err)
		}
	})
	h, err := New(st, &config.Config{IngestToken: token}, nil, evaluators...)
	if err != nil {
		t.Fatal(err)
	}
	return h.SetupRouter(), st
}

func pluginPost(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return performRequest(router, "POST", path, mustJSONBody(t, body), map[string]string{"Authorization": "Bearer test-token", "Content-Type": "application/json"})
}

func validIngestItem(id string) map[string]any {
	return map[string]any{"external_id": id, "url": "https://x.com/example/status/" + id, "content": "A useful update", "published_at": "2026-11-15T11:20:00Z"}
}

func ingestBatch(items ...any) map[string]any {
	return map[string]any{"source_id": "x-main", "source_type": "x", "items": items}
}

func heartbeatBody() map[string]any {
	return map[string]any{"source_id": "x-main", "plugin_version": "0.1.0", "status": "ok", "fetched_count": 0, "timestamp": "2026-11-15T11:20:00Z"}
}

func TestPluginBearerAuthFailsClosed(t *testing.T) {
	for _, serverToken := range []string{"test-token", "", "   "} {
		router, _ := pluginRouter(t, serverToken)
		for _, path := range []string{"/api/v1/ingest", "/api/v1/heartbeat"} {
			for _, header := range []string{"", "test-token", "Basic test-token", "Bearer", "Bearer ", "Bearer wrong", "Bearer  test-token", "Bearer test-token extra", "Bearer\ttest-token", "Bearer test-token,"} {
				w := performRequest(router, "POST", path, strings.NewReader("{}"), map[string]string{"Authorization": header})
				if w.Code != 401 {
					t.Fatalf("token set=%v header=%q path=%s: %d", serverToken != "", header, path, w.Code)
				}
			}
			w := performRequest(router, "POST", path, strings.NewReader("{}"), map[string]string{"Authorization": "Bearer test-token"})
			want := 400
			if strings.TrimSpace(serverToken) == "" {
				want = 401
			}
			if w.Code != want {
				t.Fatalf("valid header token configured=%v: %d want %d", serverToken != "", w.Code, want)
			}
		}
		req := httptest.NewRequest("POST", "/api/v1/ingest", strings.NewReader("{}"))
		req.Header.Add("Authorization", "Bearer test-token")
		req.Header.Add("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("multiple authorization headers accepted: %d", w.Code)
		}
	}
}

func TestIngestEnvelopeValidation(t *testing.T) {
	router, _ := pluginRouter(t, "test-token")
	cases := []any{
		nil, []any{}, map[string]any{}, ingestBatch(), ingestBatch(make([]any, 101)...),
		map[string]any{"source_type": "x", "items": []any{validIngestItem("1")}},
		map[string]any{"source_id": "x", "items": []any{validIngestItem("1")}},
		map[string]any{"source_id": strings.Repeat("x", 129), "source_type": "x", "items": []any{validIngestItem("1")}},
		map[string]any{"source_id": "x", "source_type": strings.Repeat("x", 65), "items": []any{validIngestItem("1")}},
		map[string]any{"source_id": "x", "source_type": "x", "items": "bad"},
		map[string]any{"source_id": "x", "source_type": "x", "items": []any{validIngestItem("1")}, "extra": true},
	}
	for i, body := range cases {
		if w := pluginPost(t, router, "/api/v1/ingest", body); w.Code != 400 {
			t.Errorf("case %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	for _, body := range []string{"{", "{} {}", "{} trailing", `{"source_id":null}`} {
		w := performRequest(router, "POST", "/api/v1/ingest", strings.NewReader(body), map[string]string{"Authorization": "Bearer test-token"})
		if w.Code != 400 {
			t.Errorf("invalid JSON %q: %d", body, w.Code)
		}
	}
	large := strings.Repeat(" ", int(MaxPluginPayload)+1)
	for _, path := range []string{"/api/v1/ingest", "/api/v1/heartbeat"} {
		w := performRequest(router, "POST", path, strings.NewReader(large), map[string]string{"Authorization": "Bearer test-token"})
		if w.Code != 413 {
			t.Errorf("oversized %s: %d", path, w.Code)
		}
	}
}

func TestIngestIndividualValidation(t *testing.T) {
	router, _ := pluginRouter(t, "test-token")
	for _, field := range []string{"external_id", "url", "content", "published_at"} {
		item := validIngestItem("bad")
		delete(item, field)
		w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(item))
		var result ingestResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Rejected != 1 || len(result.Errors) != 1 {
			t.Fatalf("missing %s: %d %s", field, w.Code, w.Body.String())
		}
	}
	for _, change := range []struct {
		field string
		value any
	}{
		{"external_id", strings.Repeat("x", 257)}, {"external_id", 3}, {"url", "relative/path"},
		{"published_at", "yesterday"}, {"published_at", "2026-11-15T12:20:00+01:00"},
		{"content", ""}, {"extra", true}, {"raw_meta", []string{}}, {"raw_meta", nil}, {"title", nil},
	} {
		item := validIngestItem("bad")
		item[change.field] = change.value
		w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(item))
		var result ingestResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Rejected != 1 || len(result.Errors) != 1 || result.Errors[0].Reason == "" {
			t.Fatalf("bad %s: %d %s", change.field, w.Code, w.Body.String())
		}
	}
}

func TestIngestDedupAndPassThrough(t *testing.T) {
	calls := 0
	spy := evaluatorFunc(func(ctx context.Context, input triage.Input) (triage.Result, error) {
		calls++
		if input.SourceID != "x-main" || input.SourceType != "x" || input.Author != "@example" || input.RawMeta["likes"] != float64(7) {
			t.Fatalf("lost envelope: %+v", input)
		}
		return (triage.PassThrough{}).Evaluate(ctx, input)
	})
	router, st := pluginRouter(t, "test-token", spy)
	item := validIngestItem("42")
	item["author"] = "@example"
	item["raw_meta"] = map[string]any{"likes": 7}
	w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(item, item, validIngestItem("")))
	var response ingestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || response.Accepted != 1 || response.Deduplicated != 1 || response.Rejected != 1 || len(response.Errors) != 1 || calls != 1 {
		t.Fatalf("%d %s calls=%d", w.Code, w.Body.String(), calls)
	}
	w = pluginPost(t, router, "/api/v1/ingest", ingestBatch(item))
	if !strings.Contains(w.Body.String(), `"deduplicated":1`) || calls != 1 {
		t.Fatalf("replay: %s calls=%d", w.Body.String(), calls)
	}
	items, err := st.ListItems(store.ListItemsParams{})
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v %v", items, err)
	}
	got := items[0]
	if got.TriageStatus != "kept" || got.Priority != "normal" || got.Domain != "other" || got.AIScore != nil || got.AITags == nil || got.Author != "@example" {
		t.Fatalf("unexpected stored item: %+v", got)
	}
	feed, err := st.GetFeed(got.FeedID)
	if err != nil || !feed.Suspended {
		t.Fatalf("source feed must not be polled: %+v %v", feed, err)
	}
	detail := performRequest(router, "GET", fmt.Sprintf("/api/v1/items/%d", got.ID), nil, nil)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), `"ai_tags":[]`) || !strings.Contains(detail.Body.String(), `"likes":7`) {
		t.Fatalf("detail lost metadata: %d %s", detail.Code, detail.Body.String())
	}
}

func TestIngestMaximumBatch(t *testing.T) {
	router, _ := pluginRouter(t, "test-token")
	items := make([]any, 100)
	for i := range items {
		items[i] = validIngestItem(fmt.Sprint(i))
	}
	w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(items...))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"accepted":100`) || !strings.Contains(w.Body.String(), `"errors":[]`) {
		t.Fatalf("max batch: %d %s", w.Code, w.Body.String())
	}
}

func TestTriageInjectionFailureAndFilters(t *testing.T) {
	evaluator := evaluatorFunc(func(_ context.Context, input triage.Input) (triage.Result, error) {
		score := 93
		r := triage.Result{Domain: "ai", AIScore: &score, AITags: []string{"models", "release"}, AISummary: "A new model", Priority: "high", TriageStatus: "kept"}
		if input.ExternalID == "filtered" {
			r.TriageStatus = "filtered"
			r.Priority = "low"
			r.Domain = "other"
			r.AIScore = nil
		}
		if input.ExternalID == "failed" {
			return triage.Result{}, errors.New("service unavailable")
		}
		if input.ExternalID == "invalid" {
			score = 101
		}
		return r, nil
	})
	router, _ := pluginRouter(t, "test-token", evaluator)
	w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(validIngestItem("kept"), validIngestItem("filtered"), validIngestItem("failed"), validIngestItem("invalid")))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"accepted":4`) {
		t.Fatalf("triage: %d %s", w.Code, w.Body.String())
	}
	for _, test := range []struct {
		query string
		count int
	}{
		{"triage_status=kept&domain=ai&priority=high&min_score=90", 1},
		{"triage_status=filtered", 1}, {"triage_status=failed", 2}, {"domain=investing", 0},
		{"min_score=94", 0}, {"min_score=0", 1}, {"priority=low", 1},
		{"domain=%27+OR+1%3D1--", 0},
	} {
		w := performRequest(router, "GET", "/api/items?"+test.query, nil, nil)
		var list struct {
			Data  []model.Item `json:"data"`
			Total int          `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(list.Data) != test.count || list.Total != test.count {
			t.Errorf("%s: %d %s", test.query, w.Code, w.Body.String())
		}
	}
	for _, query := range []string{"min_score=-1", "min_score=101", "min_score=bad", "priority=bad", "triage_status=bad"} {
		if w := performRequest(router, "GET", "/api/v1/items?"+query, nil, nil); w.Code != 400 {
			t.Errorf("%s: %d", query, w.Code)
		}
	}
}

func TestHeartbeatValidationAndUpsert(t *testing.T) {
	router, _ := pluginRouter(t, "test-token")
	for _, field := range []string{"source_id", "plugin_version", "status", "fetched_count", "timestamp"} {
		body := heartbeatBody()
		delete(body, field)
		if w := pluginPost(t, router, "/api/v1/heartbeat", body); w.Code != 400 {
			t.Errorf("missing %s: %d", field, w.Code)
		}
	}
	for _, change := range []struct {
		field string
		value any
	}{
		{"status", "down"}, {"fetched_count", -1}, {"fetched_count", 0.5}, {"fetched_count", nil},
		{"timestamp", "bad"}, {"plugin_version", strings.Repeat("v", 65)}, {"source_id", strings.Repeat("s", 129)},
		{"extra", true}, {"error_message", 42},
	} {
		body := heartbeatBody()
		body[change.field] = change.value
		if w := pluginPost(t, router, "/api/v1/heartbeat", body); w.Code != 400 {
			t.Errorf("invalid %s: %d %s", change.field, w.Code, w.Body.String())
		}
	}
	for _, status := range []string{"ok", "degraded", "error"} {
		body := heartbeatBody()
		body["status"] = status
		body["error_message"] = "collection unavailable"
		w := pluginPost(t, router, "/api/v1/heartbeat", body)
		var response struct {
			Status     string `json:"status"`
			ServerTime string `json:"server_time"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(time.RFC3339, response.ServerTime); err != nil || w.Code != 200 || response.Status != "ok" {
			t.Fatalf("heartbeat: %d %s", w.Code, w.Body.String())
		}
	}
	body := heartbeatBody()
	body["error_message"] = nil
	if w := pluginPost(t, router, "/api/v1/heartbeat", body); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w := performRequest(router, "GET", "/api/v1/plugins/heartbeats", nil, nil)
	var response struct {
		Data  []store.PluginHeartbeat `json:"data"`
		Total int                     `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || response.Total != 1 || len(response.Data) != 1 || response.Data[0].Status != "ok" || response.Data[0].FetchedCount != 0 || response.Data[0].ErrorMessage != nil || response.Data[0].UpdatedAt == "" {
		t.Fatalf("upsert: %d %s", w.Code, w.Body.String())
	}
}

func TestPluginReadsUseReaderAuthentication(t *testing.T) {
	h, err := New(nil, &config.Config{Password: "reader-password", IngestToken: "test-token"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := h.SetupRouter()
	for _, path := range []string{"/api/items", "/api/v1/items", "/api/v1/plugins/heartbeats"} {
		w := performRequest(router, "GET", path, nil, map[string]string{"Authorization": "Bearer test-token"})
		if w.Code != 401 {
			t.Errorf("plugin token bypassed reader auth on %s: %d", path, w.Code)
		}
	}
}

func TestPluginExactSchemaKeys(t *testing.T) {
	router, _ := pluginRouter(t, "test-token")
	top := ingestBatch(validIngestItem("1"))
	top["SOURCE_ID"] = top["source_id"]
	delete(top, "source_id")
	if w := pluginPost(t, router, "/api/v1/ingest", top); w.Code != 400 {
		t.Fatalf("case-insensitive top-level key: %d", w.Code)
	}
	hb := heartbeatBody()
	hb["STATUS"] = hb["status"]
	delete(hb, "status")
	if w := pluginPost(t, router, "/api/v1/heartbeat", hb); w.Code != 400 {
		t.Fatalf("case-insensitive heartbeat key: %d", w.Code)
	}
	item := validIngestItem("1")
	item["Content"] = item["content"]
	delete(item, "content")
	w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(item))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"rejected":1`) {
		t.Fatalf("case-insensitive item key: %d %s", w.Code, w.Body.String())
	}
}

func TestRetryRecoversInterruptedEvaluation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	evaluator := evaluatorFunc(func(evalCtx context.Context, input triage.Input) (triage.Result, error) {
		calls++
		if input.Content != "A useful update" || input.PublishedAt.Nanosecond() != 123000000 {
			t.Fatalf("retry changed original envelope: %+v", input)
		}
		r, err := (triage.PassThrough{}).Evaluate(evalCtx, input)
		if calls == 1 {
			cancel()
		}
		return r, err
	})
	router, st := pluginRouter(t, "test-token", evaluator)
	original := validIngestItem("recover")
	original["published_at"] = "2026-11-15T11:20:00.123Z"
	req := httptest.NewRequest("POST", "/api/v1/ingest", mustJSONBody(t, ingestBatch(original))).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("expected interrupted writeback: %d %s", w.Code, w.Body.String())
	}
	items, err := st.ListItems(store.ListItemsParams{})
	if err != nil || len(items) != 1 || items[0].TriageStatus != "pending" {
		t.Fatalf("pending reservation missing: %+v %v", items, err)
	}
	modified := validIngestItem("recover")
	modified["content"] = "Must not overwrite original"
	w = pluginPost(t, router, "/api/v1/ingest", ingestBatch(modified))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"deduplicated":1`) || !strings.Contains(w.Body.String(), `"accepted":0`) {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	item, err := st.GetItem(items[0].ID)
	if err != nil || item.TriageStatus != "kept" || calls != 2 {
		t.Fatalf("recovery: %+v %v calls=%d", item, err, calls)
	}
}

func TestBookmarkTriageMetadataAndFilters(t *testing.T) {
	score := 92
	evaluator := evaluatorFunc(func(_ context.Context, input triage.Input) (triage.Result, error) {
		return triage.Result{Domain: "ai", AIScore: &score, AITags: []string{"release"}, AISummary: "A model update", Priority: "high", TriageStatus: "kept"}, nil
	})
	router, st := pluginRouter(t, "test-token", evaluator)
	if w := pluginPost(t, router, "/api/v1/ingest", ingestBatch(validIngestItem("saved"))); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	items, err := st.ListItems(store.ListItemsParams{})
	if err != nil || len(items) != 1 {
		t.Fatal(err)
	}
	item := items[0]
	if _, err := st.CreateBookmark(&item.ID, &item.FeedID, item.Link, item.Title, item.Content, item.PubDate, "x-main"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateBookmark(nil, nil, "https://example.org/orphan", "Orphan", "saved snapshot", 1, ""); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"triage_status=kept", "domain=ai&priority=high&min_score=90"} {
		w := performRequest(router, "GET", "/api/bookmarks?"+query, nil, nil)
		var result struct {
			Data  []model.Bookmark `json:"data"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || result.Total != 1 || len(result.Data) != 1 || result.Data[0].Triage == nil || result.Data[0].Triage.AIScore == nil || *result.Data[0].Triage.AIScore != 92 || result.Data[0].SourceID != "x-main" {
			t.Fatalf("bookmark %s: %d %s", query, w.Code, w.Body.String())
		}
	}
	w := performRequest(router, "GET", "/api/bookmarks?min_score=93", nil, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":0`) {
		t.Fatalf("bookmark score filter: %d %s", w.Code, w.Body.String())
	}
	w = performRequest(router, "GET", "/api/bookmarks", nil, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":2`) || !strings.Contains(w.Body.String(), `"triage":null`) {
		t.Fatalf("orphan snapshot: %d %s", w.Code, w.Body.String())
	}
}
