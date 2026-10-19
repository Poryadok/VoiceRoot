package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func reportPermissionRequest() *rolev1.CheckPermissionRequest {
	return &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
}
func reportPermissionPrincipal(t *testing.T, req *rolev1.CheckPermissionRequest, issuer string) principal.Principal {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash}
}

func TestReportPermissionRequiresExactCallerScopeBeforeDecision(t *testing.T) {
	for _, permission := range []string{"TEXT_CHAT_VIEW", "TEXT_CHAT_READ_HISTORY", "MODERATION_MANAGE_REPORTS"} {
		req := reportPermissionRequest()
		req.PermissionName = permission
		issuer, ok := ReportPermissionIssuer(permission)
		require.True(t, ok)
		p := reportPermissionPrincipal(t, req, issuer)
		require.NoError(t, RequireReportPermission(principal.WithVerified(context.Background(), p), req))
	}
	require.Error(t, RequireReportPermission(context.Background(), reportPermissionRequest()), "subject fields cannot authenticate a caller")
	for _, tc := range []struct {
		name   string
		mutate func(*rolev1.CheckPermissionRequest, *principal.Principal)
		code   codes.Code
		rehash bool
	}{
		{"wrong issuer", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			p.Issuer = "space"
			p.Subject = "service:space"
		}, codes.Unauthenticated, false},
		{"moderation cannot view", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			p.Issuer = "moderation"
			p.Subject = "service:moderation"
		}, codes.Unauthenticated, false},
		{"foreign audience", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { p.Audience = "space" }, codes.Unauthenticated, false},
		{"foreign method", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			p.RPC = rolev1.RoleService_GetMemberRoles_FullMethodName
		}, codes.Unauthenticated, false},
		{"actor profile claim", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { p.ProfileID = r.ProfileId }, codes.Unauthenticated, false},
		{"bot capability", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { p.BotID = uuid.NewString() }, codes.Unauthenticated, false},
		{"changed viewer", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { r.ProfileId = uuid.NewString() }, codes.Unauthenticated, false},
		{"unknown permission", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { r.PermissionName = "MEMBER_BAN" }, codes.PermissionDenied, true},
		{"no chat", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { r.Chat = nil }, codes.InvalidArgument, true},
		{"no type", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { r.Chat.Type = nil }, codes.InvalidArgument, true},
		{"DM scope", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			r.Chat.Type = chatv1.ChatType_CHAT_TYPE_DM.Enum()
		}, codes.InvalidArgument, true},
		{"voice present empty", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) { empty := ""; r.VoiceRoomId = &empty }, codes.InvalidArgument, true},
		{"outer unknown", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			r.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
		}, codes.InvalidArgument, true},
		{"nested unknown", func(r *rolev1.CheckPermissionRequest, p *principal.Principal) {
			r.Chat.ProtoReflect().SetUnknown([]byte{0xa0, 6, 1})
		}, codes.InvalidArgument, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := reportPermissionRequest()
			p := reportPermissionPrincipal(t, req, "messaging")
			tc.mutate(req, &p)
			if tc.rehash {
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p.RequestHash = hash
			}
			err := RequireReportPermission(principal.WithVerified(context.Background(), p), req)
			require.Equal(t, tc.code, status.Code(err))
		})
	}
}
