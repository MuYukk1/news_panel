import { describe, expect, it } from "vitest";
import { parseTriageSearch } from "./triage-search";
import { normalizeItemFilters, normalizeBookmarkFilters, queryKeys } from "@/queries/keys";

describe("triage query navigation", () => {
  it("preserves score zero and a selected article in shareable filters", () => {
    expect(parseTriageSearch({ article: "42", min_score: "0", domain: "ai", priority: "high", triage_status: "kept" }))
      .toEqual({ article: 42, min_score: 0, domain: "ai", priority: "high", triage_status: "kept" });
  });
  it("ignores invalid filter values", () => {
    for (const min_score of ["", null, false, -1, 101, 0.5, "bad"]) {
      expect(parseTriageSearch({ min_score, priority: "urgent", triage_status: "unknown" }))
        .toMatchObject({ min_score: undefined, priority: undefined, triage_status: undefined });
    }
  });
  it("isolates caches across triage filters and keeps saved snapshots unfiltered by default", () => {
    expect(queryKeys.items.list({domain:"ai"})).not.toEqual(queryKeys.items.list({domain:"investing"}));
    expect(normalizeItemFilters({}).triageStatus).toBe("kept");
    expect(normalizeBookmarkFilters({}).triageStatus).toBeUndefined();
    expect(normalizeBookmarkFilters({minScore:0}).minScore).toBe(0);
  });
});
