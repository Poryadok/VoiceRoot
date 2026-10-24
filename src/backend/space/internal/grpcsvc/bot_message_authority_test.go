package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/roleoverrideprincipal"
)

type botAuthorityVerifier struct {
	key              *rsa.PublicKey
	observedDeadline *time.Time
}

func (v botAuthorityVerifier) Verify(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	*v.observedDeadline, _ = ctx.Deadline()
	return principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: "messaging", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return v.key, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
}

func TestBotMessageAuthorityOnlyExactSignedMessagingSelector(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	chat := uuid.NewString()
	b := &commonv1.BotMessageSendBinding{ProtocolVersion: 1, BotId: uuid.NewString(), ActorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), InstallationId: uuid.NewString(), InstallOperationId: uuid.NewString(), InstallationGeneration: 1, ChatId: chat}
	req := &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: b.SpaceId, ProfileId: b.ActorProfileId, ChatId: &chat, BotMessageSendBinding: b}
	method := spacev1.SpaceService_GetMemberAdmissionAuthority_FullMethodName
	signed := func(body proto.Message) context.Context {
		hash, e := principal.RequestHash(body)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := signer.IssueService(principal.ServiceInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash})
		require.NoError(t, e)
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	reached := false
	handler := func(ctx context.Context, r any) (any, error) {
		reached = true
		return nil, roleoverrideprincipal.RequireMemberAdmissionAuthority(ctx, r.(proto.Message))
	}
	var observedDeadline time.Time
	gate := roleoverrideprincipal.StrictUnaryInterceptor("space", botAuthorityVerifier{&key.PublicKey, &observedDeadline})
	info := &grpc.UnaryServerInfo{FullMethod: method}
	_, err = gate(signed(req), req, info, handler)
	require.NoError(t, err)
	require.True(t, reached)
	require.False(t, observedDeadline.IsZero(), "budget starts before credential verification")
	require.Positive(t, time.Until(observedDeadline))
	require.LessOrEqual(t, time.Until(observedDeadline), 5*time.Second)
	raw := &SpaceGRPC{}
	_, err = raw.GetMemberAdmissionAuthority(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	unscoped := proto.Clone(req).(*spacev1.GetMemberAdmissionAuthorityRequest)
	unscoped.BotMessageSendBinding = nil
	reached = false
	_, err = gate(signed(unscoped), unscoped, info, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, reached)
	changed := proto.Clone(req).(*spacev1.GetMemberAdmissionAuthorityRequest)
	changed.BotMessageSendBinding.InstallationGeneration++
	_, err = gate(signed(req), changed, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = roleoverrideprincipal.OrdinaryUnaryInterceptor("space")(signed(req), req, info, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = gate(signed(req), req, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ValidateRoleOverrideTarget_FullMethodName}, handler)
	require.Error(t, err)
}
