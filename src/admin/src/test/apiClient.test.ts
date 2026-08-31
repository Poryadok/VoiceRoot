import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiFetch, apiUrl } from "../api/client";
import { setAccessToken } from "../oauth/session";

describe("api client", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubEnv("VITE_VOICE_API_BASE", "http://api.test");
    vi.stubEnv("VITE_OAUTH_DISABLED", "false");
    vi.stubEnv("VITE_STAFF_TOKEN", "");
    vi.stubGlobal("fetch", vi.fn());
  });

  it("apiUrl normalizes path and base trailing slash", () => {
    expect(apiUrl("/api/v1/foo")).toBe("http://api.test/api/v1/foo");
    expect(apiUrl("api/v1/bar")).toBe("http://api.test/api/v1/bar");
  });

  it("apiFetch attaches session access token", async () => {
    setAccessToken("session-token", 3600);
    vi.mocked(fetch).mockResolvedValue(new Response("{}", { status: 200 }));

    await apiFetch("/api/v1/admin/moderation/reports");

    expect(fetch).toHaveBeenCalledWith(
      "http://api.test/api/v1/admin/moderation/reports",
      expect.objectContaining({
        headers: expect.any(Headers),
      }),
    );
    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).get("Authorization")).toBe(
      "Bearer session-token",
    );
  });

  it("apiFetch uses VITE_STAFF_TOKEN when oauth disabled", async () => {
    vi.stubEnv("VITE_OAUTH_DISABLED", "true");
    vi.stubEnv("VITE_STAFF_TOKEN", "dev-staff-jwt");
    vi.mocked(fetch).mockResolvedValue(new Response("{}", { status: 200 }));

    await apiFetch("/api/v1/analytics/dashboard/product");

    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).get("Authorization")).toBe(
      "Bearer dev-staff-jwt",
    );
  });

  it("apiFetch clears session on 401", async () => {
    setAccessToken("expired-token", 3600);
    vi.mocked(fetch).mockResolvedValue(new Response("", { status: 401 }));

    await apiFetch("/api/v1/admin/moderation/reports");

    expect(sessionStorage.getItem("voice_admin_access_token")).toBeNull();
  });

  it("apiFetch sets JSON content type for body requests", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response("{}", { status: 200 }));

    await apiFetch("/api/v1/admin/moderation/reports", {
      method: "POST",
      body: JSON.stringify({ status: "pending" }),
    });

    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).get("Content-Type")).toBe(
      "application/json",
    );
  });
});
