package main

import (
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"net/http"
	"strings"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

func isProtectedMatchmakingTerminalRoute(r *http.Request) bool {
	if r == nil {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/matchmaking/"), "/")
	if r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "profile" && parts[1] == "me" && parts[2] == "matches" {
		return true
	}
	if len(parts) != 2 || parts[1] == "" {
		return false
	}
	return (r.Method == http.MethodGet && parts[0] == "matches") || ((r.Method == http.MethodGet || r.Method == http.MethodDelete) && parts[0] == "search")
}
func validMatchmakingTerminalRequest(request proto.Message, method string) bool {
	if request == nil || !request.ProtoReflect().IsValid() || len(request.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	switch method {
	case matchmakingv1.MatchmakingService_GetMatch_FullMethodName:
		r, ok := request.(*matchmakingv1.GetMatchRequest)
		return ok && canonicalLifecycleUUID(r.MatchId)
	case matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName:
		r, ok := request.(*matchmakingv1.GetSearchStatusRequest)
		return ok && canonicalLifecycleUUID(r.SessionId)
	case matchmakingv1.MatchmakingService_CancelSearch_FullMethodName:
		r, ok := request.(*matchmakingv1.CancelSearchRequest)
		return ok && canonicalLifecycleUUID(r.SessionId)
	case matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName:
		r, ok := request.(*matchmakingv1.GetMatchHistoryRequest)
		return ok && (r.ProfileId == "" || canonicalLifecycleUUID(r.ProfileId))
	default:
		return false
	}
}
func (t *transcoder) serveProtectedMatchmakingTerminal(w http.ResponseWriter, r *http.Request, request proto.Message, method string) bool {
	if !validMatchmakingTerminalRequest(request, method) {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid matchmaking request"))
		return true
	}
	// These existing read/idempotent-session APIs have no business operation
	// field. This fresh identifier binds one transport attempt, never a new
	// StartSearch/Respond business operation or source authority.
	ctx, err := t.publicMatchmakingContext(r, request, method, uuid.NewString())
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	var response proto.Message
	switch method {
	case matchmakingv1.MatchmakingService_GetMatch_FullMethodName:
		response, err = t.clients.matchmakingComplete.GetMatch(ctx, request.(*matchmakingv1.GetMatchRequest))
	case matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName:
		response, err = t.clients.matchmakingComplete.GetSearchStatus(ctx, request.(*matchmakingv1.GetSearchStatusRequest))
	case matchmakingv1.MatchmakingService_CancelSearch_FullMethodName:
		response, err = t.clients.matchmakingComplete.CancelSearch(ctx, request.(*matchmakingv1.CancelSearchRequest))
	case matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName:
		response, err = t.clients.matchmakingComplete.GetMatchHistory(ctx, request.(*matchmakingv1.GetMatchHistoryRequest))
	}
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, response)
	return true
}
