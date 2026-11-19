import type { Item, PluginHeartbeat } from "@/lib/api";
import type { TriageSearch } from "@/lib/triage-search";

export const PANEL_DOMAINS = ["ai", "investing", "deals", "other"] as const;
export type PanelDomain = (typeof PANEL_DOMAINS)[number];

export const PANEL_PRIORITIES: readonly Item["priority"][] = [
  "critical",
  "high",
  "normal",
  "low",
];

export const PANEL_TRIAGE_TABS = [
  "kept",
  "filtered",
  "pending",
  "all",
] as const;
export type PanelTriageTab = (typeof PANEL_TRIAGE_TABS)[number];

export interface DomainVisual {
  key: PanelDomain;
  gradient: string;
  tone: string;
  badgeBg: string;
}

export const DOMAIN_VISUALS: Record<PanelDomain, DomainVisual> = {
  ai: {
    key: "ai",
    gradient: "linear-gradient(135deg, #5B5BEA 0%, #9A7BFF 100%)",
    tone: "#6366F1",
    badgeBg: "rgba(99, 102, 241, 0.14)",
  },
  investing: {
    key: "investing",
    gradient: "linear-gradient(135deg, #17B38A 0%, #4CD4A9 100%)",
    tone: "#149E79",
    badgeBg: "rgba(20, 158, 121, 0.14)",
  },
  deals: {
    key: "deals",
    gradient: "linear-gradient(135deg, #F08228 0%, #FFBE55 100%)",
    tone: "#D97706",
    badgeBg: "rgba(217, 119, 6, 0.14)",
  },
  other: {
    key: "other",
    gradient: "linear-gradient(135deg, #3B82F6 0%, #6CC2F8 100%)",
    tone: "#2F7FE8",
    badgeBg: "rgba(47, 127, 232, 0.14)",
  },
};

export function normalizePanelDomain(domain?: string | null): PanelDomain {
  const normalized = (domain ?? "").trim().toLowerCase();
  if (
    normalized === "ai" ||
    normalized === "investing" ||
    normalized === "deals" ||
    normalized === "other"
  ) {
    return normalized;
  }
  return "other";
}

export function getDomainVisual(domain?: string | null): DomainVisual {
  return DOMAIN_VISUALS[normalizePanelDomain(domain)];
}

export const PRIORITY_COLORS: Record<Item["priority"], string> = {
  critical: "var(--prio-critical)",
  high: "var(--prio-high)",
  normal: "var(--prio-normal)",
  low: "var(--prio-low)",
};

export const HEARTBEAT_COLORS: Record<PluginHeartbeat["status"], string> = {
  ok: "var(--status-ok)",
  degraded: "var(--prio-high)",
  error: "var(--prio-critical)",
};

export interface DomainPulseRow {
  domain: PanelDomain;
  count: number;
  unread: number;
  avgScore: number | null;
  scoredCount: number;
}

export interface RawMetaMetrics {
  likes?: number;
  retweets?: number;
  replies?: number;
  views?: number;
}

export interface ExtractedRawMeta {
  metrics: RawMetaMetrics;
  hasMetrics: boolean;
  originLabel: string | null;
}

export interface HeartbeatSummary {
  total: number;
  ok: number;
  degraded: number;
  error: number;
}

/**
 * Return a sorted copy of items ordered by descending ai_score (null last),
 * breaking ties by pub_date descending, limited to `limit` entries.
 */
export function selectTopPicks(items: readonly Item[], limit = 5): Item[] {
  return [...items]
    .sort((a, b) => {
      const scoreDiff = (b.ai_score ?? -1) - (a.ai_score ?? -1);
      if (scoreDiff !== 0) return scoreDiff;
      return b.pub_date - a.pub_date;
    })
    .slice(0, Math.max(0, limit));
}

/**
 * Aggregate kept items into per-domain count, unread count, and average score.
 */
export function computeDomainPulse(items: readonly Item[]): DomainPulseRow[] {
  const buckets = new Map<
    PanelDomain,
    { count: number; unread: number; scoreSum: number; scoredCount: number }
  >();

  for (const domain of PANEL_DOMAINS) {
    buckets.set(domain, { count: 0, unread: 0, scoreSum: 0, scoredCount: 0 });
  }

  for (const item of items) {
    const key = normalizePanelDomain(item.domain);
    const bucket = buckets.get(key)!;
    bucket.count += 1;
    if (item.unread) bucket.unread += 1;
    if (typeof item.ai_score === "number" && Number.isFinite(item.ai_score)) {
      bucket.scoreSum += item.ai_score;
      bucket.scoredCount += 1;
    }
  }

  return PANEL_DOMAINS.map((domain) => {
    const b = buckets.get(domain)!;
    return {
      domain,
      count: b.count,
      unread: b.unread,
      scoredCount: b.scoredCount,
      avgScore: b.scoredCount > 0 ? Math.round(b.scoreSum / b.scoredCount) : null,
    };
  }).sort((a, b) => {
    if (b.count !== a.count) return b.count - a.count;
    return (b.avgScore ?? -1) - (a.avgScore ?? -1);
  });
}

function parseFiniteMetric(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value) && value >= 0) {
    return value;
  }
  return undefined;
}

/**
 * Safely extract public engagement numbers and origin_label from item.raw_meta.
 * Never exposes arbitrary keys or sensitive credentials.
 */
