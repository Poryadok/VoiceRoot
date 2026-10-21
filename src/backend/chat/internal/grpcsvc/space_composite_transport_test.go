package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type signedCompositeClient struct {
	spacev1.SpaceServiceClient
	t    *testing.T
	key  *rsa.PrivateKey
	seen map[string]bool
}

func (s *signedCompositeClient) CommitSpaceChatCreation(ctx context.Context, req *spacev1.CommitSpaceChatCreationRequest, _ ...grpc.CallOption) (*spacev1.CommitSpaceChatCreationResponse, error) {
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(s.t, ok)
	require.Len(s.t, md, 2, "no delegated token/raw identity is forwarded")
	require.Equal(s.t, []string{req.Binding.OperationId}, md.Get("x-request-id"))
	hash, err := principal.RequestHash(req)
	require.NoError(s.t, err)
	p, err := principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_CommitSpaceChatCreation_FullMethodName, ExpectedRequestID: req.Binding.OperationId, ExpectedRequestHash: hash,
		KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &s.key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			require.False(s.t, s.seen[jti])
			s.seen[jti] = true
			return nil
		}})
	require.NoError(s.t, err)
	require.Equal(s.t, "service:chat", p.Subject)
	return &spacev1.CommitSpaceChatCreationResponse{Outcome: &spacev1.SpaceChatCreationOutcome{Binding: req.Binding}}, nil
}

func TestCompositeRemoteUsesFreshOwnPrincipalAndExactBody(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client := &signedCompositeClient{t: t, key: key, seen: map[string]bool{}}
	remote := &SignedSpaceCompositeRemote{Client: client, Issuer: issuer}
	binding := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), ChatId: uuid.NewString(), NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 1, AccountType: "regular"}}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer delegated-original", "x-voice-profile-id", uuid.NewString()))
	for range 2 {
		out, err := remote.Commit(ctx, binding)
		require.NoError(t, err)
		require.True(t, proto.Equal(binding, out.Binding))
	}
	require.Len(t, client.seen, 2)
}
