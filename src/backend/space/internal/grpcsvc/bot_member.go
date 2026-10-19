package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

// A delegated-user service cannot impersonate the Bot installation producer,
// even when its actor has SPACE_MANAGE_BOTS.
func (s *SpaceGRPC) AddBotMember(context.Context, *spacev1.AddBotMemberRequest) (*spacev1.AddBotMemberResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}
func (s *SpaceGRPC) RemoveBotMember(context.Context, *spacev1.RemoveBotMemberRequest) (*spacev1.RemoveBotMemberResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}
