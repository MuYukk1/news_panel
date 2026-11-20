import { describe, expect, it } from "vitest";
import {
  areAllVisibleSelected,
  collectMissingGroupNames,
  discoveredToCandidates,
  filterImportCandidates,
  mapCandidatesToBatchItems,
  parsedOpmlToCandidates,
  toggleAllVisibleSelection,
  toggleCandidateSelection,
} from "./feed-import";

describe("feed-import multi-select and OPML helpers", () => {
  it("normalizes and deduplicates discovered feeds and parsed OPML entries", () => {
    const fromDiscovered = discoveredToCandidates([
      {
        title: "  AI Research  ",
        link: "https://example.com/ai.xml",
        site_url: "https://example.com",
        group_name: "AI Labs",
      },
      {
        title: "Duplicate Link",
        link: "https://example.com/ai.xml",
      },
      {
        title: "",
        link: "https://example.com/no-title.xml",
      },
    ]);

    expect(fromDiscovered).toEqual([
      {
        name: "AI Research",
        link: "https://example.com/ai.xml",
        siteUrl: "https://example.com",
        groupName: "AI Labs",
      },
      {
        name: "https://example.com/no-title.xml",
        link: "https://example.com/no-title.xml",
        siteUrl: undefined,
        groupName: undefined,
      },
    ]);

    const fromOpml = parsedOpmlToCandidates([
      {
        name: "Paper Weekly",
        link: "https://example.com/pw.xml",
        siteUrl: "https://example.com/pw",
        groupName: "CN AI",
      },
      {
        name: "Duplicate",
        link: "https://example.com/pw.xml",
      },
    ]);
    expect(fromOpml).toHaveLength(1);
    expect(fromOpml[0]).toMatchObject({
      name: "Paper Weekly",
      groupName: "CN AI",
    });
  });

  it("filters candidates by name, URL, or group and toggles visible selection cleanly", () => {
    const candidates = [
      {
        name: "OpenAI News",
        link: "https://openai.com/rss.xml",
        groupName: "AI Labs",
      },
      {
        name: "Macro Catalyst",
        link: "https://finance.example.com/feed.xml",
        groupName: "Investing",
      },
      {
        name: "Anthropic",
        link: "https://anthropic.com/feed.xml",
        groupName: "AI Labs",
      },
    ];

    const aiFiltered = filterImportCandidates(candidates, "ai labs");
    expect(aiFiltered.map((c) => c.name)).toEqual(["OpenAI News", "Anthropic"]);

    let selected = new Set<string>(candidates.map((c) => c.link));
    expect(areAllVisibleSelected(selected, aiFiltered)).toBe(true);

    // Deselecting all visible only removes visible matches, keeping non-matching items selected.
    selected = toggleAllVisibleSelection(selected, aiFiltered);
    expect(selected.has("https://openai.com/rss.xml")).toBe(false);
    expect(selected.has("https://anthropic.com/feed.xml")).toBe(false);
    expect(selected.has("https://finance.example.com/feed.xml")).toBe(true);

    // Selecting all visible adds them back.
    selected = toggleAllVisibleSelection(selected, aiFiltered);
    expect(selected.size).toBe(3);

    // Toggling single item flips only that link.
    selected = toggleCandidateSelection(selected, "https://openai.com/rss.xml");
    expect(selected.has("https://openai.com/rss.xml")).toBe(false);
    expect(selected.size).toBe(2);
  });

  it("collects missing group names and maps auto vs single group payloads", () => {
    const candidates = [
      {
        name: "Feed A",
        link: "https://a.example.com/rss",
        siteUrl: "https://a.example.com",
        groupName: "Existing",
      },
      {
        name: "Feed B",
        link: "https://b.example.com/rss",
        groupName: "New Group",
      },
      {
        name: "Feed C",
        link: "https://c.example.com/rss",
      },
    ];

    const missing = collectMissingGroupNames(candidates, new Set(["Existing"]));
    expect(missing).toEqual(["New Group"]);

    const groupMap = new Map<string, number>([
      ["Existing", 1],
      ["New Group", 2],
    ]);

    const autoPayload = mapCandidatesToBatchItems(candidates, {
      groupMode: "auto",
      targetGroupId: 9,
      groupNameToId: groupMap,
      fallbackGroupId: 1,
    });
    expect(autoPayload).toEqual([
      {
        group_id: 1,
        name: "Feed A",
        link: "https://a.example.com/rss",
        site_url: "https://a.example.com",
      },
      {
        group_id: 2,
        name: "Feed B",
        link: "https://b.example.com/rss",
      },
      {
        group_id: 1,
        name: "Feed C",
        link: "https://c.example.com/rss",
      },
    ]);

    const singlePayload = mapCandidatesToBatchItems(candidates, {
      groupMode: "single",
      targetGroupId: 9,
      groupNameToId: groupMap,
      fallbackGroupId: 1,
    });
    expect(singlePayload.every((item) => item.group_id === 9)).toBe(true);
  });
});