export function extractRawMeta(
  rawMeta: Record<string, unknown> | null | undefined,
): ExtractedRawMeta {
  if (!rawMeta || typeof rawMeta !== "object") {
    return { metrics: {}, hasMetrics: false, originLabel: null };
  }

  const metricsObj =
    rawMeta.metrics &&
    typeof rawMeta.metrics === "object" &&
    !Array.isArray(rawMeta.metrics)
      ? (rawMeta.metrics as Record<string, unknown>)
      : null;

  const metrics: RawMetaMetrics = {};
  if (metricsObj) {
    const likes = parseFiniteMetric(metricsObj.likes);
    const retweets = parseFiniteMetric(metricsObj.retweets);
    const replies = parseFiniteMetric(metricsObj.replies);
    const views = parseFiniteMetric(metricsObj.views);
    if (likes !== undefined) metrics.likes = likes;
    if (retweets !== undefined) metrics.retweets = retweets;
    if (replies !== undefined) metrics.replies = replies;
    if (views !== undefined) metrics.views = views;
  }

  const originLabel =
    typeof rawMeta.origin_label === "string" && rawMeta.origin_label.trim()
      ? rawMeta.origin_label.trim()
      : null;

  return {
    metrics,
    hasMetrics: Object.keys(metrics).length > 0,
    originLabel,
  };
}

export function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return "0";
  if (value >= 1_000_000) {
    const m = value / 1_000_000;
    return `${m >= 10 ? Math.round(m) : Number(m.toFixed(1))}M`;
  }
  if (value >= 1_000) {
    const k = value / 1_000;
    return `${k >= 100 ? Math.round(k) : Number(k.toFixed(1))}K`;
  }
  return String(Math.round(value));
}

export function summarizeHeartbeats(
  heartbeats: readonly PluginHeartbeat[],
): HeartbeatSummary {
  let ok = 0;
  let degraded = 0;
  let error = 0;
  for (const hb of heartbeats) {
    if (hb.status === "ok") ok += 1;
    else if (hb.status === "degraded") degraded += 1;
    else error += 1;
  }
  return { total: heartbeats.length, ok, degraded, error };
}

export function sortLoadedArticles(
  items: readonly Item[],
  sortBy: "time" | "score",
): Item[] {
  if (sortBy === "time") {
    return [...items];
  }
  return [...items].sort((a, b) => {
    const scoreDiff = (b.ai_score ?? -1) - (a.ai_score ?? -1);
    if (scoreDiff !== 0) return scoreDiff;
    return b.pub_date - a.pub_date;
  });
}

export function countActiveAdvancedFilters(search: TriageSearch): number {
  let count = 0;
  if (search.priority) count += 1;
  if (search.min_score !== undefined && search.min_score > 0) count += 1;
  return count;
}

export function formatRelativeIsoTime(
  iso: string | null | undefined,
  locale: string,
  nowMs = Date.now(),
): string {
  if (!iso) return "—";
  const parsed = Date.parse(iso);
  if (Number.isNaN(parsed)) return "—";

  const diffMs = nowMs - parsed;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const absSeconds = Math.floor(Math.abs(diffMs) / 1000);
  const sign = diffMs >= 0 ? -1 : 1;

  if (absSeconds < 60) return rtf.format(0, "second");
  const minutes = Math.floor(absSeconds / 60);
  if (minutes < 60) return rtf.format(sign * minutes, "minute");
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return rtf.format(sign * hours, "hour");
  const days = Math.floor(hours / 24);
  if (days < 30) return rtf.format(sign * days, "day");

  return new Date(parsed).toLocaleDateString(locale);
}

export function getGreetingKey(
  hour: number,
): "overview.greeting.morning" | "overview.greeting.afternoon" | "overview.greeting.evening" {
  if (hour >= 5 && hour < 12) return "overview.greeting.morning";
  if (hour >= 12 && hour < 18) return "overview.greeting.afternoon";
  return "overview.greeting.evening";
}

export function isPluginSourceFeed(link: string): boolean {
  return link.startsWith("urn:news_panel:source:");
}

export interface ItemTimestampPair {
  publishedUnix: number | null;
  fetchedUnix: number | null;
}

/**
 * Resolve original publish time (preferring RFC 3339 `published_at`, falling
 * back to Unix seconds `pub_date`) and plugin fetch/ingest time (`created_at`).
 */
export function resolveItemTimestamps(
  item: Pick<Item, "pub_date" | "published_at" | "created_at">,
): ItemTimestampPair {
  let publishedUnix: number | null = null;
  if (item.published_at) {
    const parsedMs = Date.parse(item.published_at);
    if (!Number.isNaN(parsedMs) && parsedMs > 0) {
      publishedUnix = Math.floor(parsedMs / 1000);
    }
  }
  if (
    publishedUnix === null &&
    typeof item.pub_date === "number" &&
    Number.isFinite(item.pub_date) &&
    item.pub_date > 0
  ) {
    publishedUnix = Math.floor(item.pub_date);
  }

  const fetchedUnix =
    typeof item.created_at === "number" &&
    Number.isFinite(item.created_at) &&
    item.created_at > 0
      ? Math.floor(item.created_at)
      : null;

  return { publishedUnix, fetchedUnix };
}

export function formatExactDateTime(
  unixSeconds: number | null | undefined,
  locale: string,
): string {
  if (
    typeof unixSeconds !== "number" ||
    !Number.isFinite(unixSeconds) ||
    unixSeconds <= 0
  ) {
    return "—";
  }
  return new Date(unixSeconds * 1000).toLocaleString(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}
