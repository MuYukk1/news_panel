package model

import (
	"encoding/json"
	"news_panel/internal/triage"
)

// Group represents a feed group.
type Group struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// Feed represents an RSS/Atom feed.
type Feed struct {
	ID        int64  `json:"id"`
	GroupID   int64  `json:"group_id"`
	Name      string `json:"name"`
	Link      string `json:"link"`
	SiteURL   string `json:"site_url,omitempty"`
	Suspended bool   `json:"suspended"`
	AIEnabled bool   `json:"ai_enabled"`
	Proxy     string `json:"proxy,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`

	FetchState FeedFetchState `json:"fetch_state"`

	UnreadCount  int64            `json:"unread_count"`
	ItemCount    int64            `json:"item_count"`
	TriageCounts FeedTriageCounts `json:"triage_counts"`
	Progress     *FeedProgress    `json:"progress,omitempty"`
}

// FeedTriageCounts summarizes persisted AI triage buckets for a feed.
type FeedTriageCounts struct {
	Kept     int64 `json:"kept"`
	Filtered int64 `json:"filtered"`
	Unscored int64 `json:"unscored"`
	Pending  int64 `json:"pending"`
	Failed   int64 `json:"failed"`
}

// FeedProgress reports active in-flight pull or AI triage progress for a feed.
type FeedProgress struct {
	Phase      string `json:"phase"` // "fetching" | "triaging"
	Total      int    `json:"total"`
	Processed  int    `json:"processed"`
	FetchedNew int    `json:"fetched_new"`
	StartedAt  int64  `json:"started_at,omitempty"`
	UpdatedAt  int64  `json:"updated_at,omitempty"`
}

// FeedFetchState stores runtime pull metadata for a feed.
// Time fields are Unix seconds; 0 means unknown/unset.
type FeedFetchState struct {
	// ETag is used for If-None-Match conditional requests.
	ETag string `json:"etag,omitempty"`
	// LastModified stores raw HTTP Last-Modified for If-Modified-Since.
	LastModified string `json:"last_modified,omitempty"`
	// CacheControl stores raw HTTP Cache-Control value.
	CacheControl string `json:"cache_control,omitempty"`
	// ExpiresAt is parsed from the HTTP Expires header.
	ExpiresAt int64 `json:"expires_at"`
	// LastCheckedAt is the last fetch attempt time (success or failure).
	LastCheckedAt int64 `json:"last_checked_at"`
	// NextCheckAt is the earliest next fetch time decided by pull policy.
	NextCheckAt int64 `json:"next_check_at"`
	// LastHTTPStatus is the last HTTP status code observed during fetch.
	LastHTTPStatus int `json:"last_http_status"`
	// RetryAfterUntil is derived from Retry-After and blocks fetch before this time.
	RetryAfterUntil int64 `json:"retry_after_until"`
	// LastSuccessAt is the last successful check time (includes 200 and 304).
	LastSuccessAt int64 `json:"last_success_at"`
	// LastErrorAt is the most recent fetch failure time.
	LastErrorAt int64 `json:"last_error_at"`
	// LastError keeps the latest fetch error message.
	LastError string `json:"last_error,omitempty"`
	// ConsecutiveFailures is reset on success and incremented on each failure.
	ConsecutiveFailures int64 `json:"consecutive_failures"`
}

// Item represents a feed item.
type Item struct {
	triage.Result
	SourceID    string          `json:"source_id"`
	ExternalID  string          `json:"external_id"`
	SourceType  string          `json:"source_type"`
	PublishedAt string          `json:"published_at"`
	Author      string          `json:"author"`
	RawMeta     json.RawMessage `json:"raw_meta"`
	ID          int64           `json:"id"`
	FeedID      int64           `json:"feed_id"`
	GUID        string          `json:"guid"`
	Title       string          `json:"title"`
	Link        string          `json:"link"`
	Content     string          `json:"content"`
	PubDate     int64           `json:"pub_date"`
	Unread      bool            `json:"unread"`
	CreatedAt   int64           `json:"created_at"`
}

// Settings holds the user's synced preferences. A nil field means the user
// has never explicitly chosen a value (clients fall back to their own default).
type Settings struct {
	Locale          *string `json:"locale"`
	ArticlePageSize *int64  `json:"article_page_size"`
	Theme           *string `json:"theme"`
	AutoMarkRead    *string `json:"auto_mark_read"`
	UpdatedAt       int64   `json:"updated_at"`
}

