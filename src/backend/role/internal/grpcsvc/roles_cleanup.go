package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
)

func (s *RoleGRPC) DeleteRolesCreatedByProfile(ctx context.Context, req *rolev1.DeleteRolesCreatedByProfileRequest) (*rolev1.DeleteRolesCreatedByProfileResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "role persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("created_by_profile_id", req.GetCreatedByProfileId())
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.DeleteRolesCreatedByProfile(ctx, spaceID, profileID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &rolev1.DeleteRolesCreatedByProfileResponse{}, nil
}
