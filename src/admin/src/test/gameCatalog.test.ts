import { describe, expect, it } from "vitest";
import { findDuplicateName } from "../api/gameCatalog";

describe("gameCatalog helpers", () => {
  it("findDuplicateName matches case-insensitively", () => {
    const games = [
      { id: "1", name: "Apex Legends" },
      { id: "2", name: "Dota 2" },
    ];
    expect(findDuplicateName("apex legends", games)?.id).toBe("1");
    expect(findDuplicateName("  DOTA 2 ", games)?.id).toBe("2");
    expect(findDuplicateName("Counter-Strike", games)).toBeUndefined();
  });
});
