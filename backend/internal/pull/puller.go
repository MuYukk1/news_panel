package pull

import (
	"context"
	"fmt"
	"log/slog"
	"news_panel/internal/ai"
	"news_panel/internal/triage"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"news_panel/internal/config"
	"news_panel/internal/model"
	"news_panel/internal/pullpolicy"
	"news_panel/internal/store"
)

type Puller struct {
	evaluator   triage.TriageEvaluator
	rssLimit    int
	store       *store.Store
	config      *config.Config
	logger      *slog.Logger
	interval    time.Duration
	timeout     time.Duration
	maxBackoff  time.Duration
	concurrency *semaphore.Weighted
}

func New(st *store.Store, cfg *config.Config) *Puller {
	return &Puller{
		store:       st,
		rssLimit:    ai.RSSMaxItemsPerPull(),
		config:      cfg,
		logger:      slog.Default(),
		interval:    time.Duration(cfg.PullInterval) * time.Second,
		timeout:     time.Duration(cfg.PullTimeout) * time.Second,
		maxBackoff:  time.Duration(cfg.PullMaxBackoff) * time.Second,
		concurrency: semaphore.NewWeighted(int64(cfg.PullConcurrency)),
	}
}

// SetEvaluator installs the shared pipeline before starting the puller.
func (p *Puller) SetEvaluator(eval triage.TriageEvaluator) { p.evaluator = eval }

// Start begins periodic feed pulling. Blocks until context is cancelled.
func (p *Puller) Start(ctx context.Context) error {
	p.logger.Info("pull service started", "interval", p.interval, "timeout", p.timeout, "concurrency", p.config.PullConcurrency)

	// Run immediately on startup
	p.pullAll(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("pull service stopping")
			return ctx.Err()
		case <-ticker.C:
			p.pullAll(ctx)
		}
	}
}

const pluginShadowFeedPrefix = "urn:news_panel:source:"

func isPluginShadowFeed(link string) bool {
	return strings.HasPrefix(strings.TrimSpace(link), pluginShadowFeedPrefix)
}

// pullAll fetches all feeds concurrently with semaphore limiting.
func (p *Puller) pullAll(ctx context.Context) {
	feeds, err := p.store.ListFeeds()
	if err != nil {
		p.logger.Error("failed to list feeds", "error", err)
		return
	}

	now := time.Now().Unix()
	_, _ = p.dispatchFeeds(ctx, feeds, func(feed *model.Feed) bool {
		if isPluginShadowFeed(feed.Link) {
			return false
		}
		state := pullpolicy.FeedRuntimeState{
			Suspended:           feed.Suspended,
			RetryAfterUntil:     feed.FetchState.RetryAfterUntil,
			NextCheckAt:         feed.FetchState.NextCheckAt,
			ConsecutiveFailures: feed.FetchState.ConsecutiveFailures,
			LastErrorAt:         feed.FetchState.LastErrorAt,
			LastCheckedAt:       feed.FetchState.LastCheckedAt,
		}
		return !pullpolicy.ShouldSkip(now, state, p.interval, p.maxBackoff)
	})
}

