package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func isProtectedMatchmakingRoute(r *http.Request) bool {
	if r == nil || r.Method != http.MethodPost {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/matchmaking/")
	parts := strings.Split(rest, "/")
	if len(parts) == 3 && parts[0] == "matches" && parts[1] != "" && parts[2] == "complete" {
		return true
	}
	return len(parts) == 4 && parts[0] == "matches" && parts[1] != "" && parts[2] == "voice" &&
		(parts[3] == "join" || parts[3] == "token" || parts[3] == "leave")
}

func (t *transcoder) serveMatchmaking(w http.ResponseWriter, r *http.Request, rest string) bool {
	rest = strings.TrimPrefix(rest, "/")
	switch {
	case strings.HasPrefix(rest, "profile"):
		sub := strings.TrimPrefix(rest, "profile")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingProfile(w, r, sub)
	case rest == "lfp-requests/decide" || strings.HasPrefix(rest, "lfp-requests/"):
		return t.serveMatchmakingLfpDecide(w, r, rest)
	case strings.HasPrefix(rest, "game-requests"):
		sub := strings.TrimPrefix(rest, "game-requests")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingGameRequests(w, r, sub)
	case strings.HasPrefix(rest, "games"):
		sub := strings.TrimPrefix(rest, "games")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingGames(w, r, sub)
	case strings.HasPrefix(rest, "search"):
		sub := strings.TrimPrefix(rest, "search")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingSearch(w, r, sub)
	case strings.HasPrefix(rest, "matches"):
		traceMatchFoundTransport("matchmaking-matches-branch")
		sub := strings.TrimPrefix(rest, "matches")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingMatches(w, r, sub)
	case strings.HasPrefix(rest, "players"):
		sub := strings.TrimPrefix(rest, "players")
		sub = strings.TrimPrefix(sub, "/")
		return t.serveMatchmakingPlayers(w, r, sub)
	case rest == "bans" || strings.HasPrefix(rest, "bans/"):
		return t.serveMatchmakingBans(w, r, rest)
	default:
		return false
	}
}

func (t *transcoder) serveMatchmakingMatches(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if rest == "" {
		traceMatchFoundTransport("matchmaking-matches-empty")
		return false
	}
	parts := strings.Split(rest, "/")
	matchID := parts[0]
	if matchID == "" {
		traceMatchFoundTransport("matchmaking-match-id-empty")
		return false
	}
	if len(parts) == 3 && parts[1] == "voice" {
		return t.serveMatchSquadMember(w, r, matchID, parts[2])
	}

	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		resp, err := t.clients.matchmaking.GetMatch(ctx, &matchmakingv1.GetMatchRequest{MatchId: matchID})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "respond":
		req := &matchmakingv1.RespondToMatchRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.MatchId = matchID
		resp, err := t.clients.matchmaking.RespondToMatch(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "complete":
		traceMatchFoundTransport("matchmaking-complete-post-route")
		req := &matchmakingv1.CompleteMatchRequest{}
		traceMatchFoundTransport("complete-match-decode")
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.MatchId = matchID
		protectedCtx, err := t.matchmakingCompleteContext(r, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchmakingComplete.CompleteMatch(protectedCtx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "rate":
		req := &matchmakingv1.RateMatchRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		req.MatchId = matchID
		resp, err := t.clients.matchmaking.RateMatch(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	default:
		traceMatchFoundTransport("matchmaking-matches-unmatched")
		return false
	}
}

func (t *transcoder) serveMatchSquadMember(w http.ResponseWriter, r *http.Request, matchID, action string) bool {
	parsedMatchID, err := uuid.Parse(matchID)
	if err != nil || parsedMatchID.String() != matchID {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid match id"))
		return true
	}
	if r.Method != http.MethodPost {
		return false
	}
	if t.clients.matchSquadMember == nil {
		writeGRPCError(w, status.Error(codes.Unavailable, "MatchSquad member transport unavailable"))
		return true
	}

	var requestID string
	switch action {
	case "join":
		req := &callsv1.JoinMatchSquadRoomRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MatchId != "" && req.MatchId != matchID {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "match id does not match route"))
			return true
		}
		req.MatchId = matchID
		if req.ProtocolVersion != 1 || !canonicalLifecycleUUID(req.OperationId) || !canonicalLifecycleUUID(req.RoomId) {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid MatchSquad join request"))
			return true
		}
		requestID = req.OperationId
		ctx, err := t.matchSquadMemberContext(r, req, callsv1.MatchSquadMemberService_JoinMatchSquadRoom_FullMethodName, requestID)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchSquadMember.JoinMatchSquadRoom(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case "token":
		req := &callsv1.GetMatchSquadJoinTokenRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MatchId != "" && req.MatchId != matchID {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "match id does not match route"))
			return true
		}
		req.MatchId = matchID
		if req.ProtocolVersion != 1 || !canonicalLifecycleUUID(req.RoomId) || !canonicalLifecycleUUID(req.MediaEpoch) {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid MatchSquad token request"))
			return true
		}
		requestID = uuid.NewString()
		ctx, err := t.matchSquadMemberContext(r, req, callsv1.MatchSquadMemberService_GetMatchSquadJoinToken_FullMethodName, requestID)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchSquadMember.GetMatchSquadJoinToken(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case "leave":
		req := &callsv1.LeaveMatchSquadRoomRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		if req.MatchId != "" && req.MatchId != matchID {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "match id does not match route"))
			return true
		}
		req.MatchId = matchID
		if req.ProtocolVersion != 1 || !canonicalLifecycleUUID(req.OperationId) || !canonicalLifecycleUUID(req.RoomId) || !canonicalLifecycleUUID(req.ExpectedMediaEpoch) {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid MatchSquad leave request"))
			return true
		}
		requestID = req.OperationId
		ctx, err := t.matchSquadMemberContext(r, req, callsv1.MatchSquadMemberService_LeaveMatchSquadRoom_FullMethodName, requestID)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchSquadMember.LeaveMatchSquadRoom(ctx, req)
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

func (t *transcoder) serveMatchmakingPlayers(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if rest == "" {
		return false
	}
	parts := strings.Split(rest, "/")
	profileID := parts[0]
	if profileID == "" {
		return false
	}
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "rating" {
		gameID := r.URL.Query().Get("game_id")
		resp, err := t.clients.matchmaking.GetPlayerRating(ctx, &matchmakingv1.GetPlayerRatingRequest{
			ProfileId: profileID,
			GameId:    gameID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	}
	return false
}

func (t *transcoder) serveMatchmakingBans(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	switch {
	case r.Method == http.MethodPost && rest == "bans":
		req := &matchmakingv1.BanFromMMRequest{}
		if err := readProtoJSON(r, req); err != nil {
			writeGRPCError(w, err)
			return true
		}
		resp, err := t.clients.matchmaking.BanFromMM(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "bans/"):
		target := strings.TrimPrefix(rest, "bans/")
		if target == "" || strings.Contains(target, "/") {
			return false
		}
		resp, err := t.clients.matchmaking.GetMMBanStatus(ctx, &matchmakingv1.GetMMBanStatusRequest{
			TargetProfileId: target,
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

// serveMatchmakingLfpDecide: POST /api/v1/matchmaking/lfp-requests/decide
func (t *transcoder) serveMatchmakingLfpDecide(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if rest != "lfp-requests/decide" || r.Method != http.MethodPost {
		return false
	}
	req := &matchmakingv1.DecideLfpRequestRequest{}
	if err := readProtoJSON(r, req); err != nil {
		writeGRPCError(w, err)
		return true
	}
	resp, err := t.clients.matchmaking.DecideLfpRequest(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
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

// serveMatchmakingGameRequests: POST /api/v1/matchmaking/game-requests (user submit).
func (t *transcoder) serveMatchmakingGameRequests(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)
	if rest != "" {
		return false
	}
	if r.Method != http.MethodPost {
		return false
	}
	req := &matchmakingv1.SubmitGameRequestRequest{}
	if err := readProtoJSON(r, req); err != nil {
		writeGRPCError(w, err)
		return true
	}
	resp, err := t.clients.matchmaking.SubmitGameRequest(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
}

func (t *transcoder) serveMatchmakingProfile(w http.ResponseWriter, r *http.Request, rest string) bool {
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && rest == "me/matches":
		page := &commonv1.CursorPageRequest{}
		_ = decodeQueryJSON(page, queryFirst(r, "page"))
		if page.Cursor == "" {
			page.Cursor = queryFirst(r, "cursor")
		}
		if page.PageSize == 0 {
			page.PageSize = parseInt32Query(queryFirst(r, "page_size"))
		}
		resp, err := t.clients.matchmaking.GetMatchHistory(ctx, &matchmakingv1.GetMatchHistoryRequest{
			Page: page,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

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
