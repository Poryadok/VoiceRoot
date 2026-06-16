package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"

	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) AddBotMember(ctx context.Context, req *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceManageBots); err != nil {
		return nil, err
	}
	if err := s.Store.AddBotMember(ctx, spaceID, profileID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.AddBotMemberResponse{}, nil
}

func (s *SpaceGRPC) RemoveBotMember(ctx context.Context, req *spacev1.RemoveBotMemberRequest) (*spacev1.RemoveBotMemberResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpacePermission(ctx, spaceID, permissions.SpaceManageBots); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveBotMember(ctx, spaceID, profileID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.RemoveBotMemberResponse{}, nil
}
