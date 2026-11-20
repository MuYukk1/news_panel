import type { Item } from "@/lib/api";
import type { TriageStatusFilter } from "@/lib/triage-search";

export interface ItemFilters {
  triageStatus?: TriageStatusFilter;
  domain?: string;
  priority?: Item["priority"];
  minScore?: number;
  feedId?: number | null;
  groupId?: number | null;
  unread?: boolean;
}

export interface NormalizedItemFilters {
  triageStatus: TriageStatusFilter;
  domain?: string;
  priority?: Item["priority"];
  minScore?: number;
  feedId: number | null;
  groupId: number | null;
  unread: boolean;
}

export function normalizeItemFilters(
  filters: ItemFilters,
): NormalizedItemFilters {
  return {
    triageStatus: filters.triageStatus ?? "kept",
    domain: filters.domain || undefined,
    priority: filters.priority,
    minScore: filters.minScore,
    feedId: filters.feedId ?? null,
    groupId: filters.groupId ?? null,
    unread: filters.unread ?? false,
  };
}

export interface BookmarkFilters {
  triageStatus?: TriageStatusFilter;
  domain?: string;
  priority?: Item["priority"];
  minScore?: number;
  feedId?: number | null;
  groupId?: number | null;
}

export interface NormalizedBookmarkFilters {
  triageStatus?: TriageStatusFilter;
  domain?: string;
  priority?: Item["priority"];
  minScore?: number;
  feedId: number | null;
  groupId: number | null;
}

export function normalizeBookmarkFilters(
  filters: BookmarkFilters,
): NormalizedBookmarkFilters {
  return {
    triageStatus: filters.triageStatus,
    domain: filters.domain || undefined,
    priority: filters.priority,
    minScore: filters.minScore,
    feedId: filters.feedId ?? null,
    groupId: filters.groupId ?? null,
  };
}

export const queryKeys = {
  groups: {
    all: ["groups"] as const,
    list: () => [...queryKeys.groups.all, "list"] as const,
  },
  feeds: {
    all: ["feeds"] as const,
    list: () => [...queryKeys.feeds.all, "list"] as const,
  },
  items: {
    all: ["items"] as const,
    lists: () => [...queryKeys.items.all, "list"] as const,
    list: (filters: ItemFilters) =>
      [...queryKeys.items.all, "list", normalizeItemFilters(filters)] as const,
    details: () => [...queryKeys.items.all, "detail"] as const,
    detail: (id: number) => [...queryKeys.items.all, "detail", id] as const,
    overview: () => [...queryKeys.items.all, "overview"] as const,
    overviewCount: (key: "unread-kept" | "kept" | "filtered" | "unscored") =>
      [...queryKeys.items.overview(), "count", key] as const,
    overviewTop: (limit = 50) =>
      [...queryKeys.items.overview(), "top", limit] as const,
  },
  bookmarks: {
    all: ["bookmarks"] as const,
    lists: () => [...queryKeys.bookmarks.all, "list"] as const,
  },
  plugins: {
    all: ["plugins"] as const,
    heartbeats: () => [...queryKeys.plugins.all, "heartbeats"] as const,
  },
};
