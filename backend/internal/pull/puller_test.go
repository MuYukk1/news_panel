package pull

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"news_panel/internal/triage"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"news_panel/internal/config"
	"news_panel/internal/store"
)

func TestRefreshFeedPreservesValidatorsWhen304OmitHeaders(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Header().Set("ETag", `"etag-v1"`)
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
			w.Header().Set("Cache-Control", "max-age=86400")
			w.Header().Set("Expires", "Tue, 03 Jan 2006 15:04:05 GMT")
			_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Demo</title><link>https://example.com</link>
<item><guid>g1</guid><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`)
			return
		}

		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	feed, err := st.CreateFeed(1, "Feed A", server.URL, "", "")
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	p := New(st, &config.Config{
		PullInterval:      1800,
		PullTimeout:       5,
		PullConcurrency:   1,
		PullMaxBackoff:    604800,
		AllowPrivateFeeds: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.RefreshFeed(ctx, feed.ID); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	if err := p.RefreshFeed(ctx, feed.ID); err != nil {
		t.Fatalf("second refresh: %v", err)
	}

	updatedFeed, err := st.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}

	if updatedFeed.FetchState.ETag != `"etag-v1"` {
		t.Fatalf("etag = %q, want %q", updatedFeed.FetchState.ETag, `"etag-v1"`)
	}
	if updatedFeed.FetchState.LastModified != "Mon, 02 Jan 2006 15:04:05 GMT" {
		t.Fatalf("last_modified = %q, want %q", updatedFeed.FetchState.LastModified, "Mon, 02 Jan 2006 15:04:05 GMT")
	}
	if updatedFeed.FetchState.CacheControl != "max-age=86400" {
		t.Fatalf("cache_control = %q, want %q", updatedFeed.FetchState.CacheControl, "max-age=86400")
	}

	expires, err := http.ParseTime("Tue, 03 Jan 2006 15:04:05 GMT")
	if err != nil {
		t.Fatalf("parse expires: %v", err)
	}
	if updatedFeed.FetchState.ExpiresAt != expires.Unix() {
		t.Fatalf("expires_at = %d, want %d", updatedFeed.FetchState.ExpiresAt, expires.Unix())
	}
}

func TestRefreshAllWaitsForRunningJobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Demo</title><link>https://example.com</link>
<item><guid>%s</guid><title>Item</title><link>https://example.com%s</link></item>
</channel></rss>`, r.URL.Path, r.URL.Path)
	}))
	defer server.Close()

	if _, err := st.CreateFeed(1, "Feed A", server.URL+"/a", "", ""); err != nil {
		t.Fatalf("create feed A: %v", err)
	}
	if _, err := st.CreateFeed(1, "Feed B", server.URL+"/b", "", ""); err != nil {
		t.Fatalf("create feed B: %v", err)
	}

	p := New(st, &config.Config{
		PullInterval:      1800,
		PullTimeout:       5,
		PullConcurrency:   1,
		PullMaxBackoff:    604800,
		AllowPrivateFeeds: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := p.RefreshAll(ctx)
	if err != nil {
		t.Fatalf("refresh all: %v", err)
	}
	if count != 2 {
		t.Fatalf("refresh count = %d, want 2", count)
	}

	feeds, err := st.ListFeeds()
	if err != nil {
		t.Fatalf("list feeds: %v", err)
	}
	for _, feed := range feeds {
		if feed.FetchState.LastSuccessAt <= 0 {
			t.Fatalf("feed %d last_success_at = %d, want > 0", feed.ID, feed.FetchState.LastSuccessAt)
		}
	}
}

func TestRefreshFeedBackfillsNameFromTitle(t *testing.T) {
	const rssWithTitle = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Tech Daily</title><link>https://example.com</link>
<item><guid>g1</guid><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`
	const rssWithoutTitle = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><link>https://example.com</link>
<item><guid>g1</guid><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`

	tests := []struct {
		name       string
		feedName   string
		serverBody string
		wantName   string
	}{
		{
			// Feeds added by URL default their name to the link; a successful
			// pull must replace it with the feed's own channel title.
			name:       "name equal to link is replaced by title",
			feedName:   "", // replaced with server.URL below
			serverBody: rssWithTitle,
			wantName:   "Tech Daily",
		},
		{
			name:       "custom name is kept",
			feedName:   "My Custom Name",
			serverBody: rssWithTitle,
			wantName:   "My Custom Name",
		},
		{
			name:       "feed without title keeps default name",
			feedName:   "", // replaced with server.URL below
			serverBody: rssWithoutTitle,
			wantName:   "", // replaced with server.URL below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/rss+xml")
				_, _ = fmt.Fprint(w, tt.serverBody)
			}))
			defer server.Close()

			feedName := tt.feedName
			if feedName == "" {
				feedName = server.URL
			}
			wantName := tt.wantName
			if wantName == "" {
				wantName = server.URL
			}

			dbPath := filepath.Join(t.TempDir(), "test.db")
			st, err := store.New(dbPath)
			if err != nil {
				t.Fatalf("create store: %v", err)
			}
			defer st.Close()

			feed, err := st.CreateFeed(1, feedName, server.URL, "", "")
			if err != nil {
				t.Fatalf("create feed: %v", err)
			}

			p := New(st, &config.Config{
				PullInterval:      1800,
				PullTimeout:       5,
				PullConcurrency:   1,
				PullMaxBackoff:    604800,
				AllowPrivateFeeds: true,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := p.RefreshFeed(ctx, feed.ID); err != nil {
				t.Fatalf("refresh feed: %v", err)
			}

			updated, err := st.GetFeed(feed.ID)
			if err != nil {
				t.Fatalf("get feed: %v", err)
			}
			if updated.Name != wantName {
				t.Fatalf("feed name = %q, want %q", updated.Name, wantName)
			}
		})
	}
}

