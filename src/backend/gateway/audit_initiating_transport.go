package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func (t *transcoder) auditInitiatingContext(r *http.Request, request proto.Message, method, requestID string) (context.Context, error) {
	if r == nil {
		return nil, status.Error(codes.Unauthenticated, "verified actor required")
	}
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) || (claims.AccountType != "regular" && claims.AccountType != "guest" && claims.AccountType != "sdk-account") {
		return nil, status.Error(codes.Unauthenticated, "verified explicit actor required")
	}
	audience := auditInitiatingAudience(method)
	if audience == "" {
		return nil, status.Error(codes.PermissionDenied, "initiating method is not allowed")
	}
	if t == nil || t.lifecycleIssuer == nil || !t.auditInitiatingClientAvailable(audience) {
		return nil, status.Error(codes.Unavailable, "protected initiating transport unavailable")
	}
	if requestID == "" || strings.TrimSpace(requestID) != requestID {
		return nil, status.Error(codes.InvalidArgument, "request ID required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid initiating request")
	}
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: claims.AccountType, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "initiating credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func (t *transcoder) auditInitiatingClientAvailable(audience string) bool {
	switch audience {
	case "role":
		return t.clients.roleAudit != nil
	case "chat":
		return t.clients.chatAudit != nil
	case "bot":
		return t.clients.botAuditAdmin != nil
	default:
		return false
	}
}

func auditInitiatingAudience(method string) string {
	switch method {
	case rolev1.RoleService_CreateRole_FullMethodName, rolev1.RoleService_UpdateRole_FullMethodName, rolev1.RoleService_DeleteRole_FullMethodName, rolev1.RoleService_ReorderRoles_FullMethodName,
		rolev1.RoleService_AssignRole_FullMethodName, rolev1.RoleService_RevokeRole_FullMethodName, rolev1.RoleService_SetChatOverride_FullMethodName, rolev1.RoleService_RemoveChatOverride_FullMethodName,
		rolev1.RoleService_SetVoiceRoomOverride_FullMethodName, rolev1.RoleService_RemoveVoiceRoomOverride_FullMethodName, rolev1.RoleService_SetDefaultJoinRole_FullMethodName:
		return "role"
	case chatv1.ChatService_CreateChat_FullMethodName, chatv1.ChatService_UpdateChat_FullMethodName, chatv1.ChatService_DeleteChat_FullMethodName:
		return "chat"
	case botv1.BotService_InstallBotInSpace_FullMethodName, botv1.BotService_UninstallBotFromSpace_FullMethodName:
		return "bot"
	default:
		return ""
	}
}
