package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"news_panel/internal/triage"
)

func normalizedItem(source, external string) triage.Input {
	return triage.Input{SourceID: source, SourceType: "x", ExternalID: external, URL: "https://x.com/example/status/" + external, Content: "news", PublishedAt: time.Now().UTC()}
}

func TestPluginIdempotencyAndRSSCompatibility(t *testing.T) {
	st, _ := setupTestDB(t)
	defer closeStore(t, st)
	ctx := context.Background()
	id, created, err := st.IngestItem(ctx, normalizedItem("source-a", "same"))
	if err != nil || !created {
		t.Fatalf("insert: %v %v", created, err)
	}
	item, err := st.GetItem(id)
	if err != nil || item.TriageStatus != "pending" || item.Domain != "other" || item.Priority != "normal" || item.AIScore != nil || len(item.AITags) != 0 {
		t.Fatalf("defaults: %+v %v", item, err)
	}
	if _, created, err := st.IngestItem(ctx, normalizedItem("source-a", "same")); err != nil || created {
		t.Fatalf("duplicate: %v %v", created, err)
	}
	if _, created, err := st.IngestItem(ctx, normalizedItem("source-b", "same")); err != nil || !created {
		t.Fatalf("cross-source ID: %v %v", created, err)
	}
	feed := mustCreateFeed(t, st, 1, "RSS", "https://example.org/rss", "", "")
	for _, guid := range []string{"same", "other"} {
		item, err := st.CreateItem(feed.ID, guid, "title", "https://example.org", "content", 1)
		if err != nil || item.SourceID != "" || item.ExternalID != "" || item.TriageStatus != "kept" {
			t.Fatalf("RSS %s: %+v %v", guid, item, err)
		}
	}
	if _, err := st.CreateItem(feed.ID, "same", "duplicate", "", "", 1); err == nil {
		t.Fatal("RSS unique index lost")
	}
	n, err := st.BatchCreateItemsIgnore(feed.ID, []BatchCreateItemInput{{GUID: "same"}, {GUID: "third"}})
	if err != nil || n != 1 {
		t.Fatalf("RSS batch dedup: %d %v", n, err)
	}
	count, err := st.CountItems(ListItemsParams{})
	if err != nil || count != 5 {
		t.Fatalf("count: %d %v", count, err)
	}
}

func TestConcurrentPluginIdempotency(t *testing.T) {
	st, _ := setupTestDB(t)
	defer closeStore(t, st)
	var inserted atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			_, created, err := st.IngestItem(context.Background(), normalizedItem("same-source", "same-id"))
			if err != nil {
				t.Error(err)
				return
			}
			if created {
				inserted.Add(1)
			}
		})
	}
	wg.Wait()
	if inserted.Load() != 1 {
		t.Fatalf("concurrent insert count: %d", inserted.Load())
	}
}

func TestUpgradeV5PreservesRSSAndAddsPluginSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v5.db")
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
		if version < 6 {
			if err := st.applyMigration(version, entry.Name()); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err = db.Exec(`INSERT INTO feeds (name,link) VALUES ('existing','https://example.org/rss');
        INSERT INTO items (feed_id,guid,title,content) VALUES (1,'old-a','Old A','body'),(1,'old-b','Old B','body');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, st)
	items, err := st.ListItems(ListItemsParams{TriageStatus: "kept"})
	if err != nil || len(items) != 2 {
		t.Fatalf("upgraded RSS: %d %v", len(items), err)
	}
	for _, item := range items {
		if item.SourceID != "" || item.AIScore != nil || item.AITags == nil || item.Domain != "other" {
			t.Fatalf("bad upgrade: %+v", item)
		}
	}
	if err := st.migrate(); err != nil {
		t.Fatal(err)
	}
	if _, created, err := st.IngestItem(context.Background(), normalizedItem("source", "external")); err != nil || !created {
		t.Fatalf("upgraded ingest: %v %v", created, err)
	}
	if _, err := st.CreateItem(1, "old-a", "", "", "", 0); err == nil {
		t.Fatal("upgraded RSS duplicate accepted")
	}
	var indexSQL string
	if err := st.db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='idx_items_source_external'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	t.Logf("v5 -> v6 preserves two RSS rows and original RSS uniqueness; plugin index: %s", indexSQL)
}

func TestTriageStorageValidation(t *testing.T) {
	st, _ := setupTestDB(t)
	defer closeStore(t, st)
	ctx := context.Background()
	id, _, err := st.IngestItem(ctx, normalizedItem("s", "1"))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := (triage.PassThrough{}).Evaluate(ctx, triage.Input{})
	score := 88
	result.AIScore = &score
	result.AITags = []string{"AI", "release"}
	result.AISummary = "Summary"
	result.Domain = "ai"
	result.Priority = "high"
	if err := st.UpdateTriage(ctx, id, result); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetItem(id)
	if err != nil || got.AIScore == nil || *got.AIScore != 88 || fmt.Sprint(got.AITags) != "[AI release]" || got.AISummary != "Summary" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	score = 101
	if err := st.UpdateTriage(ctx, id, result); err == nil {
		t.Fatal("invalid score persisted")
	}
	score = 88
	result.Priority = "urgent"
	if err := st.UpdateTriage(ctx, id, result); err == nil {
		t.Fatal("invalid priority persisted")
	}
}
