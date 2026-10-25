package grpcsvc

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestBotInstallationRemoval_NoRawAuthority(t *testing.T) {
	_, err := (&RoleGRPC{}).ApplyBotInstallationRemoval(context.Background(), &rolev1.ApplyBotInstallationRemovalRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestBotInstallationRemoval_HandlerExactAuthorityBeforeStore(t *testing.T) {
	request := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: uuid.NewString(), DecisionSha256: make([]byte, 32), Purpose: "global_delete", BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), FrozenScopeSha256: make([]byte, 32), Evidence: &rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist{OrphanWhitelist: &rolev1.BotRemovalOrphanWhitelistEvidence{SnapshotId: uuid.NewString(), SnapshotSha256: make([]byte, 32)}}}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "role", RPC: rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, RequestID: request.OperationId, RequestHash: hash}
	cases := []struct {
		name   string
		change func(*principal.Principal)
	}{
		{"issuer", func(p *principal.Principal) { p.Issuer = "space" }},
		{"bot actor", func(p *principal.Principal) { p.Kind = "bot_actor" }},
		{"delegated", func(p *principal.Principal) { p.Kind = "delegated_user" }},
		{"audience", func(p *principal.Principal) { p.Audience = "messaging" }},
		{"generic method", func(p *principal.Principal) { p.RPC = rolev1.RoleService_RevokeRole_FullMethodName }},
		{"requestid", func(p *principal.Principal) { p.RequestID = uuid.NewString() }},
		{"hash", func(p *principal.Principal) { p.RequestHash = "sha256:" + string(bytes.Repeat([]byte{'0'}, 64)) }},
		{"mixed user", func(p *principal.Principal) { p.AccountID = uuid.NewString() }},
		{"mixed epoch", func(p *principal.Principal) { p.SessionEpoch = 1 }},
		{"mixed capability", func(p *principal.Principal) { p.BotScope = "SPACE_MANAGE_ROLES" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := valid
			tt.change(&p)
			_, err := (&RoleGRPC{}).ApplyBotInstallationRemoval(principal.WithVerified(context.Background(), p), request)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
		})
	}
	// Exact verified authority reaches persistence admission, never an unsigned
	// fallback; this test intentionally has no database or fake positive store.
	_, err = (&RoleGRPC{}).ApplyBotInstallationRemoval(principal.WithVerified(context.Background(), valid), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	malformed := proto.Clone(request).(*rolev1.ApplyBotInstallationRemovalRequest)
	malformed.ProtoReflect().SetUnknown([]byte{0x78, 1})
	valid.RequestHash, err = principal.RequestHash(malformed)
	require.NoError(t, err)
	_, err = (&RoleGRPC{}).ApplyBotInstallationRemoval(principal.WithVerified(context.Background(), valid), malformed)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
