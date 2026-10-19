package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/permissions"
)

func (s *RoleGRPC) checkQualifiedJoinApproval(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if err := principalgrpc.RequireJoinApprovalPermission(ctx, req); err != nil {
		return nil, err
	}
	return s.checkQualifiedSpaceDecision(ctx, req, permissions.SpaceApproveJoinRequests)
}

func (s *RoleGRPC) checkQualifiedSettings(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if err := principalgrpc.RequireSettingsPermission(ctx, req); err != nil {
		return nil, err
	}
	return s.checkQualifiedSpaceDecision(ctx, req, permissions.SpaceManageSettings)
}

func (s *RoleGRPC) checkQualifiedBotManagement(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if err := principalgrpc.RequireBotManagementPermission(ctx, req); err != nil {
		return nil, err
	}
	return s.checkQualifiedSpaceDecision(ctx, req, permissions.SpaceManageBots)
}

func (s *RoleGRPC) checkQualifiedSpaceDecision(ctx context.Context, req *rolev1.CheckPermissionRequest, permission string) (*rolev1.CheckPermissionResponse, error) {
	if s == nil || s.Store == nil || s.MemberAuthority == nil {
		return nil, status.Error(codes.Unavailable, "native membership authority unavailable")
	}
	// The owner RPC completes before the first Role lock. Its original bounded
	// deadline remains in the immutable observation while the local fence waits.
	observation, err := s.MemberAuthority.ObserveMember(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: req.SpaceId, ProfileId: req.ProfileId})
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	observed, err := memberauthority.WithObservations(ctx, observation)
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	mask, err := s.Store.GetQualifiedEffectiveMask(observed, uuid.MustParse(req.SpaceId), uuid.MustParse(req.ProfileId), nil, nil)
	if err != nil {
		return nil, memberAdmissionError(ctx, err)
	}
	allowed, err := permissions.HasPermission(mask, permission)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "approval policy unavailable")
	}
	return &rolev1.CheckPermissionResponse{Allowed: allowed}, nil
}
