package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// ReportPermissionIssuer classifies one query; it does not authenticate callers.
func ReportPermissionIssuer(permission string) (string, bool) {
	switch permission {
	case "TEXT_CHAT_VIEW", "TEXT_CHAT_READ_HISTORY":
		return "messaging", true
	case "MODERATION_MANAGE_REPORTS":
		return "moderation", true
	}
	return "", false
}
func RequireReportPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || (p.Issuer != "messaging" && p.Issuer != "moderation") || p.Subject != "service:"+p.Issuer || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.InstallationID != "" || p.BotScope != "" || p.SpaceID != "" {
		return status.Error(codes.Unauthenticated, "verified report decision workload required")
	}
	canonical := func(raw string) bool {
		id, err := uuid.Parse(raw)
		return err == nil && id != uuid.Nil && id.String() == raw
	}
	if !canonical(p.RequestID) {
		return status.Error(codes.Unauthenticated, "invalid report decision request ID")
	}
	if req == nil {
		return status.Error(codes.InvalidArgument, "invalid report decision")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return status.Error(codes.Unauthenticated, "report decision body mismatch")
	}
	issuer, known := ReportPermissionIssuer(req.PermissionName)
	if !known {
		return status.Error(codes.PermissionDenied, "report permission required")
	}
	if issuer != p.Issuer {
		return status.Error(codes.Unauthenticated, "report decision workload mismatch")
	}
	if len(req.ProtoReflect().GetUnknown()) != 0 || !canonical(req.SpaceId) || !canonical(req.ProfileId) || req.Chat == nil || req.VoiceRoomId != nil || len(req.Chat.ProtoReflect().GetUnknown()) != 0 || !canonical(req.Chat.Id) || req.Chat.Type == nil || (req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_GROUP && req.Chat.GetType() != chatv1.ChatType_CHAT_TYPE_CHANNEL) {
		return status.Error(codes.InvalidArgument, "exact report Chat scope required")
	}
	return nil
}
