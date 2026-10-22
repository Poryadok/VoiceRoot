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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/chat/internal/chatcreation"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/principal"
)

type signedCompositeClient struct {
	spacev1.SpaceServiceClient
	t    *testing.T
	key  *rsa.PrivateKey
	seen map[string]bool
}

type publicCompositeClient struct {
	spacev1.SpaceServiceClient
	get func(context.Context, *spacev1.GetSpaceChatCreationOutcomeRequest) (*spacev1.GetSpaceChatCreationOutcomeResponse, error)
}

func (s publicCompositeClient) GetSpaceChatCreationOutcome(ctx context.Context, req *spacev1.GetSpaceChatCreationOutcomeRequest, _ ...grpc.CallOption) (*spacev1.GetSpaceChatCreationOutcomeResponse, error) {
	return s.get(ctx, req)
}

func TestCompositePublicUsesFreshCurrentActorAndCannotCacheAllow(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	user := &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 7, AccountType: "regular"}
	b := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: uuid.NewString(), SpaceId: uuid.NewString(), ChatId: uuid.NewString(), NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: user}}
	actor := chatcreation.Initiator{ProfileID: uuid.MustParse(b.CreatorProfileId), User: proto.Clone(user).(*spacev1.SpaceChatCreationUser), AccountType: "guest"}
	actor.User.SessionEpoch = 8
	actor.User.AccountType = "guest"
	seen := map[string]bool{}
	calls := 0
	remote := &SignedSpaceCompositeRemote{Issuer: issuer}
	remote.Client = publicCompositeClient{get: func(ctx context.Context, req *spacev1.GetSpaceChatCreationOutcomeRequest) (*spacev1.GetSpaceChatCreationOutcomeResponse, error) {
		calls++
		require.Equal(t, b.SpaceId, req.SpaceId)
		require.Equal(t, b.OperationId, req.OperationId)
		require.True(t, proto.Equal(actor.User, req.PublicActor.GetUser()))
		require.Equal(t, b.CreatorProfileId, req.PublicActor.ProfileId)
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		_, err = principal.VerifyService(ctx, md.Get("authorization")[0][7:], principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, ExpectedRequestID: b.OperationId, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			require.False(t, seen[jti])
			seen[jti] = true
			return nil
		}})
		require.NoError(t, err)
		if calls > 1 {
			return nil, status.Error(codes.NotFound, "current membership removed")
		}
		return &spacev1.GetSpaceChatCreationOutcomeResponse{Outcome: &spacev1.SpaceChatCreationOutcome{Binding: b, State: spacev1.SpaceChatCreationOutcomeState_SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED}}, nil
	}}
	// Current Chat guest authority is independent of the saved create snapshot.
	remote.Chats = publicCompositeChatReader{row: &store.ChatRow{ID: uuid.MustParse(b.ChatId), SpaceID: uuidPointer(uuid.MustParse(b.SpaceId)), AllowGuests: true}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer original", "x-voice-profile-id", uuid.NewString()))
	op := &store.SpaceCompositeOperation{State: "COMPLETE", Binding: b}
	require.NoError(t, remote.Check(ctx, actor, op))
	require.Equal(t, codes.NotFound, status.Code(remote.Check(ctx, actor, op)))
	require.Equal(t, 2, calls)
	require.Len(t, seen, 2)
	require.Equal(t, int64(7), b.GetUser().SessionEpoch, "immutable intent is never rewritten")
	require.Equal(t, "regular", b.GetUser().AccountType)
}

type publicCompositeChatReader struct {
	row *store.ChatRow
	err error
}

func (r publicCompositeChatReader) FindChatByID(context.Context, uuid.UUID) (*store.ChatRow, error) {
	return r.row, r.err
}
func uuidPointer(id uuid.UUID) *uuid.UUID { return &id }

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