// pullFeed fetches single feed and saves new items.
func (p *Puller) pullFeed(ctx context.Context, feed *model.Feed) {
	p.logger.Debug("pulling feed", "feed_id", feed.ID, "feed_name", feed.Name)

	fetchCtx := ctx
	var cancelFetch context.CancelFunc
	if p.timeout > 0 {
		fetchCtx, cancelFetch = context.WithTimeout(ctx, p.timeout)
	}
	result, err := FetchAndParse(fetchCtx, feed, p.timeout, p.config.AllowPrivateFeeds)
	if cancelFetch != nil {
		cancelFetch()
	}
	checkedAt := time.Now().Unix()
	if err != nil {
		httpStatus := 0
		retryAfterUntil := int64(0)
		if result != nil {
			httpStatus = result.HTTPStatus
			retryAfterUntil = result.RetryAfterUntil
		}

		if err := p.store.UpdateFeedFetchFailure(feed.ID, store.UpdateFeedFetchFailureParams{
			CheckedAt:       checkedAt,
			HTTPStatus:      httpStatus,
			LastError:       err.Error(),
			RetryAfterUntil: retryAfterUntil,
			IntervalSeconds: int64(p.interval.Seconds()),
			MaxBackoff:      int64(p.maxBackoff.Seconds()),
		}); err != nil {
			p.logger.Error("failed to record failure", "feed_id", feed.ID, "error", err)
		}

		p.logger.Warn("failed to fetch feed", "feed_id", feed.ID, "feed_name", feed.Name, "status", httpStatus, "error", err)
		return
	}

	if result.NotModified {
		etag := result.ETag
		// Some servers reply 304 without echoing validators; keep the previous
		// ones so future conditional requests remain effective.
		if strings.TrimSpace(etag) == "" {
			etag = feed.FetchState.ETag
		}

		lastModified := result.LastModified
		if strings.TrimSpace(lastModified) == "" {
			lastModified = feed.FetchState.LastModified
		}

		cacheControl := result.CacheControl
		if strings.TrimSpace(cacheControl) == "" {
			cacheControl = feed.FetchState.CacheControl
		}

		expiresAt := result.ExpiresAt
		if expiresAt == 0 {
			expiresAt = feed.FetchState.ExpiresAt
		}

		nextCheckAt := pullpolicy.ComputeNextCheckAt(
			checkedAt,
			p.interval,
			p.maxBackoff,
			0,
			result.RetryAfterUntil,
			cacheControl,
			expiresAt,
		)

		if err := p.store.UpdateFeedFetchSuccess(feed.ID, store.UpdateFeedFetchSuccessParams{
			CheckedAt:       checkedAt,
			HTTPStatus:      result.HTTPStatus,
			ETag:            etag,
			LastModified:    lastModified,
			CacheControl:    cacheControl,
			ExpiresAt:       expiresAt,
			RetryAfterUntil: result.RetryAfterUntil,
			NextCheckAt:     nextCheckAt,
		}); err != nil {
			p.logger.Error("failed to persist not-modified state", "feed_id", feed.ID, "error", err)
			return
		}

		p.logger.Debug("feed not modified", "feed_id", feed.ID, "feed_name", feed.Name)
		return
	}

	nextCheckAt := pullpolicy.ComputeNextCheckAt(
		checkedAt,
		p.interval,
		p.maxBackoff,
		0,
		result.RetryAfterUntil,
		result.CacheControl,
		result.ExpiresAt,
	)

	inputs := make([]store.BatchCreateItemInput, 0, len(result.Items))
	for _, item := range result.Items {
		inputs = append(inputs, store.BatchCreateItemInput{
			GUID:    item.GUID,
			Title:   item.Title,
			Link:    item.Link,
			Content: item.Content,
			PubDate: item.PubDate,
		})
	}

	newCount, err := p.saveItems(ctx, feed, inputs)
	if err != nil {
		p.logger.Error("failed to save feed items", "feed_id", feed.ID, "error", err)
		return
	}

	if err := p.store.UpdateFeedFetchSuccess(feed.ID, store.UpdateFeedFetchSuccessParams{
		CheckedAt:       checkedAt,
		HTTPStatus:      result.HTTPStatus,
		ETag:            result.ETag,
		LastModified:    result.LastModified,
		CacheControl:    result.CacheControl,
		ExpiresAt:       result.ExpiresAt,
		RetryAfterUntil: result.RetryAfterUntil,
		NextCheckAt:     nextCheckAt,
	}); err != nil {
		p.logger.Error("failed to update fetch state", "feed_id", feed.ID, "error", err)
		return
	}

	if strings.TrimSpace(feed.SiteURL) == "" && result.SiteURL != "" {
		if err := p.store.UpdateFeedSiteURLIfEmpty(feed.ID, result.SiteURL); err != nil {
			p.logger.Warn("failed to auto-fill site_url", "feed_id", feed.ID, "site_url", result.SiteURL, "error", err)
		}
	}

	if strings.TrimSpace(feed.Name) == "" || strings.TrimSpace(feed.Name) == strings.TrimSpace(feed.Link) {
		if err := p.store.UpdateFeedNameIfDefault(feed.ID, result.FeedTitle, feed.Link); err != nil {
			p.logger.Warn("failed to auto-fill feed name", "feed_id", feed.ID, "title", result.FeedTitle, "error", err)
		}
	}

	p.logger.Info("feed pulled successfully", "feed_id", feed.ID, "feed_name", feed.Name, "new_items", newCount)
}

