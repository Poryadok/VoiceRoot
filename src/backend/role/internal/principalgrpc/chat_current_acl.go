package principalgrpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// RequireChatCurrentACL admits only Chat's current public-result disclosure
// decision. Subject/Space/Chat are Chat-owned immutable intent data, not actors.
func RequireChatCurrentACL(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "chat" || p.Subject != "service:chat" || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" || !p.ExpiresAt.After(time.Now()) {
		return status.Error(codes.Unauthenticated, "verified current Chat ACL principal required")
	}
	if req == nil || req.PermissionName != "TEXT_CHAT_VIEW" || req.Chat == nil || req.Chat.Type == nil || req.VoiceRoomId != nil {
		return status.Error(codes.PermissionDenied, "exact Chat VIEW disclosure decision required")
	}
	if req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP && req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return status.Error(codes.InvalidArgument, "explicit group or channel required")
	}
	if len(req.ProtoReflect().GetUnknown()) != 0 || len(req.Chat.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "unknown Chat ACL fields")
	}
	for _, raw := range []string{req.SpaceId, req.ProfileId, req.Chat.Id, p.RequestID} {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return status.Error(codes.InvalidArgument, "canonical Chat ACL identities and operation required")
		}
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return status.Error(codes.Unauthenticated, "Chat ACL body binding mismatch")
	}
	return nil
}
