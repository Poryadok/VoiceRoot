import { describe, expect, it } from "vitest";
import { buildResolutionJson } from "../lib/resolutionJson";

describe("buildResolutionJson", () => {
  it("returns empty object JSON when note is blank", () => {
    expect(buildResolutionJson()).toBe("{}");
    expect(buildResolutionJson("   ")).toBe("{}");
  });

  it("serializes trimmed note", () => {
    expect(buildResolutionJson("  Spam confirmed  ")).toBe(
      JSON.stringify({ note: "Spam confirmed" }),
    );
  });
});
