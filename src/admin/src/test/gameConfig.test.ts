import { describe, expect, it } from "vitest";
import {
  defaultGameConfig,
  parseGameConfigJson,
  parseListInput,
  serializeGameConfig,
  validateGameConfig,
} from "../lib/gameConfig";

describe("gameConfig", () => {
  it("default config passes validation", () => {
    expect(validateGameConfig(defaultGameConfig())).toBeUndefined();
  });

  it("parseListInput splits comma-separated values", () => {
    expect(parseListInput("eu, cis, global")).toEqual(["eu", "cis", "global"]);
  });

  it("rejects empty regions", () => {
    const config = { ...defaultGameConfig(), regions: [] };
    expect(validateGameConfig(config)).toMatch(/region/i);
  });

  it("rejects roles_required without roles", () => {
    const config = defaultGameConfig();
    config.modes[0].roles_required = true;
    expect(validateGameConfig(config)).toMatch(/role/i);
  });

  it("rejects non-decreasing rank values", () => {
    const config = defaultGameConfig();
    config.modes[0].ranks = [
      { name: "Gold", value: 2000 },
      { name: "Silver", value: 1000 },
    ];
    expect(validateGameConfig(config)).toMatch(/non-decreasing/i);
  });

  it("serializeGameConfig omits empty optional fields", () => {
    const json = serializeGameConfig(defaultGameConfig());
    const parsed = JSON.parse(json) as Record<string, unknown>;
    expect(parsed.regions).toEqual(["global"]);
    expect(parsed.modes).toHaveLength(1);
    expect(parsed.genre).toBeUndefined();
    expect(parsed.platforms).toBeUndefined();
  });

  it("parseGameConfigJson round-trips structured config", () => {
    const raw = serializeGameConfig({
      genre: "MOBA",
      platforms: ["pc"],
      regions: ["eu"],
      modes: [
        {
          name: "5v5 Ranked",
          slots: 10,
          party_size_min: 1,
          party_size_max: 5,
          roles_required: true,
          rank_required: true,
          roles: [{ name: "Carry", required: true }],
          ranks: [
            { name: "Herald", value: 0 },
            { name: "Guardian", value: 770 },
          ],
        },
      ],
    });
    const parsed = parseGameConfigJson(raw);
    expect(parsed.genre).toBe("MOBA");
    expect(parsed.modes[0].roles[0].name).toBe("Carry");
    expect(validateGameConfig(parsed)).toBeUndefined();
  });
});
