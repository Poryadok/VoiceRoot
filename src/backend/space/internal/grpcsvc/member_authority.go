package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/roleoverrideprincipal"
)

// GetMemberAdmissionAuthority serves only Role's protected observation route.
// The store reads one fenced snapshot and never calls Role back.
func (s *SpaceGRPC) GetMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	if err := roleoverrideprincipal.RequireMemberAdmissionAuthority(ctx, req); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	response, err := s.Store.ReadMemberAdmissionAuthority(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	return response, nil
}
