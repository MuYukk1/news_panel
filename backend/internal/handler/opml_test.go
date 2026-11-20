package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"news_panel/internal/config"
)

func TestRewriteGitHubBlobURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantURL string
		wantOK  bool
	}{
		{
			name:    "github blob opml link",
			input:   "https://github.com/xiangyugongzuoliu/awesome-rss-feeds-list/blob/main/feeds/cn-ai-research.opml",
			wantURL: "https://raw.githubusercontent.com/xiangyugongzuoliu/awesome-rss-feeds-list/main/feeds/cn-ai-research.opml",
			wantOK:  true,
		},
		{
			name:    "github blob with query and nested path",
			input:   "https://github.com/owner/repo/blob/v1.0/a/b/feeds.xml?raw=1#L1",
			wantURL: "https://raw.githubusercontent.com/owner/repo/v1.0/a/b/feeds.xml",
			wantOK:  true,
		},
		{
			name:   "non-blob github url",
			input:  "https://github.com/owner/repo/tree/main/feeds",
			wantOK: false,
		},
		{
			name:   "already raw github url",
			input:  "https://raw.githubusercontent.com/owner/repo/main/feeds.opml",
			wantOK: false,
		},
		{
			name:   "non-github url",
			input:  "https://example.com/owner/repo/blob/main/feeds.opml",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rewriteGitHubBlobURL(tc.input)
			if ok != tc.wantOK || got != tc.wantURL {
				t.Fatalf("rewriteGitHubBlobURL(%q) = (%q, %v), want (%q, %v)", tc.input, got, ok, tc.wantURL, tc.wantOK)
			}
		})
	}
}

func TestParseOPMLDocumentNestedGroupsAndDedup(t *testing.T) {
	payload := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>AI Research</title></head>
  <body>
    <outline text="AI Labs" title="AI Labs">
      <outline type="rss" text="OpenAI Blog" title="OpenAI &amp; Research" xmlUrl="https://openai.com/news/rss.xml" htmlUrl="https://openai.com/news" />
      <outline type="rss" text="Duplicate OpenAI" xmlUrl="https://openai.com/news/rss.xml" />
      <outline type="rss" text="Anthropic" xmlurl="https://anthropic.com/rss.xml" htmlurl="https://anthropic.com" />
    </outline>
    <outline type="rss" text="Ungrouped Feed" xmlUrl="https://example.com/feed.xml" />
  </body>
</opml>`

	feeds, isOPML := parseOPMLDocument([]byte(payload))
	if !isOPML {
		t.Fatal("expected valid OPML document")
	}
	if len(feeds) != 3 {
		t.Fatalf("expected 3 deduplicated feeds, got %d: %+v", len(feeds), feeds)
	}
	if feeds[0].Title != "OpenAI & Research" || feeds[0].Link != "https://openai.com/news/rss.xml" || feeds[0].SiteURL != "https://openai.com/news" || feeds[0].GroupName != "AI Labs" {
		t.Fatalf("unexpected first feed: %+v", feeds[0])
	}
	if feeds[1].Title != "Anthropic" || feeds[1].SiteURL != "https://anthropic.com" || feeds[1].GroupName != "AI Labs" {
		t.Fatalf("unexpected second feed: %+v", feeds[1])
	}
	if feeds[2].Title != "Ungrouped Feed" || feeds[2].GroupName != "" {
		t.Fatalf("unexpected third feed: %+v", feeds[2])
	}

	if _, ok := parseOPMLDocument([]byte(`<rss version="2.0"><channel><title>Not OPML</title></channel></rss>`)); ok {
		t.Fatal("RSS document must not be classified as OPML")
	}
}

func TestValidateFeedRemoteOPMLMultiFeedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <body>
    <outline text="CN AI">
      <outline text="Paper Weekly" xmlUrl="https://example.com/ai1.xml" htmlUrl="https://example.com/ai1" />
      <outline text="LLM Watch" xmlUrl="https://example.com/ai2.xml" htmlUrl="https://example.com/ai2" />
    </outline>
  </body>
</opml>`)
	}))
	defer server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}
	r := newTestRouter()
	r.POST("/api/feeds/validate", h.validateFeed)

	w := performRequest(
		r,
		http.MethodPost,
		"/api/feeds/validate",
		strings.NewReader(fmt.Sprintf(`{"url":%q}`, server.URL+"/cn-ai-research.opml")),
		map[string]string{"Content-Type": "application/json"},
	)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Feeds []discoveredFeed `json:"feeds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data.Feeds) != 2 {
		t.Fatalf("expected 2 discovered OPML feeds, got %d: %+v", len(resp.Data.Feeds), resp.Data.Feeds)
	}
	if resp.Data.Feeds[0].GroupName != "CN AI" || resp.Data.Feeds[0].SiteURL != "https://example.com/ai1" {
		t.Fatalf("unexpected feed metadata: %+v", resp.Data.Feeds[0])
	}
}

func TestValidateFeedRejectsPrivateOPMLWhenDisallowed(t *testing.T) {
	h := &Handler{config: &config.Config{AllowPrivateFeeds: false}}
	code, _ := postValidateFeed(t, h, "http://127.0.0.1:8080/feeds.opml")
	if code != http.StatusBadRequest {
		t.Fatalf("expected SSRF rejection 400 for loopback OPML URL, got %d", code)
	}
}
