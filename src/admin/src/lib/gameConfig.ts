export interface GameRole {
  name: string;
  required: boolean;
}

export interface GameRank {
  name: string;
  value: number;
}

export interface GameMode {
  name: string;
  slots: number;
  party_size_min: number;
  party_size_max: number;
  roles_required: boolean;
  rank_required: boolean;
  roles: GameRole[];
  ranks: GameRank[];
}

export interface GameConfig {
  genre?: string;
  platforms?: string[];
  regions: string[];
  modes: GameMode[];
}

export function defaultGameConfig(): GameConfig {
  return {
    regions: ["global"],
    modes: [
      {
        name: "Default",
        slots: 1,
        party_size_min: 1,
        party_size_max: 1,
        roles_required: false,
        rank_required: false,
        roles: [],
        ranks: [],
      },
    ],
  };
}

function splitList(raw: string): string[] {
  return raw
    .split(/[,;\n]+/)
    .map((part) => part.trim())
    .filter(Boolean);
}

export function parseListInput(raw: string): string[] {
  return splitList(raw);
}

export function formatListInput(values: string[] | undefined): string {
  return (values ?? []).join(", ");
}

export function validateGameConfig(config: GameConfig): string | undefined {
  if (!config.regions.length) {
    return "At least one region is required.";
  }
  if (!config.modes.length) {
    return "At least one matchmaking mode is required.";
  }
  const modeNames = new Set<string>();
  for (let i = 0; i < config.modes.length; i += 1) {
    const mode = config.modes[i];
    const label = `Mode ${i + 1}`;
    const name = mode.name.trim();
    if (!name) {
      return `${label}: name is required.`;
    }
    const key = name.toLowerCase();
    if (modeNames.has(key)) {
      return `${label}: duplicate mode name "${name}".`;
    }
    modeNames.add(key);
    if (mode.slots <= 0) {
      return `${label}: slots must be positive.`;
    }
    if (mode.party_size_min <= 0 || mode.party_size_max < mode.party_size_min) {
      return `${label}: invalid party size range.`;
    }
    if (mode.party_size_max > mode.slots) {
      return `${label}: party size max cannot exceed slots.`;
    }
    if (mode.roles_required && mode.roles.length === 0) {
      return `${label}: add at least one role or disable roles required.`;
    }
    if (mode.rank_required && mode.ranks.length === 0) {
      return `${label}: add at least one rank or disable rank required.`;
    }
    const roleNames = new Set<string>();
    for (const role of mode.roles) {
      const roleName = role.name.trim();
      if (!roleName) {
        return `${label}: role name is required.`;
      }
      const roleKey = roleName.toLowerCase();
      if (roleNames.has(roleKey)) {
        return `${label}: duplicate role "${roleName}".`;
      }
      roleNames.add(roleKey);
    }
    const rankNames = new Set<string>();
    let prevValue = -1;
    for (const rank of mode.ranks) {
      const rankName = rank.name.trim();
      if (!rankName) {
        return `${label}: rank name is required.`;
      }
      const rankKey = rankName.toLowerCase();
      if (rankNames.has(rankKey)) {
        return `${label}: duplicate rank "${rankName}".`;
      }
      rankNames.add(rankKey);
      if (prevValue >= 0 && rank.value < prevValue) {
        return `${label}: rank values must be non-decreasing.`;
      }
      prevValue = rank.value;
    }
  }
  return undefined;
}

export function serializeGameConfig(config: GameConfig): string {
  const payload = {
    ...(config.genre?.trim() ? { genre: config.genre.trim() } : {}),
    ...(config.platforms?.length ? { platforms: config.platforms } : {}),
    regions: config.regions,
    modes: config.modes.map((mode) => {
      const next: Record<string, unknown> = {
        name: mode.name.trim(),
        slots: mode.slots,
        party_size_min: mode.party_size_min,
        party_size_max: mode.party_size_max,
      };
      if (mode.roles_required) {
        next.roles_required = true;
      }
      if (mode.rank_required) {
        next.rank_required = true;
      }
      if (mode.roles.length) {
        next.roles = mode.roles.map((role) => ({
          name: role.name.trim(),
          ...(role.required ? { required: true } : {}),
        }));
      }
      if (mode.ranks.length) {
        next.ranks = mode.ranks.map((rank) => ({
          name: rank.name.trim(),
          value: rank.value,
        }));
      }
      return next;
    }),
  };
  return JSON.stringify(payload, null, 2);
}

export function parseGameConfigJson(raw: string): GameConfig {
  const parsed = JSON.parse(raw) as Partial<GameConfig>;
  return {
    genre: typeof parsed.genre === "string" ? parsed.genre : "",
    platforms: Array.isArray(parsed.platforms)
      ? parsed.platforms.filter((value): value is string => typeof value === "string")
      : [],
    regions: Array.isArray(parsed.regions)
      ? parsed.regions.filter((value): value is string => typeof value === "string")
      : [],
    modes: Array.isArray(parsed.modes)
      ? parsed.modes.map((mode) => ({
          name: typeof mode?.name === "string" ? mode.name : "",
          slots: Number(mode?.slots) || 0,
          party_size_min: Number(mode?.party_size_min) || 0,
          party_size_max: Number(mode?.party_size_max) || 0,
          roles_required: Boolean(mode?.roles_required),
          rank_required: Boolean(mode?.rank_required),
          roles: Array.isArray(mode?.roles)
            ? mode.roles.map((role) => ({
                name: typeof role?.name === "string" ? role.name : "",
                required: Boolean(role?.required),
              }))
            : [],
          ranks: Array.isArray(mode?.ranks)
            ? mode.ranks.map((rank) => ({
                name: typeof rank?.name === "string" ? rank.name : "",
                value: Number(rank?.value) || 0,
              }))
            : [],
        }))
      : [],
  };
}
