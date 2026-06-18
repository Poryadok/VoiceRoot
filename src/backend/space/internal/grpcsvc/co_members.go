package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) AreCoMembers(ctx context.Context, req *spacev1.AreCoMembersRequest) (*spacev1.AreCoMembersResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	profileA, err := parseUUIDField("profile_id_a", req.GetProfileIdA())
	if err != nil {
		return nil, err
	}
	profileB, err := parseUUIDField("profile_id_b", req.GetProfileIdB())
	if err != nil {
		return nil, err
	}
	var spaceIDs []uuid.UUID
	for _, sid := range req.GetSpaceIds() {
		id, parseErr := parseUUIDField("space_ids", sid)
		if parseErr != nil {
			return nil, parseErr
		}
		spaceIDs = append(spaceIDs, id)
	}
	ok, err := s.Store.AreCoMembers(ctx, profileA, profileB, spaceIDs)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.AreCoMembersResponse{CoMembers: ok}, nil
}
