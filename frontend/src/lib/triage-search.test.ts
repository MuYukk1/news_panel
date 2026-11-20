import { describe, expect, it } from "vitest";
import { parseTriageSearch } from "./triage-search";
import { normalizeItemFilters, normalizeBookmarkFilters, queryKeys } from "@/queries/keys";
import { buildListItemsParams } from "@/queries/items";
import { buildListBookmarksParams } from "@/queries/bookmarks";

describe("triage query navigation", () => {
  it("preserves score zero and a selected article in shareable filters", () => {
    expect(parseTriageSearch({ article: "42", min_score: "0", domain: "ai", priority: "high", triage_status: "kept" }))
      .toEqual({ article: 42, min_score: 0, domain: "ai", priority: "high", triage_status: "kept", feed_id: undefined });
  });
  it("ignores invalid filter values", () => {
    for (const min_score of ["", null, false, -1, 101, 0.5, "bad"]) {
      expect(parseTriageSearch({ min_score, priority: "urgent", triage_status: "unknown" }))
        .toMatchObject({ min_score: undefined, priority: undefined, triage_status: undefined });
    }
    for (const feed_id of ["", null, false, 0, -3, 12.5, "12.5", "bad"]) {
      expect(parseTriageSearch({ feed_id }).feed_id).toBeUndefined();
    }
  });
  it("supports unscored triage_status and positive integer feed_id", () => {
    expect(parseTriageSearch({ triage_status: "unscored", feed_id: "15" })).toEqual({
      article: undefined,
      triage_status: "unscored",
      domain: undefined,
      priority: undefined,
      min_score: undefined,
      feed_id: 15,
    });
  });
  it("isolates caches across triage filters and keeps saved snapshots unfiltered by default", () => {
    expect(queryKeys.items.list({domain:"ai"})).not.toEqual(queryKeys.items.list({domain:"investing"}));
    expect(normalizeItemFilters({}).triageStatus).toBe("kept");
    expect(normalizeItemFilters({triageStatus:"all"}).triageStatus).toBe("all");
    expect(normalizeItemFilters({triageStatus:"unscored"}).triageStatus).toBe("unscored");
    expect(normalizeBookmarkFilters({}).triageStatus).toBeUndefined();
    expect(normalizeBookmarkFilters({minScore:0}).minScore).toBe(0);
  });
  it("supports explicit triage_status=all and trims whitespace domain", () => {
    expect(parseTriageSearch({ triage_status: "all", domain: "  ai  " })).toEqual({
      article: undefined,
      triage_status: "all",
      domain: "ai",
      priority: undefined,
      min_score: undefined,
      feed_id: undefined,
    });
    expect(parseTriageSearch({ domain: "   " }).domain).toBeUndefined();
  });
  it("maps kept to scored=true and unscored to virtual triage_status in API params", () => {
    const keptParams = buildListItemsParams(normalizeItemFilters({ triageStatus: "kept", feedId: 7 }), null, 20);
    expect(keptParams).toMatchObject({ triage_status: "kept", scored: true, feed_id: 7, limit: 20 });

    const unscoredParams = buildListItemsParams(normalizeItemFilters({ triageStatus: "unscored" }), null, 20);
    expect(unscoredParams).toMatchObject({ triage_status: "unscored", scored: undefined });

    const allParams = buildListItemsParams(normalizeItemFilters({ triageStatus: "all" }), null, 20);
    expect(allParams.triage_status).toBeUndefined();
    expect(allParams.scored).toBeUndefined();

    const bmKept = buildListBookmarksParams(normalizeBookmarkFilters({ triageStatus: "kept" }), null, 50);
    expect(bmKept).toMatchObject({ triage_status: "kept", scored: true });

    const bmUnscored = buildListBookmarksParams(normalizeBookmarkFilters({ triageStatus: "unscored" }), null, 50);
    expect(bmUnscored).toMatchObject({ triage_status: "unscored", scored: undefined });
  });
});
