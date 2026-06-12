package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/matchmaking/internal/authctx"
	"voice/backend/matchmaking/internal/store"

	matchmakingv1 "voice.app/voice/matchmaking/v1"
)

// GetMatchHistory returns paginated match squads for the authenticated profile.
func (s *MatchmakingGRPC) GetMatchHistory(ctx context.Context, req *matchmakingv1.GetMatchHistoryRequest) (*matchmakingv1.GetMatchHistoryResponse, error) {
	profileID, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing profile")
	}
	if s.Matches == nil {
		return nil, status.Error(codes.Unavailable, "match unavailable")
	}

	requested := strings.TrimSpace(req.GetProfileId())
	if requested != "" {
		reqID, err := uuid.Parse(requested)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
		}
		if reqID != profileID {
			return nil, status.Error(codes.PermissionDenied, "cannot read another profile history")
		}
	}

	pageSize := int32(50)
	var cursor string
	if req.GetPage() != nil {
		if req.GetPage().GetPageSize() > 0 {
			pageSize = req.GetPage().GetPageSize()
		}
		cursor = req.GetPage().GetCursor()
	}

	res, err := s.Matches.ListHistoryForProfile(ctx, store.ListMatchHistoryParams{
		ProfileID: profileID,
		Cursor:    cursor,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list match history: %v", err)
	}

	matches := make([]*matchmakingv1.Match, 0, len(res.Matches))
	for _, m := range res.Matches {
		matches = append(matches, toProtoMatch(m))
	}
	return &matchmakingv1.GetMatchHistoryResponse{
		MatchList: &matchmakingv1.MatchList{
			Matches:    matches,
			NextCursor: res.NextCursor,
		},
	}, nil
}
