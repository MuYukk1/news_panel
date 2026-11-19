import { describe, expect, it } from "vitest";
import type { Item, PluginHeartbeat } from "@/lib/api";
import {
  computeDomainPulse,
  countActiveAdvancedFilters,
  extractRawMeta,
  formatCompactNumber,
  formatExactDateTime,
  getGreetingKey,
  isPluginSourceFeed,
  resolveItemTimestamps,
  selectTopPicks,
  sortLoadedArticles,
  summarizeHeartbeats,
} from "./panel-utils";

function makeItem(overrides: Partial<Item> = {}): Item {
  return {
    id: 1,
    feed_id: 10,
    guid: "guid-1",
    title: "Test Item",
    link: "https://x.com/test/status/1",
    content: "Test content",
    pub_date: 1_700_000_000,
    unread: true,
    created_at: 1_700_000_000,
    source_id: "x-twifork-main",
    external_id: "1",
    source_type: "x",
    author: "@tester",
    raw_meta: {},
    domain: "ai",
    ai_score: 80,
    ai_tags: ["LLM"],
    ai_summary: "中文摘要",
    priority: "high",
    triage_status: "kept",
    ...overrides,
  };
}

describe("panel-utils aggregation and sorting", () => {
  it("selectTopPicks sorts by ai_score descending with null last and tie-breaks by pub_date without mutating input", () => {
    const input = [
      makeItem({ id: 1, ai_score: null, pub_date: 900 }),
      makeItem({ id: 2, ai_score: 75, pub_date: 500 }),
      makeItem({ id: 3, ai_score: 92, pub_date: 400 }),
      makeItem({ id: 4, ai_score: 92, pub_date: 800 }),
      makeItem({ id: 5, ai_score: 60, pub_date: 700 }),
      makeItem({ id: 6, ai_score: 88, pub_date: 600 }),
    ];
    const snapshotIds = input.map((i) => i.id);

    const picks = selectTopPicks(input, 4);

    expect(picks.map((i) => i.id)).toEqual([4, 3, 6, 2]);
    expect(input.map((i) => i.id)).toEqual(snapshotIds);
  });

  it("computeDomainPulse aggregates counts, unread counts, and average scores while normalizing unknown domains to other", () => {
    const items = [
      makeItem({ id: 1, domain: "ai", ai_score: 90, unread: true }),
      makeItem({ id: 2, domain: "ai", ai_score: 70, unread: false }),
      makeItem({ id: 3, domain: "investing", ai_score: 85, unread: true }),
      makeItem({ id: 4, domain: "deals", ai_score: null, unread: true }),
      makeItem({ id: 5, domain: "macro-custom", ai_score: 50, unread: false }),
    ];

    const pulse = computeDomainPulse(items);
    const byDomain = new Map(pulse.map((row) => [row.domain, row]));

    expect(byDomain.get("ai")).toEqual({
      domain: "ai",
      count: 2,
      unread: 1,
      scoredCount: 2,
      avgScore: 80,
    });
    expect(byDomain.get("investing")).toEqual({
      domain: "investing",
      count: 1,
      unread: 1,
      scoredCount: 1,
      avgScore: 85,
    });
    expect(byDomain.get("deals")).toEqual({
      domain: "deals",
      count: 1,
      unread: 1,
      scoredCount: 0,
      avgScore: null,
    });
    expect(byDomain.get("other")).toEqual({
      domain: "other",
      count: 1,
      unread: 0,
      scoredCount: 1,
      avgScore: 50,
    });
  });

  it("extractRawMeta safely parses numeric metrics and origin_label while ignoring invalid or sensitive fields", () => {
    const parsed = extractRawMeta({
      origin_label: "  @OpenAI  ",
      cookie: "secret-should-be-ignored",
      metrics: {
        likes: 12800,
        retweets: 3400,
        replies: -5,
        views: "invalid",
      },
    });

    expect(parsed).toEqual({
      metrics: { likes: 12800, retweets: 3400 },
      hasMetrics: true,
      originLabel: "@OpenAI",
    });

    expect(extractRawMeta({})).toEqual({
      metrics: {},
      hasMetrics: false,
      originLabel: null,
    });

    expect(formatCompactNumber(12800)).toBe("12.8K");
    expect(formatCompactNumber(2_100_000)).toBe("2.1M");
    expect(formatCompactNumber(420)).toBe("420");
  });

  it("summarizes plugin heartbeats, sorts loaded items, and counts active advanced filters", () => {
    const beats: PluginHeartbeat[] = [
      {
        source_id: "x-main",
        plugin_version: "0.1.0",
        status: "ok",
        fetched_count: 12,
        error_message: null,
        timestamp: "2026-11-19T00:00:00Z",
        updated_at: "2026-11-19T00:00:01Z",
      },
      {
        source_id: "x-backup",
        plugin_version: "0.1.0",
        status: "degraded",
        fetched_count: 2,
        error_message: "rate limited",
        timestamp: "2026-11-19T00:00:00Z",
        updated_at: "2026-11-19T00:00:01Z",
      },
      {
        source_id: "x-broken",
        plugin_version: "0.1.0",
        status: "error",
        fetched_count: 0,
        error_message: "cookie expired",
        timestamp: "2026-11-19T00:00:00Z",
        updated_at: "2026-11-19T00:00:01Z",
      },
    ];

    expect(summarizeHeartbeats(beats)).toEqual({
      total: 3,
      ok: 1,
      degraded: 1,
      error: 1,
    });

    const a = makeItem({ id: 1, ai_score: 40, pub_date: 200 });
    const b = makeItem({ id: 2, ai_score: 95, pub_date: 100 });
    expect(sortLoadedArticles([a, b], "time").map((i) => i.id)).toEqual([1, 2]);
    expect(sortLoadedArticles([a, b], "score").map((i) => i.id)).toEqual([
      2, 1,
    ]);

    expect(countActiveAdvancedFilters({})).toBe(0);
    expect(countActiveAdvancedFilters({ min_score: 0 })).toBe(0);
    expect(
      countActiveAdvancedFilters({ priority: "critical", min_score: 60 }),
    ).toBe(2);

    expect(getGreetingKey(8)).toBe("overview.greeting.morning");
    expect(getGreetingKey(15)).toBe("overview.greeting.afternoon");
    expect(getGreetingKey(22)).toBe("overview.greeting.evening");
    expect(isPluginSourceFeed("urn:news_panel:source:x-twifork-main")).toBe(
      true,
    );
    expect(isPluginSourceFeed("https://example.com/rss.xml")).toBe(false);
  });

  it("resolveItemTimestamps prefers RFC 3339 published_at, falls back to pub_date, and resolves created_at separately", () => {
    const withPublishedAt = resolveItemTimestamps({
      published_at: "2026-11-18T12:30:00Z",
      pub_date: 1_700_000_000,
      created_at: 1_795_000_000,
    });
    expect(withPublishedAt).toEqual({
      publishedUnix: Math.floor(Date.parse("2026-11-18T12:30:00Z") / 1000),
      fetchedUnix: 1_795_000_000,
    });

    const fallbackToPubDate = resolveItemTimestamps({
      published_at: "not-a-date",
      pub_date: 1_700_000_123,
      created_at: 1_700_005_000,
    });
    expect(fallbackToPubDate).toEqual({
      publishedUnix: 1_700_000_123,
      fetchedUnix: 1_700_005_000,
    });

    const emptyTimestamps = resolveItemTimestamps({
      published_at: "",
      pub_date: 0,
      created_at: 0,
    });
    expect(emptyTimestamps).toEqual({
      publishedUnix: null,
      fetchedUnix: null,
    });

    expect(formatExactDateTime(null, "en")).toBe("—");
    expect(formatExactDateTime(0, "en")).toBe("—");
    expect(formatExactDateTime(1_795_000_000, "en")).not.toBe("—");
  });
});
