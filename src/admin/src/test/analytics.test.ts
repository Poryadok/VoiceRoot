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
