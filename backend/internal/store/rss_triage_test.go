package store

import "testing"

func TestBatchCreateItemsForTriage(t *testing.T) {
	s, _ := setupTestDB(t)
	defer closeStore(t, s)
	group := mustCreateGroup(t, s, "rss")
	feed := mustCreateFeed(t, s, group.ID, "feed", "https://example.test/rss", "", "")
	inputs := []BatchCreateItemInput{{GUID: "one", Title: "first"}, {GUID: "one", Title: "duplicate"}, {GUID: "two", Title: "second"}}
	rows, err := s.BatchCreateItemsForTriage(feed.ID, inputs)
	if err != nil || len(rows) != 2 {
		t.Fatalf("created=%d, err=%v", len(rows), err)
	}
	for i, r := range rows {
		stored, err := s.GetItem(r.ID)
		if err != nil || r.BatchCreateItemInput != inputs[i*2] || stored.GUID != r.GUID || stored.TriageStatus != "pending" {
			t.Fatalf("wrong row %+v (%v)", r, err)
		}
	}
	rows, err = s.BatchCreateItemsForTriage(feed.ID, inputs)
	if err != nil || len(rows) != 0 {
		t.Fatal("duplicate GUID requeued")
	}
}
