import { apiJson } from "./client";

export interface CatalogGame {
  id: string;
  name: string;
  status: string;
  config_json?: string;
  created_by_profile_id?: string;
}

export interface ListGameRequestsResponse {
  game_list?: {
    games?: CatalogGame[];
    next_cursor?: string;
  };
}

export function listGameRequests(status = "pending_moderation"): Promise<ListGameRequestsResponse> {
  const search = new URLSearchParams();
  if (status) search.set("status", status);
  return apiJson(`/api/v1/admin/matchmaking/game-requests?${search}`);
}

export function approveGameRequest(gameId: string): Promise<{ game?: CatalogGame }> {
  return apiJson(`/api/v1/admin/matchmaking/game-requests/${gameId}/approve`, {
    method: "POST",
    body: "{}",
  });
}

export function rejectGameRequest(gameId: string): Promise<{ game?: CatalogGame }> {
  return apiJson(`/api/v1/admin/matchmaking/game-requests/${gameId}/reject`, {
    method: "POST",
    body: "{}",
  });
}
