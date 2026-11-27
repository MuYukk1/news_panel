import { api } from "./client";
import type {
  APIResponse, AISettingsPatch, AITriageSettings, AITriageStats, DecisionTestResult,
  ListResponse,
  PaginatedListResponse,
  LoginRequest,
  Group,
  Feed,
  Item,
  Bookmark,
  CreateGroupRequest,
  UpdateGroupRequest,
  CreateFeedRequest,
  UpdateFeedRequest,
  ValidateFeedRequest,
  ValidateFeedResponse,
  CreateBookmarkRequest,
  MarkItemsReadRequest,
  ListItemsParams,
  ListBookmarksParams,
  BatchCreateFeedsRequest,
  BatchCreateFeedsResponse,
  SearchResponse,
  OIDCStatusResponse,
  OIDCLoginResponse,
  Settings,
  UpdateSettingsRequest,
  PluginHeartbeat,
  UpdatePluginRequest,
} from "./types";

// Session APIs
export const sessionAPI = {
  login: (data: LoginRequest) =>
    api.post<APIResponse<{ message: string }>>("/sessions", data),

  status: () => api.get<void>("/sessions"),

  logout: () => api.delete<void>("/sessions"),
};

// OIDC APIs
export const oidcAPI = {
  status: () => api.get<APIResponse<OIDCStatusResponse>>("/oidc/enabled"),

  login: () => api.get<APIResponse<OIDCLoginResponse>>("/oidc/login"),
};

// Group APIs
export const groupAPI = {
  list: () => api.get<ListResponse<Group>>("/groups"),

  get: (id: number) => api.get<APIResponse<Group>>(`/groups/${id}`),

  create: (data: CreateGroupRequest) =>
    api.post<APIResponse<Group>>("/groups", data),

  update: (id: number, data: UpdateGroupRequest) =>
    api.patch<APIResponse<Group>>(`/groups/${id}`, data),

  delete: (id: number) => api.delete<void>(`/groups/${id}`),
};

// Feed APIs
export const feedAPI = {
  list: () => api.get<ListResponse<Feed>>("/feeds"),

  get: (id: number) => api.get<APIResponse<Feed>>(`/feeds/${id}`),

  create: (data: CreateFeedRequest) =>
    api.post<APIResponse<Feed>>("/feeds", data),

  update: (id: number, data: UpdateFeedRequest) =>
    api.patch<APIResponse<Feed>>(`/feeds/${id}`, data),

  delete: (id: number) => api.delete<void>(`/feeds/${id}`),

  validate: (data: ValidateFeedRequest) =>
    api.post<APIResponse<ValidateFeedResponse>>("/feeds/validate", data),

  refresh: () => api.post<void>("/feeds/refresh"),

  batchCreate: (data: BatchCreateFeedsRequest) =>
    api.post<APIResponse<BatchCreateFeedsResponse>>("/feeds/batch", data),
};

// Item APIs
export const itemAPI = {
  list: (params?: ListItemsParams) => {
    const query = new URLSearchParams();
    if (params?.feed_id) query.set("feed_id", params.feed_id.toString());
    if (params?.group_id) query.set("group_id", params.group_id.toString());
    if (params?.unread !== undefined)
      query.set("unread", params.unread.toString());
    if (params?.limit) query.set("limit", params.limit.toString());
    if (params?.before) query.set("before", params.before);
    if (params?.order_by) query.set("order_by", params.order_by);
    if (params?.triage_status) query.set("triage_status", params.triage_status);
    if (params?.scored !== undefined) query.set("scored", String(params.scored));
    if (params?.domain) query.set("domain", params.domain);
    if (params?.priority) query.set("priority", params.priority);
    if (params?.min_score !== undefined) query.set("min_score", String(params.min_score));

    const queryString = query.toString();
    return api.get<PaginatedListResponse<Item>>(
      `/items${queryString ? `?${queryString}` : ""}`,
    );
  },

  get: (id: number) => api.get<APIResponse<Item>>(`/items/${id}`),

  markRead: (data: MarkItemsReadRequest) =>
    api.patch<void>("/items/-/read", data),

  markUnread: (data: MarkItemsReadRequest) =>
    api.patch<void>("/items/-/unread", data),
};

// Bookmark APIs
export const bookmarkAPI = {
  list: (params: ListBookmarksParams = {}) => {
    const query = new URLSearchParams();
    if (params.feed_id) query.set("feed_id", params.feed_id.toString());
    if (params.group_id) query.set("group_id", params.group_id.toString());
    query.set("limit", (params.limit ?? 50).toString());
    if (params.before) query.set("before", params.before);
    if (params.triage_status) query.set("triage_status", params.triage_status);
    if (params.scored !== undefined) query.set("scored", String(params.scored));
    if (params.domain) query.set("domain", params.domain);
    if (params.priority) query.set("priority", params.priority);
    if (params.min_score !== undefined) query.set("min_score", String(params.min_score));
    return api.get<PaginatedListResponse<Bookmark>>(`/bookmarks?${query}`);
  },

  get: (id: number) => api.get<APIResponse<Bookmark>>(`/bookmarks/${id}`),

  create: (data: CreateBookmarkRequest) =>
    api.post<APIResponse<Bookmark>>("/bookmarks", data),

  delete: (id: number) => api.delete<void>(`/bookmarks/${id}`),
};

// Settings APIs
export const aiSettingsAPI = {
  get: () => api.get<APIResponse<AITriageSettings>>("/v1/ai/settings"),
  update: (patch: AISettingsPatch) => api.patch<APIResponse<AITriageSettings>>("/v1/ai/settings", patch),
  test: (patch: AISettingsPatch) => api.post<APIResponse<DecisionTestResult>>("/v1/ai/settings/test", patch),
  stats: () => api.get<APIResponse<AITriageStats>>("/v1/ai/stats"),
};

export const settingsAPI = {
  get: () => api.get<APIResponse<Settings>>("/settings"),

  update: (data: UpdateSettingsRequest) =>
    api.patch<APIResponse<Settings>>("/settings", data),
};

// Plugin APIs
export const pluginAPI = {
  heartbeats: () =>
    api.get<ListResponse<PluginHeartbeat>>("/v1/plugins/heartbeats"),

  update: (sourceId: string, data: UpdatePluginRequest) =>
    api.patch<APIResponse<PluginHeartbeat>>(
      `/v1/plugins/${encodeURIComponent(sourceId)}`,
      data,
    ),
};

// Search APIs
export const searchAPI = {
  search: (q: string, limit = 10) =>
    api.get<APIResponse<SearchResponse>>(
      `/search?q=${encodeURIComponent(q)}&limit=${limit}`,
    ),
};

export * from "./types";
export { APIError, setUnauthorizedCallback } from "./client";
