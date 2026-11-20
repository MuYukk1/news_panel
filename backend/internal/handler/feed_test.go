package handler

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/0x2E/feedfinder"
	"news_panel/internal/model"
)

func TestNormalizeDiscoveredFeedsNormalizesTitles(t *testing.T) {
	found := []feedfinder.Feed{
		{Title: "  Blog =&gt; Notes  ", Link: "http://example.com/a.xml"},
		{Title: "AT&T Feed", Link: "http://example.com/b.xml"},
		{Title: "std::vector&lt;int&gt;", Link: "http://example.com/c.xml"},
	}

	got := normalizeDiscoveredFeeds(found)

	want := []string{
		"Blog => Notes",
		"AT&T Feed",
		"std::vector<int>",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d feeds, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Title != w {
			t.Fatalf("feed %d title = %q, want %q", i, got[i].Title, w)
		}
	}
}

func TestUpdateFeedAIEnabledAndSuspended(t *testing.T) {
	router, st := pluginRouter(t, "test-token")
	feed, err := st.CreateFeed(1, "Tech Feed", "https://example.org/tech.xml", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if feed.AIEnabled {
		t.Fatalf("expected default ai_enabled=false")
	}

	w := performRequest(
		router,
		"PATCH",
		fmt.Sprintf("/api/feeds/%d", feed.ID),
		strings.NewReader(`{"ai_enabled":true,"suspended":true}`),
		map[string]string{"Content-Type": "application/json"},
	)
	if w.Code != 200 {
		t.Fatalf("patch feed: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data model.Feed `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Data.AIEnabled || !resp.Data.Suspended {
		t.Fatalf("expected ai_enabled=true and suspended=true, got %+v", resp.Data)
	}
}
