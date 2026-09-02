import { describe, expect, it, vi, beforeEach } from "vitest";
import { fetchDashboard, exportAnalytics } from "../api/analytics";

describe("analytics API client", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  it("fetchDashboard calls staff analytics route", async () => {
    const mock = vi.mocked(fetch);
    mock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ dashboard_type: "product", metrics: [] }),
    } as Response);

    const res = await fetchDashboard("product");
    expect(res.dashboard_type).toBe("product");
    expect(mock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/analytics/dashboard/product"),
      expect.objectContaining({ headers: expect.any(Object) }),
    );
  });

  it("fetchDashboard forwards from/to query params", async () => {
    const mock = vi.mocked(fetch);
    mock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ dashboard_type: "health", metrics: [] }),
    } as Response);

    await fetchDashboard("health", {
      from: "2026-01-01T00:00:00Z",
      to: "2026-01-31T00:00:00Z",
    });
    expect(mock).toHaveBeenCalledWith(
      expect.stringMatching(
        /\/api\/v1\/analytics\/dashboard\/health\?.*from=2026-01-01T00%3A00%3A00Z.*to=2026-01-31T00%3A00%3A00Z/,
      ),
      expect.any(Object),
    );
  });

  it("fetchRetention calls staff retention route", async () => {
    const mock = vi.mocked(fetch);
    mock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ cohorts: [] }),
    } as Response);

    const { fetchRetention } = await import("../api/analytics");
    await fetchRetention();
    expect(mock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/analytics/retention"),
      expect.objectContaining({ headers: expect.any(Object) }),
    );
  });

  it("exportAnalytics requests blob export", async () => {
    const mock = vi.mocked(fetch);
    mock.mockResolvedValueOnce({
      ok: true,
      blob: async () => new Blob(["a,b"], { type: "text/csv" }),
    } as Response);

    const blob = await exportAnalytics("csv", "message_sent");
    expect(blob.type).toBe("text/csv");
    expect(mock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/v1\/analytics\/export\?.*format=csv.*event_type=message_sent/),
      expect.any(Object),
    );
  });
});