func TestPullerSkipsPluginShadowFeedsEvenWhenUnsuspended(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	shadow, err := st.CreateFeed(1, "x-twifork-main", "urn:news_panel:source:x-twifork-main", "", "")
	if err != nil {
		t.Fatalf("create shadow feed: %v", err)
	}
	suspended := false
	if err := st.UpdateFeed(shadow.ID, store.UpdateFeedParams{Suspended: &suspended}); err != nil {
		t.Fatalf("unsuspend shadow feed: %v", err)
	}

	p := New(st, &config.Config{
		PullInterval:      1800,
		PullTimeout:       5,
		PullConcurrency:   1,
		PullMaxBackoff:    604800,
		AllowPrivateFeeds: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.pullAll(ctx)
	count, err := p.RefreshAll(ctx)
	if err != nil {
		t.Fatalf("refresh all: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 shadow feeds pulled in RefreshAll, got %d", count)
	}
	if err := p.RefreshFeed(ctx, shadow.ID); err != nil {
		t.Fatalf("refresh shadow feed: %v", err)
	}

	after, err := st.GetFeed(shadow.ID)
	if err != nil {
		t.Fatalf("get shadow feed: %v", err)
	}
	if after.FetchState.LastCheckedAt != 0 || after.FetchState.LastError != "" {
		t.Fatalf("expected shadow feed to remain untouched by puller, got %+v", after.FetchState)
	}
}

type rssEvalFunc func(context.Context, triage.Input) (triage.Result, error)

func (f rssEvalFunc) Evaluate(ctx context.Context, in triage.Input) (triage.Result, error) {
	return f(ctx, in)
}

func TestRSSBudgetCountsOnlyNewItems(t *testing.T) {
	t.Setenv("AI_RSS_MAX_ITEMS_PER_PULL", "3")
	st, err := store.New(filepath.Join(t.TempDir(), "rss.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var round atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprint(w, `<rss version="2.0"><channel><title>Feed</title><link>https://example.test</link>`)
		item := func(id int) {
			fmt.Fprintf(w, `<item><guid>g%d</guid><title>title</title><link>https://example.test/%d</link><description>body</description><pubDate>Mon, 02 Jan 2006 15:04:%02d GMT</pubDate></item>`, id, id, id)
		}
		item(5) // already present before this source opts in
		if round.Load() > 0 {
			for _, i := range []int{1, 2, 4, 3, 4} {
				item(i)
			}
		}
		fmt.Fprint(w, `</channel></rss>`)
	}))
	defer srv.Close()
	feed, err := st.CreateFeed(1, "Feed", srv.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	p := New(st, &config.Config{PullInterval: 1800, PullTimeout: 5, PullConcurrency: 1, PullMaxBackoff: 604800, AllowPrivateFeeds: true})
	var seen []string
	p.SetEvaluator(rssEvalFunc(func(ctx context.Context, in triage.Input) (triage.Result, error) {
		seen = append(seen, in.ExternalID)
		if in.SourceType != "rss" || in.SourceID == "" || in.Title != "title" || in.Author != "Feed" || in.Content != "body" || in.URL == "" || in.PublishedAt.Location() != time.UTC {
			t.Errorf("incomplete RSS input %+v", in)
		}
		switch in.ExternalID {
		case "g4":
			return triage.Result{}, fmt.Errorf("temporary model failure")
		case "g3":
			return triage.Result{}, nil // invalid result
		}
		score := 80
		return triage.Result{Domain: "ai", Priority: "high", TriageStatus: "kept", AIScore: &score}, nil
	}))
	refresh := func() {
		t.Helper()
		if err := p.RefreshFeed(context.Background(), feed.ID); err != nil {
			t.Fatal(err)
		}
	}
	refresh()
	if len(seen) != 0 {
		t.Fatal("RSS AI ran before opt-in")
	}
	enabled := true
	if err := st.UpdateFeed(feed.ID, store.UpdateFeedParams{AIEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}
	round.Store(1)
	refresh()
	refresh()
	if !slices.Equal(seen, []string{"g4", "g3", "g2"}) {
		t.Fatalf("evaluated %v", seen)
	}
	saved, err := st.ListItems(store.ListItemsParams{FeedID: &feed.ID})
	if err != nil || len(saved) != 5 {
		t.Fatalf("stored=%d err=%v", len(saved), err)
	}
	for _, item := range saved {
		want := "kept"
		if item.GUID == "g4" || item.GUID == "g3" {
			want = "failed"
		}
		if item.TriageStatus != want || (item.AIScore != nil) != (item.GUID == "g2") {
			t.Errorf("incorrect result for %s: %+v", item.GUID, item.Result)
		}
	}
	updated, err := st.GetFeed(feed.ID)
	if err != nil || updated.FetchState.LastSuccessAt == 0 || updated.FetchState.ConsecutiveFailures != 0 {
		t.Fatal("evaluation error altered fetch status")
	}
}

func TestRefreshFeedDecouplesFetchTimeoutFromTriage(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Fast RSS</title><link>https://example.com</link>
<item><guid>g1</guid><title>Item 1</title><link>https://example.com/1</link><description>body 1</description></item>
<item><guid>g2</guid><title>Item 2</title><link>https://example.com/2</link><description>body 2</description></item>
</channel></rss>`)
	}))
	defer server.Close()

	feed, err := st.CreateFeed(1, "Fast RSS", server.URL, "", "")
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	enabled := true
	if err := st.UpdateFeed(feed.ID, store.UpdateFeedParams{AIEnabled: &enabled}); err != nil {
		t.Fatalf("enable ai: %v", err)
	}

	p := New(st, &config.Config{
		PullInterval:      1800,
		PullTimeout:       5,
		PullConcurrency:   1,
		PullMaxBackoff:    604800,
		AllowPrivateFeeds: true,
	})
	p.timeout = 50 * time.Millisecond

	var evaluated atomic.Int32
	p.SetEvaluator(rssEvalFunc(func(ctx context.Context, in triage.Input) (triage.Result, error) {
		if err := ctx.Err(); err != nil {
			return triage.Result{}, err
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < time.Second {
			t.Errorf("evaluator received unexpected short deadline: ok=%v remaining=%v", ok, time.Until(deadline))
		}
		select {
		case <-time.After(75 * time.Millisecond):
		case <-ctx.Done():
			return triage.Result{}, ctx.Err()
		}
		evaluated.Add(1)
		score := 85
		return triage.Result{Domain: "ai", Priority: "high", TriageStatus: "kept", AIScore: &score}, nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.RefreshFeed(ctx, feed.ID); err != nil {
		t.Fatalf("refresh feed: %v", err)
	}

	if got := evaluated.Load(); got != 2 {
		t.Fatalf("evaluated = %d, want 2", got)
	}

	saved, err := st.ListItems(store.ListItemsParams{FeedID: &feed.ID})
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(saved) != 2 {
		t.Fatalf("saved items = %d, want 2", len(saved))
	}
	for _, item := range saved {
		if item.TriageStatus != "kept" || item.AIScore == nil || *item.AIScore != 85 {
			t.Fatalf("item %s triage = (%q, %v), want (kept, 85)", item.GUID, item.TriageStatus, item.AIScore)
		}
	}

	updated, err := st.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if updated.FetchState.LastSuccessAt == 0 || updated.FetchState.ConsecutiveFailures != 0 {
		t.Fatalf("unexpected fetch state: %+v", updated.FetchState)
	}
}

func TestRefreshFeedFetchTimeoutStillCancelsSlowHTTP(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	unblock := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-unblock:
		case <-time.After(2 * time.Second):
		}
	}))
	defer func() {
		close(unblock)
		server.Close()
	}()

	feed, err := st.CreateFeed(1, "Slow RSS", server.URL, "", "")
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	enabled := true
	if err := st.UpdateFeed(feed.ID, store.UpdateFeedParams{AIEnabled: &enabled}); err != nil {
		t.Fatalf("enable ai: %v", err)
	}

	p := New(st, &config.Config{
		PullInterval:      1800,
		PullTimeout:       5,
		PullConcurrency:   1,
		PullMaxBackoff:    604800,
		AllowPrivateFeeds: true,
	})
	p.timeout = 50 * time.Millisecond

	var evalCalls atomic.Int32
	p.SetEvaluator(rssEvalFunc(func(ctx context.Context, in triage.Input) (triage.Result, error) {
		evalCalls.Add(1)
		return triage.Result{Domain: "other", Priority: "normal", TriageStatus: "kept"}, nil
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if err := p.RefreshFeed(ctx, feed.ID); err != nil {
		t.Fatalf("refresh feed: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("slow fetch took %v, expected prompt timeout near 50ms", elapsed)
	}

	if got := evalCalls.Load(); got != 0 {
		t.Fatalf("evaluator called %d times on failed fetch, want 0", got)
	}

	updated, err := st.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if updated.FetchState.ConsecutiveFailures != 1 || updated.FetchState.LastSuccessAt != 0 || updated.FetchState.LastError == "" {
		t.Fatalf("expected recorded fetch timeout failure, got %+v", updated.FetchState)
	}
}

func TestBackfillFeedTriageAndProgressSnapshot(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	feed, err := st.CreateFeed(1, "RSS", "https://example.test/rss.xml", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.BatchCreateItemsIgnore(feed.ID, []store.BatchCreateItemInput{
		{GUID: "g1", Title: "Old", Link: "https://example.test/1", Content: "body", PubDate: 100},
		{GUID: "g2", Title: "Mid", Link: "https://example.test/2", Content: "body", PubDate: 200},
		{GUID: "g3", Title: "New", Link: "https://example.test/3", Content: "body", PubDate: 300},
	}); err != nil {
		t.Fatal(err)
	}
	enabled := true
	if err := st.UpdateFeed(feed.ID, store.UpdateFeedParams{AIEnabled: &enabled}); err != nil {
		t.Fatal(err)
	}

	p := New(st, &config.Config{PullInterval: 1800, PullTimeout: 5, PullConcurrency: 1, PullMaxBackoff: 604800, AllowPrivateFeeds: true})
	p.SetRSSLimit(2)
	p.SetInterval(10 * time.Minute)
	if p.Interval() != 10*time.Minute {
		t.Fatalf("expected 10m interval, got %v", p.Interval())
	}

	var observedPhase string
	var observedTotal int
	p.SetEvaluator(rssEvalFunc(func(ctx context.Context, in triage.Input) (triage.Result, error) {
		if snap := p.ProgressSnapshot(); len(snap) > 0 {
			if pr, ok := snap[feed.ID]; ok {
				observedPhase = pr.Phase
				observedTotal = pr.Total
			}
		}
		score := 75
		return triage.Result{Domain: "ai", Priority: "high", TriageStatus: "kept", AIScore: &score}, nil
	}))

	processed, err := p.BackfillFeedTriage(context.Background(), feed.ID)
	if err != nil || processed != 2 {
		t.Fatalf("expected 2 backfilled items, got processed=%d err=%v", processed, err)
	}
	if observedPhase != "triaging" || observedTotal != 2 {
		t.Fatalf("expected active triaging progress (total=2), got phase=%q total=%d", observedPhase, observedTotal)
	}
	if snap := p.ProgressSnapshot(); len(snap) != 0 {
		t.Fatalf("expected cleared progress after completion, got %+v", snap)
	}

	feeds, err := st.ListFeeds()
	if err != nil || len(feeds) != 1 {
		t.Fatalf("list feeds: %v (%d)", err, len(feeds))
	}
	tc := feeds[0].TriageCounts
	if tc.Kept != 2 || tc.Unscored != 1 || tc.Pending != 0 || tc.Failed != 0 {
		t.Fatalf("unexpected triage counts: %+v", tc)
	}
}
