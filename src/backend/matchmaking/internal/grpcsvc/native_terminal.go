package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/terminalprincipal"
)

func (s *MatchmakingGRPC) nativeRoutePolicy() bool {
	return s != nil && (s.freshRoutes == nativeVoiceFreshRoutes || (s.Matches != nil && s.Matches.NativeAdmission != nil))
}
func (s *MatchmakingGRPC) readActorProfile(ctx context.Context, request proto.Message, method string) (uuid.UUID, bool) {
	if s.nativeRoutePolicy() {
		actor, ok := terminalprincipal.SubjectFromContext(ctx, request, method)
		if !ok || s.Matches == nil || s.Matches.NativeAdmission == nil || s.Matches.NativeAdmission.Validate() != nil {
			return uuid.Nil, false
		}
		return actor.ProfileID, ok
	}
	return authctx.ProfileID(ctx)
}
func (s *MatchmakingGRPC) cancelNativeSearch(ctx context.Context, request *matchmakingv1.CancelSearchRequest) (*matchmakingv1.CancelSearchResponse, error) {
	actor, ok := terminalprincipal.SubjectFromContext(ctx, request, matchmakingv1.MatchmakingService_CancelSearch_FullMethodName)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified cancellation actor required")
	}
	if s.Matches == nil || s.Matches.NativeAdmission == nil {
		return nil, status.Error(codes.Unavailable, "native cancellation unavailable")
	}
	sessionID, err := uuid.Parse(request.SessionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid session id")
	}
	err = s.Matches.CancelNativeSearch(ctx, actor.AccountID, actor.ProfileID, sessionID)
	switch {
	case err == nil:
		return &matchmakingv1.CancelSearchResponse{}, nil
	case errors.Is(err, store.ErrSessionNotFound):
		return nil, status.Error(codes.NotFound, "session not found")
	case errors.Is(err, store.ErrSessionNotSearchable):
		return nil, status.Error(codes.FailedPrecondition, "session not searching")
	case errors.Is(err, store.ErrSourcePartyUnavailable):
		return nil, status.Error(codes.Unavailable, "native cancellation unavailable")
	default:
		return nil, status.Error(codes.Unavailable, "native cancellation unavailable")
	}
}
