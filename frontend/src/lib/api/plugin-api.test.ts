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
          enabled: true,
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

  it("serializes scored and unscored parameters and patches plugin enabled state", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.startsWith("/api/v1/plugins/")) {
        return new Response(
          JSON.stringify({
            data: {
              source_id: "x-twifork-main",
              plugin_version: "0.1.0",
              status: "ok",
              fetched_count: 5,
              error_message: null,
              timestamp: "2026-11-20T00:00:00Z",
              updated_at: "2026-11-20T00:00:01Z",
              enabled: false,
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        );
      }
      return new Response(
        JSON.stringify({ data: [], total: 0, next_cursor: null }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });

    await itemAPI.list({ triage_status: "kept", scored: true, feed_id: 9 });
    const keptQuery = new URLSearchParams(
      String(fetchMock.mock.calls[0]?.[0]).split("?")[1] ?? "",
    );
    expect(keptQuery.get("triage_status")).toBe("kept");
    expect(keptQuery.get("scored")).toBe("true");
    expect(keptQuery.get("feed_id")).toBe("9");

    await itemAPI.list({ triage_status: "unscored" });
    const unscoredQuery = new URLSearchParams(
      String(fetchMock.mock.calls[1]?.[0]).split("?")[1] ?? "",
    );
    expect(unscoredQuery.get("triage_status")).toBe("unscored");
    expect(unscoredQuery.get("scored")).toBeNull();

    const updated = await pluginAPI.update("x-twifork-main", { enabled: false });
    expect(fetchMock.mock.calls[2]?.[0]).toBe("/api/v1/plugins/x-twifork-main");
    expect(fetchMock.mock.calls[2]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({ enabled: false }),
    });
    expect(updated.data?.enabled).toBe(false);
  });
});
