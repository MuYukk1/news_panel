// Core data models (matching backend/internal/model/model.go)
export interface Group {
  id: number;
  name: string;
  created_at: number;
  updated_at: number;
}

export interface Feed {
  id: number;
  group_id: number;
  name: string;
  link: string;
  site_url?: string;
  suspended: boolean;
  ai_enabled: boolean;
  proxy?: string;
  created_at: number;
  updated_at: number;
  fetch_state: FeedFetchState;
  unread_count: number;
  item_count: number;
}

export interface FeedFetchState {
  etag?: string;
  last_modified?: string;
  cache_control?: string;
  expires_at: number;
  last_checked_at: number;
  next_check_at: number;
  last_http_status: number;
  retry_after_until: number;
  last_success_at: number;
  last_error_at: number;
  last_error?: string;
  consecutive_failures: number;
}

export interface Item {
  source_id: string;
  external_id: string;
  source_type: string;
  published_at?: string;
  author: string;
  raw_meta: Record<string, unknown>;
  domain: string;
  ai_score: number | null;
  ai_tags: string[];
  ai_summary: string;
  priority: "critical" | "high" | "normal" | "low";
  triage_status: "pending" | "kept" | "filtered" | "failed";
  id: number;
  feed_id: number;
  guid: string;
  title: string;
  link: string;
  content: string;
  pub_date: number;
  unread: boolean;
  created_at: number;
}

export interface Bookmark {
  source_id?: string;
  triage?: Pick<Item, "domain" | "ai_score" | "ai_tags" | "ai_summary" | "priority" | "triage_status"> | null;
  id: number;
  item_id: number | null;
  link: string;
  title: string;
  content: string;
  pub_date: number;
  feed_name: string;
  feed_id: number | null;
  unread: boolean;
  created_at: number;
}

// User preferences synced via the backend. A null field means the user has
// never explicitly chosen a value; clients fall back to their own defaults.
export interface Settings {
  locale: string | null;
  article_page_size: number | null;
  theme: string | null;
  auto_mark_read: string | null;
  updated_at: number;
}

// Only the fields being changed are sent; a missing field leaves the stored
// value unchanged (indistinguishable from an explicit null on the wire).
export interface UpdateSettingsRequest {
  locale?: string;
  article_page_size?: number;
  theme?: string;
  auto_mark_read?: string;
}

// API response wrappers
export interface APIResponse<T> {
  data?: T;
  error?: string;
}

// Group and feed lists: small, fully returned in one response
export interface ListResponse<T> {
  data: T[];
  total: number;
}

// Item and bookmark lists: cursor-paginated (see docs/api-conventions.md)
export interface PaginatedListResponse<T> {
  data: T[];
  total: number;
  next_cursor: string | null;
}

// Request types
export interface LoginRequest {
  password: string;
}

export interface CreateGroupRequest {
  name: string;
}

export interface UpdateGroupRequest {
  name: string;
}

export interface CreateFeedRequest {
  group_id: number;
  name: string;
  link: string;
  site_url?: string;
  proxy?: string;
}

export interface UpdateFeedRequest {
  group_id?: number;
  name?: string;
  link?: string;
  site_url?: string;
  suspended?: boolean;
  ai_enabled?: boolean;
  proxy?: string;
}

export interface ValidateFeedRequest {
  url: string;
  proxy?: string;
}

export interface DiscoveredFeed {
  title: string;
  link: string;
  site_url?: string;
  group_name?: string;
}

export interface ValidateFeedResponse {
  feeds: DiscoveredFeed[];
}

// Two mutually exclusive modes: either reference an item by id (the backend
// snapshots all fields from it and ignores everything else — preferred), or
// provide a full snapshot for bookmarks without a backing item.
export type CreateBookmarkRequest =
  | { item_id: number }
  | {
      link: string;
      title: string;
      content: string;
      feed_name: string;
      pub_date?: number;
    };

export interface MarkItemsReadRequest {
  ids: number[];
}

export type ItemTriageQueryStatus = Item["triage_status"] | "unscored";

export interface ListItemsParams {
  triage_status?: ItemTriageQueryStatus;
  scored?: boolean;
  domain?: string;
  priority?: Item["priority"];
  min_score?: number;
  feed_id?: number;
  group_id?: number;
  unread?: boolean;
  limit?: number;
  before?: string;
  order_by?: "pub_date" | "created_at";
}

export interface ListBookmarksParams {
  triage_status?: ItemTriageQueryStatus;
  scored?: boolean;
  domain?: string;
  priority?: Item["priority"];
  min_score?: number;
  feed_id?: number;
  group_id?: number;
  limit?: number;
  before?: string;
}

export interface BatchCreateFeedsRequest {
  feeds: Array<{
    group_id: number;
    name: string;
    link: string;
    site_url?: string;
  }>;
}

export interface SearchFeed {
  id: number;
  name: string;
  link: string;
  site_url: string;
}

export interface SearchItem {
  id: number;
  feed_id: number;
  title: string;
  pub_date: number;
}

export interface SearchResponse {
  feeds: SearchFeed[];
  items: SearchItem[];
}

export interface BatchCreateFeedsResponse {
  created: number;
  failed: number;
  // Always present on the wire; null when every feed was created.
  errors: string[] | null;
}

export interface PluginHeartbeat {
  source_id: string;
  plugin_version: string;
  status: "ok" | "degraded" | "error";
  fetched_count: number;
  error_message: string | null;
  timestamp: string;
  updated_at: string;
  enabled: boolean;
}

export interface UpdatePluginRequest {
  enabled: boolean;
}

// OIDC
export interface OIDCStatusResponse {
  enabled: boolean;
}

export interface OIDCLoginResponse {
  auth_url: string;
}

export interface AITriageConfig {
  decision_enabled: boolean;
  decision_provider: string;
  decision_base_url: string;
  decision_account_id: string;
  decision_auth_header: string;
  decision_model: string;
  decision_json_format: boolean;
  decision_timeout_seconds: number;
  decision_l3_priorities: string;
  decision_low_action: string;
  decision_fallback_mode: string;
  topics: string;
  rss_max_items_per_pull: number;
}

export interface DecisionPreset {
  id: string;
  label: string;
  default_base_url: string;
  default_model: string;
  requires_api_key: boolean;
  requires_account_id: boolean;
  auth_header_hint: string;
}

export interface AITriageSettings {
  decision_requested: boolean;
  effective: AITriageConfig;
  sources: Record<string, "db" | "env" | "default" | "none">;
  overrides: Record<string, boolean>;
  decision_api_key_configured: boolean;
  decision_api_key_source: "db" | "env" | "none";
  decision_api_key_masked: string;
  l3_enabled: boolean;
  l3_provider: string;
  l3_model: string;
  presets: DecisionPreset[];
}

export type AISettingsPatch = Partial<AITriageConfig> & {
  decision_api_key?: string;
  clear_decision_api_key?: boolean;
  reset_to_env?: boolean;
};

export interface AITriageStats {
  l2_enabled: boolean;
  l2_provider: string;
  l1_filtered: number;
  l2_filtered: number;
  l2_bypassed_l3: number;
  l3_called: number;
}

export interface DecisionTestResult {
  ok: boolean;
  provider: string;
  model: string;
  latency_ms: number;
  outcome: { action: string; priority: string; domain: string; reason: string };
  error: string;
}
