package principalgrpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
)

// WorkloadDecisionIssuer selects a fixed verifier from the requested purpose,
// never from unverified credential claims or caller-controlled endpoints.
func WorkloadDecisionIssuer(permission string) string {
	switch permission {
	case permissions.TextChatSendMessages, permissions.TextChatPinMessages, permissions.TextChatAddReactions,
		permissions.TextChatMentionAllInChat, permissions.TextChatMentionAllOnline, permissions.TextChatCreateThreads, permissions.TextChatSendInThreads:
		return "messaging"
	case permissions.VoiceJoin, permissions.VoiceSpeak, permissions.VoiceScreenShare, permissions.VoiceMuteOthers, permissions.VoiceMoveOthers:
		return "voice"
	default:
		return ""
	}
}

// RequireWorkloadDecision permits only decisions about an explicit subject and
// owning-service node. It grants no actor identity or mutation authority.
func RequireWorkloadDecision(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	issuer := WorkloadDecisionIssuer(req.GetPermissionName())
	p, ok := principal.FromContext(ctx)
	if issuer == "" || !ok || p.Kind != "service" || p.Issuer != issuer || p.Subject != "service:"+issuer || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" || !p.ExpiresAt.After(time.Now()) {
		return status.Error(codes.Unauthenticated, "verified scoped workload decision required")
	}
	if req == nil || unknownRoleRequest(req.ProtoReflect(), 0) || !publicRoleUUID(req.SpaceId) || !publicRoleUUID(req.ProfileId) || !publicRoleUUID(p.RequestID) {
		return status.Error(codes.InvalidArgument, "canonical workload decision identities required")
	}
	switch issuer {
	case "messaging":
		if req.Chat == nil || req.Chat.Type == nil || req.VoiceRoomId != nil || !publicRoleUUID(req.Chat.Id) || (req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP && req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_CHANNEL) {
			return status.Error(codes.InvalidArgument, "exact group or channel decision required")
		}
	case "voice":
		if req.Chat != nil || req.VoiceRoomId == nil || !publicRoleUUID(req.GetVoiceRoomId()) {
			return status.Error(codes.InvalidArgument, "exact voice room decision required")
		}
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.RequestHash != hash {
		return status.Error(codes.Unauthenticated, "workload decision body binding mismatch")
	}
	return nil
}
