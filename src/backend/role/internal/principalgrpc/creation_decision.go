package principalgrpc

import (
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func validateCreationDecision(p principal.Principal, request proto.Message, requestID, hash string) error {
	deny := status.Error(codes.PermissionDenied, "Space creation decision binding required")
	if p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.RequestID != requestID || requestID == "" || p.RequestHash != hash || hash == "" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return deny
	}
	req, ok := request.(*rolev1.CheckPermissionRequest)
	if !ok || req == nil || req.PermissionName != "TEXT_CHAT_CREATE_IN_SPACE" || req.Chat != nil || req.VoiceRoomId != nil {
		return deny
	}
	for _, value := range []string{req.SpaceId, req.ProfileId} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return deny
		}
	}
	return nil
}
