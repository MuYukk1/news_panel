package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"news_panel/internal/config"
	"news_panel/internal/store"
)

func TestRefreshAllRunGuard(t *testing.T) {
	h := &Handler{}

	if !h.tryStartRefreshAll() {
		t.Fatal("expected first refresh-all start to succeed")
	}
	if h.tryStartRefreshAll() {
		t.Fatal("expected second refresh-all start to be rejected")
	}

	h.finishRefreshAll()

	if !h.tryStartRefreshAll() {
		t.Fatal("expected refresh-all to be allowed after finish")
	}
}

func TestFeedRefreshRunGuard(t *testing.T) {
	h := &Handler{}

	if !h.tryStartFeedRefresh(1) {
		t.Fatal("expected first feed 1 refresh to start")
	}
	if h.tryStartFeedRefresh(1) {
		t.Fatal("expected concurrent feed 1 refresh to be rejected")
	}
	if !h.tryStartFeedRefresh(2) {
		t.Fatal("expected feed 2 refresh to run concurrently with feed 1")
	}

	h.finishFeedRefresh(1)

	if !h.tryStartFeedRefresh(1) {
		t.Fatal("expected feed 1 refresh to be allowed after finish")
	}
	h.finishFeedRefresh(1)
	h.finishFeedRefresh(2)
}

func TestFeedRefreshTimeoutCalculation(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want time.Duration
	}{
		{
			name: "nil config uses default 15m",
			cfg:  nil,
			want: singleFeedRefreshTimeout,
		},
		{
			name: "default 30s pull timeout uses 15m",
			cfg:  &config.Config{PullTimeout: 30},
			want: singleFeedRefreshTimeout,
		},
		{
			name: "larger custom pull timeout is preserved",
			cfg:  &config.Config{PullTimeout: 1200},
			want: 20 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handler{config: tt.cfg}
			if got := h.feedRefreshTimeout(); got != tt.want {
				t.Fatalf("feedRefreshTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

type refreshCall struct {
	feedID      int64
	hasDeadline bool
	remaining   time.Duration
	release     chan error
}

type stubRefreshPuller struct {
	blocking bool
	started  chan refreshCall

	mu    sync.Mutex
	calls map[int64]int
}

func newStubRefreshPuller(blocking bool) *stubRefreshPuller {
	return &stubRefreshPuller{
		blocking: blocking,
		started:  make(chan refreshCall, 16),
		calls:    make(map[int64]int),
	}
}

func (p *stubRefreshPuller) RefreshFeed(ctx context.Context, feedID int64) error {
	deadline, hasDeadline := ctx.Deadline()
	var remaining time.Duration
	if hasDeadline {
		remaining = time.Until(deadline)
	}

	p.mu.Lock()
	p.calls[feedID]++
	p.mu.Unlock()

	call := refreshCall{
		feedID:      feedID,
		hasDeadline: hasDeadline,
		remaining:   remaining,
	}
	if p.blocking {
		call.release = make(chan error, 1)
	}
	p.started <- call

	if p.blocking {
		return <-call.release
	}
	return nil
}

func (p *stubRefreshPuller) RefreshAll(ctx context.Context) (int, error) {
	return 0, nil
}

func (p *stubRefreshPuller) callCount(feedID int64) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[feedID]
}

func waitForFeedRefreshIdle(t *testing.T, h *Handler, feedID int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.feedRefreshMu.Lock()
		inFlight := h.feedRefreshInFlight[feedID]
		h.feedRefreshMu.Unlock()
		if !inFlight {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("feed %d remained marked in-flight after refresh completed", feedID)
}

func waitForRefreshCall(t *testing.T, ch <-chan refreshCall) refreshCall {
	t.Helper()
	select {
	case call := <-ch:
		return call
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RefreshFeed invocation")
		return refreshCall{}
	}
}

func TestRefreshFeedTimeoutAndDeduplication(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "refresh.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	feed1, err := st.CreateFeed(1, "Feed 1", "http://127.0.0.1/feed1.xml", "", "")
	if err != nil {
		t.Fatalf("create feed 1: %v", err)
	}
	feed2, err := st.CreateFeed(1, "Feed 2", "http://127.0.0.1/feed2.xml", "", "")
	if err != nil {
		t.Fatalf("create feed 2: %v", err)
	}

	puller := newStubRefreshPuller(true)
	h, err := New(st, &config.Config{
		PullTimeout:       30,
		AllowPrivateFeeds: true,
	}, puller)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := h.SetupRouter()

	// 1. First refresh starts and receives a deadline >= 15m (well above PullTimeout=30s).
	w := performRequest(router, http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed1.ID), nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("first refresh feed1 status = %d, want 202", w.Code)
	}
	call1 := waitForRefreshCall(t, puller.started)
	if call1.feedID != feed1.ID {
		t.Fatalf("call1 feedID = %d, want %d", call1.feedID, feed1.ID)
	}
	if !call1.hasDeadline || call1.remaining < 14*time.Minute {
		t.Fatalf("expected deadline >= 14m (target 15m), got hasDeadline=%v remaining=%v", call1.hasDeadline, call1.remaining)
	}

	// 2. Concurrent refresh for the same feed returns 202 without invoking RefreshFeed again.
	w = performRequest(router, http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed1.ID), nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("duplicate refresh feed1 status = %d, want 202", w.Code)
	}

	// 3. Different feed refreshes in parallel while feed1 is still in-flight.
	w = performRequest(router, http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed2.ID), nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("parallel refresh feed2 status = %d, want 202", w.Code)
	}
	call2 := waitForRefreshCall(t, puller.started)
	if call2.feedID != feed2.ID {
		t.Fatalf("call2 feedID = %d, want %d", call2.feedID, feed2.ID)
	}
	if got := puller.callCount(feed1.ID); got != 1 {
		t.Fatalf("feed1 RefreshFeed calls while in-flight = %d, want 1", got)
	}

	// 4. Finish feed1 with an error and feed2 normally; guard must release on error too.
	call1.release <- errors.New("simulated refresh error")
	call2.release <- nil
	waitForFeedRefreshIdle(t, h, feed1.ID)
	waitForFeedRefreshIdle(t, h, feed2.ID)

	// 5. After completion, feed1 can be refreshed again.
	w = performRequest(router, http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed1.ID), nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("subsequent refresh feed1 status = %d, want 202", w.Code)
	}
	call3 := waitForRefreshCall(t, puller.started)
	if call3.feedID != feed1.ID {
		t.Fatalf("call3 feedID = %d, want %d", call3.feedID, feed1.ID)
	}
	call3.release <- nil
	waitForFeedRefreshIdle(t, h, feed1.ID)

	if got := puller.callCount(feed1.ID); got != 2 {
		t.Fatalf("final feed1 RefreshFeed calls = %d, want 2", got)
	}
	if got := puller.callCount(feed2.ID); got != 1 {
		t.Fatalf("final feed2 RefreshFeed calls = %d, want 1", got)
	}
}

