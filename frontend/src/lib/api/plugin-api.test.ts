import { afterEach, describe, expect, it, vi } from "vitest";
import { itemAPI, pluginAPI } from "./index";

describe("pluginAPI and itemAPI contract mapping", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("fetches external plugin heartbeats from /api/v1/plugins/heartbeats", async () => {
    const payload = {
      data: [
        {
          source_id: "x-twifork-main",
          plugin_version: "0.1.0",
          status: "ok" as const,
          fetched_count: 8,
          error_message: null,
          timestamp: "2026-11-19T00:00:00Z",
          updated_at: "2026-11-19T00:00:02Z",
        },
      ],
      total: 1,
    };

    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(payload), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const res = await pluginAPI.heartbeats();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/v1/plugins/heartbeats");
    expect(res).toEqual(payload);
  });

  it("serializes triage filters and preserves min_score=0 while omitting undefined triage_status", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({ data: [], total: 0, next_cursor: null }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await itemAPI.list({
      domain: "ai",
      priority: "high",
      min_score: 0,
      unread: true,
      limit: 25,
    });

    const requestedUrl = String(fetchMock.mock.calls[0]?.[0]);
    const query = new URLSearchParams(requestedUrl.split("?")[1] ?? "");

    expect(requestedUrl.startsWith("/api/items?")).toBe(true);
    expect(query.get("triage_status")).toBeNull();
    expect(query.get("domain")).toBe("ai");
    expect(query.get("priority")).toBe("high");
    expect(query.get("min_score")).toBe("0");
    expect(query.get("unread")).toBe("true");
    expect(query.get("limit")).toBe("25");
  });
});
