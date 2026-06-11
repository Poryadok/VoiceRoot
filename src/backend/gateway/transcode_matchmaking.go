package main

import (
	"net/http"
	"strings"

	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func (t *transcoder) serveMatchmaking(w http.ResponseWriter, r *http.Request, rest string) bool {
	rest = strings.TrimPrefix(rest, "/")
	switch {
	case strings.HasPrefix(rest, "profile"):
		sub := strings.TrimPrefix(rest, "profile")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingProfile(w, r, sub)
	case strings.HasPrefix(rest, "games"):
		sub := strings.TrimPrefix(rest, "games")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingGames(w, r, sub)
	case strings.HasPrefix(rest, "search"):
		sub := strings.TrimPrefix(rest, "search")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingSearch(w, r, sub)
	default:
		return false
	}
}

func (t *transcoder) serveMatchmakingSearch(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodPost && rest == "":
		req := &matchmakingv1.StartSearchRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchmaking.StartSearch(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.matchmaking.GetSearchStatus(ctx, &matchmakingv1.GetSearchStatusRequest{
			SessionId: rest,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && rest != "" && !strings.Contains(rest, "/"):
		_, err := t.clients.matchmaking.CancelSearch(ctx, &matchmakingv1.CancelSearchRequest{
			SessionId: rest,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, &matchmakingv1.CancelSearchResponse{})
		return true

	default:
		return false
	}
}

func (t *transcoder) serveMatchmakingGames(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.matchmaking.ListGames(ctx, &matchmakingv1.ListGamesRequest{Page: page})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "search":
		resp, err := t.clients.matchmaking.SearchGames(ctx, &matchmakingv1.SearchGamesRequest{
			Query: queryFirst(r, "query"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.matchmaking.GetGame(ctx, &matchmakingv1.GetGameRequest{GameId: rest})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && rest == "":
		req := &matchmakingv1.CreateGameRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchmaking.CreateGame(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPatch && rest != "" && !strings.Contains(rest, "/"):
		req := &matchmakingv1.UpdateGameRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.GameId = rest
		resp, err := t.clients.matchmaking.UpdateGame(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		return false
	}
}

func (t *transcoder) serveMatchmakingProfile(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "me":
		resp, err := t.clients.matchmaking.GetMyPlayerProfile(ctx, &matchmakingv1.GetMyPlayerProfileRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest != "" && !strings.Contains(rest, "/"):
		resp, err := t.clients.matchmaking.GetPlayerProfile(ctx, &matchmakingv1.GetPlayerProfileRequest{
			ProfileId: rest,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPut && strings.HasPrefix(rest, "games/"):
		gameID := strings.TrimPrefix(rest, "games/")
		if gameID == "" || strings.Contains(gameID, "/") {
			return false
		}
		req := &matchmakingv1.UpsertPlayerGameEntryRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.GameId = gameID
		resp, err := t.clients.matchmaking.UpsertPlayerGameEntry(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodDelete && strings.HasPrefix(rest, "games/"):
		gameID := strings.TrimPrefix(rest, "games/")
		if gameID == "" || strings.Contains(gameID, "/") {
			return false
		}
		resp, err := t.clients.matchmaking.DeletePlayerGameEntry(ctx, &matchmakingv1.DeletePlayerGameEntryRequest{
			GameId: gameID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		return false
	}
}