func TestCreateAndBatchCreateFeedsUseSingleFeedRefreshTimeout(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "create_refresh.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	puller := newStubRefreshPuller(false)
	h, err := New(st, &config.Config{
		PullTimeout:       30,
		AllowPrivateFeeds: true,
	}, puller)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := h.SetupRouter()

	w := performRequest(
		router,
		http.MethodPost,
		"/api/feeds",
		mustJSONBody(t, map[string]any{
			"group_id": 1,
			"name":     "Single Feed",
			"link":     "http://127.0.0.1/single.xml",
		}),
		map[string]string{"Content-Type": "application/json"},
	)
	if w.Code != http.StatusOK {
		t.Fatalf("createFeed status = %d, body = %s", w.Code, w.Body.String())
	}
	call := waitForRefreshCall(t, puller.started)
	if !call.hasDeadline || call.remaining < 14*time.Minute {
		t.Fatalf("createFeed refresh deadline = (%v, %v), want >= 14m", call.hasDeadline, call.remaining)
	}
	waitForFeedRefreshIdle(t, h, call.feedID)

	w = performRequest(
		router,
		http.MethodPost,
		"/api/feeds/batch",
		mustJSONBody(t, map[string]any{
			"feeds": []map[string]any{
				{"group_id": 1, "name": "Batch 1", "link": "http://127.0.0.1/batch1.xml"},
				{"group_id": 1, "name": "Batch 2", "link": "http://127.0.0.1/batch2.xml"},
			},
		}),
		map[string]string{"Content-Type": "application/json"},
	)
	if w.Code != http.StatusOK {
		t.Fatalf("batchCreateFeeds status = %d, body = %s", w.Code, w.Body.String())
	}
	for i := 0; i < 2; i++ {
		batchCall := waitForRefreshCall(t, puller.started)
		if !batchCall.hasDeadline || batchCall.remaining < 14*time.Minute {
			t.Fatalf("batchCreateFeeds refresh %d deadline = (%v, %v), want >= 14m", i, batchCall.hasDeadline, batchCall.remaining)
		}
		waitForFeedRefreshIdle(t, h, batchCall.feedID)
	}
}
