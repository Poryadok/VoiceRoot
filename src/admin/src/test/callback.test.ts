import { describe, expect, it } from "vitest";
import { callbackRedirectUri, parseCallbackSearch } from "../oauth/callback";

describe("oauth callback helpers", () => {
  it("parseCallbackSearch extracts code, state, and error", () => {
    expect(
      parseCallbackSearch("?code=abc&state=xyz"),
    ).toEqual({ code: "abc", state: "xyz", error: null });
    expect(parseCallbackSearch("?error=access_denied")).toEqual({
      code: null,
      state: null,
      error: "access_denied",
    });
  });

  it("callbackRedirectUri appends /callback without trailing slash duplication", () => {
    expect(callbackRedirectUri("https://admin.test")).toBe(
      "https://admin.test/callback",
    );
    expect(callbackRedirectUri("https://admin.test/")).toBe(
      "https://admin.test/callback",
    );
  });
});
