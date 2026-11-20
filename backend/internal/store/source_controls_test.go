package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"news_panel/internal/triage"
)

func TestUpgradeV6ToV7SourceControlsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v6.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	st := &Store{db: db}
	if err := st.createMigrationsTable(); err != nil {
		t.Fatal(err)
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		version, err := extractVersion(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if version < 7 {
			if err := st.applyMigration(version, entry.Name()); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err = db.Exec(`
		INSERT INTO feeds (name, link) VALUES ('Legacy RSS', 'https://example.org/rss.xml');
		INSERT INTO plugin_heartbeats (source_id, plugin_version, status, fetched_count, timestamp, updated_at)
		VALUES ('x-legacy', '0.1.0', 'ok', 3, '2026-11-20T00:00:00Z', '2026-11-20T00:00:01Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, upgraded)

	feeds, err := upgraded.ListFeeds()
	if err != nil || len(feeds) != 1 {
		t.Fatalf("list feeds after v7 upgrade: %+v %v", feeds, err)
	}
	if feeds[0].AIEnabled {
		t.Fatalf("expected existing RSS feed to default ai_enabled=false, got true")
	}

	aiOn := true
	if err := upgraded.UpdateFeed(feeds[0].ID, UpdateFeedParams{AIEnabled: &aiOn}); err != nil {
		t.Fatalf("update feed ai_enabled: %v", err)
	}
	refetched, err := upgraded.GetFeed(feeds[0].ID)
	if err != nil || !refetched.AIEnabled {
		t.Fatalf("expected ai_enabled=true after update, got %+v %v", refetched, err)
	}

	heartbeats, err := upgraded.ListHeartbeats(context.Background())
	if err != nil || len(heartbeats) != 1 {
		t.Fatalf("list heartbeats after v7 upgrade: %+v %v", heartbeats, err)
	}
	if !heartbeats[0].Enabled {
		t.Fatalf("expected existing plugin heartbeat to default enabled=true, got false")
	}
}

func TestScoredAndUnscoredFiltersAndPluginEnablePersistence(t *testing.T) {
	st, _ := setupTestDB(t)
	defer closeStore(t, st)
	ctx := context.Background()

	rssFeed := mustCreateFeed(t, st, 1, "Native RSS", "https://example.org/feed.xml", "", "")
	rssItem, err := st.CreateItem(rssFeed.ID, "rss-1", "Unscored RSS", "https://example.org/1", "body", 100)
	if err != nil {
		t.Fatal(err)
	}

	pendingID, _, err := st.IngestItem(ctx, normalizedItem("x-main", "pending-1"))
	if err != nil {
		t.Fatal(err)
	}

	scoredID, _, err := st.IngestItem(ctx, normalizedItem("x-main", "scored-1"))
	if err != nil {
		t.Fatal(err)
	}
	score := 85
	if err := st.UpdateTriage(ctx, scoredID, triage.Result{
		Domain:       "ai",
		AIScore:      &score,
		AITags:       []string{"llm"},
		AISummary:    "Scored item",
		Priority:     "high",
		TriageStatus: "kept",
	}); err != nil {
		t.Fatal(err)
	}

	filteredID, _, err := st.IngestItem(ctx, normalizedItem("x-main", "filtered-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateTriage(ctx, filteredID, triage.Result{
		Domain:       "other",
		AIScore:      nil,
		AITags:       []string{},
		AISummary:    "",
		Priority:     "low",
		TriageStatus: "filtered",
	}); err != nil {
		t.Fatal(err)
	}

	failedID, _, err := st.IngestItem(ctx, normalizedItem("x-main", "failed-1"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateTriage(ctx, failedID, triage.Result{
		Domain:       "other",
		AIScore:      nil,
		AITags:       []string{},
		AISummary:    "",
		Priority:     "normal",
		TriageStatus: "failed",
	}); err != nil {
		t.Fatal(err)
	}

	scoredTrue := true
	scoredFalse := false

	// Backward-compatible kept (without scored filter) includes both AI-scored kept and RSS kept.
	keptAll, err := st.ListItems(ListItemsParams{TriageStatus: "kept"})
	if err != nil || len(keptAll) != 2 {
		t.Fatalf("kept without scored filter: got %d (%v)", len(keptAll), err)
	}

	// kept + scored=true returns only the AI-scored kept item.
	keptScored, err := st.ListItems(ListItemsParams{TriageStatus: "kept", Scored: &scoredTrue})
	if err != nil || len(keptScored) != 1 || keptScored[0].ID != scoredID {
		t.Fatalf("kept + scored=true: got %+v %v", keptScored, err)
	}

	// scored=false returns every item with NULL ai_score (rss, pending, filtered, failed).
	unscoredRaw, err := st.ListItems(ListItemsParams{Scored: &scoredFalse})
	if err != nil || len(unscoredRaw) != 4 {
		t.Fatalf("scored=false: got %d (%v)", len(unscoredRaw), err)
	}

	// Virtual triage_status="unscored" returns rss + pending, excluding scored, filtered, and failed.
	unscoredBucket, err := st.ListItems(ListItemsParams{TriageStatus: "unscored"})
	if err != nil || len(unscoredBucket) != 2 {
		t.Fatalf("triage_status=unscored: got %d (%v)", len(unscoredBucket), err)
	}
	gotIDs := map[int64]bool{unscoredBucket[0].ID: true, unscoredBucket[1].ID: true}
	if !gotIDs[rssItem.ID] || !gotIDs[pendingID] {
		t.Fatalf("unexpected unscored IDs: %+v", gotIDs)
	}

	// Bookmarks compatibility with LEFT JOIN items (including orphan snapshot).
	if _, err := st.CreateBookmark(&rssItem.ID, &rssItem.FeedID, rssItem.Link, rssItem.Title, rssItem.Content, rssItem.PubDate, rssFeed.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateBookmark(&scoredID, &rssItem.FeedID, "https://x.com/scored", "Scored", "body", 100, "x-main"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateBookmark(&filteredID, &rssItem.FeedID, "https://x.com/filtered", "Filtered", "body", 100, "x-main"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateBookmark(nil, nil, "https://example.org/orphan", "Orphan", "body", 100, "Deleted"); err != nil {
		t.Fatal(err)
	}
	bmUnscored, err := st.ListBookmarks(ListBookmarksParams{Triage: ListItemsParams{TriageStatus: "unscored"}})
	if err != nil || len(bmUnscored) != 2 {
		t.Fatalf("bookmarks unscored (rss + orphan): got %d (%v)", len(bmUnscored), err)
	}
	bmScoredKept, err := st.ListBookmarks(ListBookmarksParams{Triage: ListItemsParams{TriageStatus: "kept", Scored: &scoredTrue}})
	if err != nil || len(bmScoredKept) != 1 {
		t.Fatalf("bookmarks kept+scored=true: got %d (%v)", len(bmScoredKept), err)
	}

	// Plugin enabled flag survives UpsertHeartbeat conflict updates and syncs shadow feed suspended state.
	hb := PluginHeartbeat{
		SourceID:      "x-main",
		PluginVersion: "0.1.0",
		Status:        "ok",
		FetchedCount:  5,
		Timestamp:     "2026-11-20T10:00:00Z",
	}
	if err := st.UpsertHeartbeat(ctx, hb); err != nil {
		t.Fatal(err)
	}
	updatedHB, err := st.SetPluginEnabled(ctx, "x-main", false)
	if err != nil || updatedHB.Enabled {
		t.Fatalf("disable plugin: %+v %v", updatedHB, err)
	}
	hb.FetchedCount = 9
	if err := st.UpsertHeartbeat(ctx, hb); err != nil {
		t.Fatal(err)
	}
	enabledNow, err := st.IsPluginEnabled(ctx, "x-main")
	if err != nil || enabledNow {
		t.Fatalf("expected UpsertHeartbeat not to overwrite enabled=false, got %v %v", enabledNow, err)
	}
	updatedHB, err = st.SetPluginEnabled(ctx, "x-main", true)
	if err != nil || !updatedHB.Enabled {
		t.Fatalf("re-enable plugin: %+v %v", updatedHB, err)
	}
	feeds, err := st.ListFeeds()
	if err != nil {
		t.Fatal(err)
	}
	var shadowSuspended *bool
	for _, f := range feeds {
		if f.Link == "urn:news_panel:source:x-main" {
			v := f.Suspended
			shadowSuspended = &v
		}
	}
	if shadowSuspended == nil || *shadowSuspended {
		t.Fatalf("expected shadow feed suspended=false when plugin enabled, got %v", shadowSuspended)
	}
}