func (p *Puller) saveItems(ctx context.Context, feed *model.Feed, inputs []store.BatchCreateItemInput) (int, error) {
	if !feed.AIEnabled || p.evaluator == nil {
		return p.store.BatchCreateItemsIgnore(feed.ID, inputs)
	}
	sort.SliceStable(inputs, func(i, j int) bool { return inputs[i].PubDate > inputs[j].PubDate })
	count, pending, err := p.store.BatchCreateItemsWithTriageLimit(feed.ID, inputs, p.rssLimit)
	if err != nil {
		return 0, err
	}
	var firstSaveErr error
	for _, item := range pending {
		input := triage.Input{SourceID: strconv.FormatInt(feed.ID, 10), SourceType: "rss", ExternalID: item.GUID,
			URL: item.Link, Title: item.Title, Content: item.Content, Author: feed.Name, PublishedAt: time.Unix(item.PubDate, 0).UTC()}
		var result triage.Result
		err := ctx.Err()
		if err == nil {
			result, err = p.evaluator.Evaluate(ctx, input)
		}
		if err == nil {
			err = result.Validate()
		}
		if err != nil {
			p.logger.Warn("RSS triage failed", "feed_id", feed.ID, "item_id", item.ID)
			result = triage.Result{Domain: "other", Priority: "normal", TriageStatus: "failed"}
		}
		// A cancelled pull must still finalize the pending rows it just committed.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err = p.store.UpdateTriage(saveCtx, item.ID, result)
		cancel()
		if err != nil {
			if firstSaveErr == nil {
				firstSaveErr = err
			}
			p.logger.Error("failed to save RSS triage", "feed_id", feed.ID, "item_id", item.ID, "error", err)
		}
	}
	return count, firstSaveErr
}

// RefreshAll triggers refresh for all non-suspended feeds and waits until all
// started refresh jobs have completed. It bypasses backoff/interval skip logic.
// Concurrency is controlled by the same semaphore as periodic pulls.
func (p *Puller) RefreshAll(ctx context.Context) (int, error) {
	feeds, err := p.store.ListFeeds()
	if err != nil {
		return 0, fmt.Errorf("list feeds: %w", err)
	}

	count, err := p.dispatchFeeds(ctx, feeds, func(feed *model.Feed) bool {
		return !feed.Suspended && !isPluginShadowFeed(feed.Link)
	})
	if err != nil {
		return count, err
	}

	return count, nil
}

func (p *Puller) dispatchFeeds(ctx context.Context, feeds []*model.Feed, shouldPull func(*model.Feed) bool) (int, error) {
	count := 0
	var wg sync.WaitGroup
	var acquireErr error

	for _, feed := range feeds {
		if !shouldPull(feed) {
			continue
		}

		if err := p.concurrency.Acquire(ctx, 1); err != nil {
			acquireErr = err
			break
		}

		count++
		wg.Add(1)
		go func(f *model.Feed) {
			defer wg.Done()
			defer p.concurrency.Release(1)
			p.pullFeed(ctx, f)
		}(feed)
	}

	wg.Wait()
	return count, acquireErr
}

// RefreshFeed manually triggers refresh for specific feed (bypasses skip logic).
// Used by HTTP handler for manual refresh requests.
func (p *Puller) RefreshFeed(ctx context.Context, feedID int64) error {
	feed, err := p.store.GetFeed(feedID)
	if err != nil {
		return fmt.Errorf("get feed: %w", err)
	}
	if isPluginShadowFeed(feed.Link) {
		return nil
	}

	if err := p.concurrency.Acquire(ctx, 1); err != nil {
		return err
	}
	defer p.concurrency.Release(1)

	p.pullFeed(ctx, feed)
	return nil
}
