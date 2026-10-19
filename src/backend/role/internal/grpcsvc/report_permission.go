package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/principalgrpc"
)

// This read uses the exact native member/chat coordinator, including its final
// post-owning-COMMIT expiry fence. The selector carries bound request data only;
// it is not a new principal or permission capability.
func (s *RoleGRPC) checkQualifiedReportPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if err := principalgrpc.RequireReportPermission(ctx, req); err != nil {
		return nil, err
	}
	unavailable := status.Error(codes.Unavailable, "report permission unavailable")
	if s == nil || s.Store == nil {
		return nil, unavailable
	}
	native, err := s.Store.NativeMembershipRequired(ctx)
	if err != nil || !native {
		return nil, unavailable
	}
	scoped := context.WithValue(ctx, nativeMemberChatScopeKey{}, nativeMemberChatScope{profile: req.ProfileId, chat: req.Chat.Id, voicePresent: req.VoiceRoomId != nil})
	response, err := ordinarySpaceRPC(scoped, s, req.SpaceId, func(callCtx context.Context, within *RoleGRPC) (*rolev1.CheckPermissionResponse, error) {
		return within.checkPermission(callCtx, req)
	}, req.ProfileId)
	if err != nil {
		switch status.Code(err) {
		case codes.PermissionDenied:
			return &rolev1.CheckPermissionResponse{Allowed: false}, nil
		default:
			return nil, unavailable
		}
	}
	if response == nil {
		return nil, unavailable
	}
	return response, nil
}
