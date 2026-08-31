import { describe, expect, it } from "vitest";
import {
  TEMP_BAN_DEFAULT_DAYS,
  tempBanExpiresAtIso,
} from "../lib/tempBanExpiry";

describe("tempBanExpiresAtIso", () => {
  it("adds clamped days in UTC", () => {
    const now = new Date("2026-06-14T12:00:00.000Z");
    expect(tempBanExpiresAtIso(TEMP_BAN_DEFAULT_DAYS, now)).toBe(
      "2026-06-21T12:00:00.000Z",
    );
  });

  it("clamps below minimum and above maximum", () => {
    const now = new Date("2026-01-01T00:00:00.000Z");
    expect(tempBanExpiresAtIso(0, now)).toBe("2026-01-02T00:00:00.000Z");
    expect(tempBanExpiresAtIso(99, now)).toBe("2026-01-31T00:00:00.000Z");
  });
});
