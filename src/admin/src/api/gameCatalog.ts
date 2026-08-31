import { apiJson } from "./client";

export interface CatalogGame {
  id: string;
  name: string;
  status?: string;
  icon_url?: string;
  config_json?: string;
}

export interface SearchGamesResponse {
  games?: CatalogGame[];
}

export interface CreateGameResponse {
  game?: CatalogGame;
}

export interface CreateGameInput {
  name: string;
  config_json: string;
  icon_url?: string;
}

export function searchGames(query: string): Promise<SearchGamesResponse> {
  const params = new URLSearchParams({ query });
  return apiJson(`/api/v1/matchmaking/games/search?${params.toString()}`);
}

export function createGame(input: CreateGameInput): Promise<CreateGameResponse> {
  return apiJson("/api/v1/matchmaking/games", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/** Case-insensitive exact name match against catalog search results. */
export function findDuplicateName(
  name: string,
  games: CatalogGame[] | undefined,
): CatalogGame | undefined {
  const normalized = name.trim().toLowerCase();
  if (!normalized) {
    return undefined;
  }
  return games?.find((g) => g.name.trim().toLowerCase() === normalized);
}
