package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestRequireChatCurrentACLPrincipal(t *testing.T) {
	r := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "TEXT_CHAT_VIEW", Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	base := principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, ExpiresAt: time.Now().Add(30 * time.Second)}
	require.Error(t, RequireChatCurrentACL(context.Background(), r))
	require.NoError(t, RequireChatCurrentACL(principal.WithVerified(context.Background(), base), r))
	for name, change := range map[string]func(*principal.Principal){
		"issuer": func(p *principal.Principal) { p.Issuer = "space" }, "kind": func(p *principal.Principal) { p.Kind = "delegated_user" }, "subject": func(p *principal.Principal) { p.Subject = "service:space" }, "aud": func(p *principal.Principal) { p.Audience = "chat" },
		"account": func(p *principal.Principal) { p.AccountID = uuid.NewString() }, "profile": func(p *principal.Principal) { p.ProfileID = uuid.NewString() }, "epoch": func(p *principal.Principal) { p.SessionEpoch = 1 }, "account_type": func(p *principal.Principal) { p.AccountType = "regular" },
		"bot": func(p *principal.Principal) { p.BotID = uuid.NewString() }, "actor": func(p *principal.Principal) { p.ActorProfileID = uuid.NewString() }, "space": func(p *principal.Principal) { p.SpaceID = r.SpaceId }, "installation": func(p *principal.Principal) { p.InstallationID = uuid.NewString() }, "scope": func(p *principal.Principal) { p.BotScope = "TEXT_CHAT_VIEW" },
		"hash": func(p *principal.Principal) { p.RequestHash = "wrong" }, "method": func(p *principal.Principal) { p.RPC = "other" }, "expired": func(p *principal.Principal) { p.ExpiresAt = time.Now().Add(-time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			p := base
			change(&p)
			require.Error(t, RequireChatCurrentACL(principal.WithVerified(context.Background(), p), r))
		})
	}
}

func TestChatCurrentACLOrdinaryDrainPreservesUnrelatedReads(t *testing.T) {
	req := &rolev1.CheckPermissionRequest{PermissionName: "TEXT_CHAT_VIEW"}
	called := false
	call := func(context.Context, any) (any, error) { called = true; return nil, nil }
	info := &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}
	_, err := OwnershipUnaryInterceptor(nil)(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer unused")), req, info, call)
	require.Error(t, err)
	require.False(t, called)
	_, err = OwnershipUnaryInterceptor(nil)(context.Background(), req, info, call)
	require.NoError(t, err)
	require.True(t, called, "BE131c9 does not globally migrate unrelated unsigned legacy reads")
}