// Bookmark represents a saved item snapshot.
type Bookmark struct {
	Triage   *triage.Result `json:"triage"`
	SourceID string         `json:"source_id"`
	ID       int64          `json:"id"`
	ItemID   *int64         `json:"item_id"` // nullable
	Link     string         `json:"link"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	PubDate  int64          `json:"pub_date"`
	FeedName string         `json:"feed_name"`
	// FeedID is a soft association to the source feed used for filtering. It is
	// nil for orphaned bookmarks whose feed/item no longer exists; the bookmark
	// itself always survives such deletions (it is a content snapshot).
	FeedID *int64 `json:"feed_id"`
	// Unread mirrors the linked item's unread state (false for orphans).
	Unread    bool  `json:"unread"`
	CreatedAt int64 `json:"created_at"`
}

// AISettings contains only explicit database overrides. Nil inherits the startup configuration.
type AISettings struct {
	DecisionEnabled        *bool   `json:"decision_enabled,omitempty"`
	DecisionProvider       *string `json:"decision_provider,omitempty"`
	DecisionBaseURL        *string `json:"decision_base_url,omitempty"`
	DecisionAPIKey         *string `json:"decision_api_key,omitempty"`
	DecisionAccountID      *string `json:"decision_account_id,omitempty"`
	DecisionAuthHeader     *string `json:"decision_auth_header,omitempty"`
	DecisionModel          *string `json:"decision_model,omitempty"`
	DecisionJSONFormat     *bool   `json:"decision_json_format,omitempty"`
	DecisionTimeoutSeconds *int    `json:"decision_timeout_seconds,omitempty"`
	DecisionL3Priorities   *string `json:"decision_l3_priorities,omitempty"`
	DecisionLowAction      *string `json:"decision_low_action,omitempty"`
	DecisionFallbackMode   *string `json:"decision_fallback_mode,omitempty"`
	Topics                 *string `json:"topics,omitempty"`
	RSSMaxItemsPerPull     *int    `json:"rss_max_items_per_pull,omitempty"`
	RSSPullIntervalSeconds *int    `json:"rss_pull_interval_seconds,omitempty"`
}

// Apply merges a partial update. An empty key preserves the saved credential.
func (s AISettings) Apply(p AISettings, clearKey bool) AISettings {
	if p.DecisionEnabled != nil {
		s.DecisionEnabled = p.DecisionEnabled
	}
	if p.DecisionProvider != nil {
		s.DecisionProvider = p.DecisionProvider
	}
	if p.DecisionBaseURL != nil {
		s.DecisionBaseURL = p.DecisionBaseURL
	}
	if p.DecisionAPIKey != nil && *p.DecisionAPIKey != "" {
		s.DecisionAPIKey = p.DecisionAPIKey
	}
	if p.DecisionAccountID != nil {
		s.DecisionAccountID = p.DecisionAccountID
	}
	if p.DecisionAuthHeader != nil {
		s.DecisionAuthHeader = p.DecisionAuthHeader
	}
	if p.DecisionModel != nil {
		s.DecisionModel = p.DecisionModel
	}
	if p.DecisionJSONFormat != nil {
		s.DecisionJSONFormat = p.DecisionJSONFormat
	}
	if p.DecisionTimeoutSeconds != nil {
		s.DecisionTimeoutSeconds = p.DecisionTimeoutSeconds
	}
	if p.DecisionL3Priorities != nil {
		s.DecisionL3Priorities = p.DecisionL3Priorities
	}
	if p.DecisionLowAction != nil {
		s.DecisionLowAction = p.DecisionLowAction
	}
	if p.DecisionFallbackMode != nil {
		s.DecisionFallbackMode = p.DecisionFallbackMode
	}
	if p.Topics != nil {
		s.Topics = p.Topics
	}
	if p.RSSMaxItemsPerPull != nil {
		s.RSSMaxItemsPerPull = p.RSSMaxItemsPerPull
	}
	if p.RSSPullIntervalSeconds != nil {
		s.RSSPullIntervalSeconds = p.RSSPullIntervalSeconds
	}
	if clearKey {
		s.DecisionAPIKey = nil
	}
	return s
}
