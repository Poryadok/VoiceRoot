package grpcsvc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/roleoverrideprincipal"
)

// GetMemberAdmissionAuthority serves Role observations and Messaging's exact
// installation-bound send observation on the protected listener.
// The store reads one fenced snapshot and never calls Role back.
func (s *SpaceGRPC) GetMemberAdmissionAuthority(ctx context.Context, req *spacev1.GetMemberAdmissionAuthorityRequest) (*spacev1.GetMemberAdmissionAuthorityResponse, error) {
	if err := roleoverrideprincipal.RequireMemberAdmissionAuthority(ctx, req); err != nil {
		return nil, err
	}
	if req.GetBotMessageSendBinding() != nil {
		p, _ := principal.FromContext(ctx)
		deadline := time.Now().Add(5 * time.Second)
		if p.ExpiresAt.Before(deadline) {
			deadline = p.ExpiresAt
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	response, err := s.Store.ReadMemberAdmissionAuthority(ctx, req)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "member authority unavailable")
	}
	if req.GetBotMessageSendBinding() != nil && ctx.Err() != nil {
		return nil, status.Error(codes.Unavailable, "member authority observation expired")
	}
	return response, nil
}
