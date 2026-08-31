import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { staffProfileIdFromToken } from "../lib/jwt";

describe("staffProfileIdFromToken", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubEnv("VITE_STAFF_TOKEN", "");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("reads profile_id from OAuth session JWT", () => {
    const payload = btoa(JSON.stringify({ profile_id: "oauth-profile" }));
    sessionStorage.setItem("voice_admin_access_token", `hdr.${payload}.sig`);
    expect(staffProfileIdFromToken()).toBe("oauth-profile");
  });

  it("falls back to VITE_STAFF_TOKEN when session is empty", () => {
    const payload = btoa(JSON.stringify({ profile_id: "env-profile" }));
    vi.stubEnv("VITE_STAFF_TOKEN", `hdr.${payload}.sig`);
    expect(staffProfileIdFromToken()).toBe("env-profile");
  });

  it("prefers session token over env staff token", () => {
    const sessionPayload = btoa(JSON.stringify({ profile_id: "session-profile" }));
    const envPayload = btoa(JSON.stringify({ profile_id: "env-profile" }));
    sessionStorage.setItem("voice_admin_access_token", `hdr.${sessionPayload}.sig`);
    vi.stubEnv("VITE_STAFF_TOKEN", `hdr.${envPayload}.sig`);
    expect(staffProfileIdFromToken()).toBe("session-profile");
  });

  it("reads profileId camelCase claim", () => {
    const payload = btoa(JSON.stringify({ profileId: "camel-profile" }));
    sessionStorage.setItem("voice_admin_access_token", `hdr.${payload}.sig`);
    expect(staffProfileIdFromToken()).toBe("camel-profile");
  });

  it("does not fall back to sub for assigned_to_profile_id", () => {
    const payload = btoa(JSON.stringify({ sub: "account-or-subject" }));
    sessionStorage.setItem("voice_admin_access_token", `hdr.${payload}.sig`);
    expect(staffProfileIdFromToken()).toBeUndefined();
  });
});
