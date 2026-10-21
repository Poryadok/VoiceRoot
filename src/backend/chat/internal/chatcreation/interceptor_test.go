package chatcreation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

func TestCreateVerifierUnitRequestReplayEpochAndPeerBoundaries(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	gateway, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: proto.String(uuid.NewString()), Name: proto.String("created"), RequestId: uuid.NewString()}
	account, profile := uuid.NewString(), uuid.NewString()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	epochErr := error(nil)
	seen := map[string]bool{}
	v := CreateVerifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
		Replay: func(_ context.Context, _ string, id string, _ time.Time) error {
			if seen[id] {
				return errors.New("replay")
			}
			seen[id] = true
			return nil
		},
		Epoch: func(_ context.Context, a string, e int64) error {
			require.Equal(t, account, a)
			require.EqualValues(t, 7, e)
			return epochErr
		}}
	issue := func(accountType string) string {
		jwt, e := gateway.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: req.RequestId, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, AccountType: accountType, ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		return jwt
	}
	token := func() string { return issue("regular") }
	incoming := func(jwt string) context.Context {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+jwt, "x-request-id", req.RequestId))
		// Synthetic verified-peer unit fixture; actual TLS negotiation is a separate
		// runtime proof and is never inferred from this context construction.
		return peer.NewContext(ctx, &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}}})
	}
	writes := 0
	handler := func(ctx context.Context, _ any) (any, error) {
		p, ok := principal.FromContext(ctx)
		require.True(t, ok)
		require.Equal(t, profile, p.ProfileID)
		writes++
		return nil, nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: chatv1.ChatService_CreateChat_FullMethodName}
	first := token()
	_, err = v.Unary()(incoming(first), req, info, handler)
	require.NoError(t, err)
	require.Equal(t, 1, writes)
	_, err = v.Unary()(incoming(first), req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	bad := proto.Clone(req).(*chatv1.CreateChatRequest)
	bad.Name = proto.String("changed")
	_, err = v.Unary()(incoming(token()), bad, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	spoof := incoming(token())
	raw, _ := metadata.FromIncomingContext(spoof)
	raw = raw.Copy()
	raw.Set("x-voice-profile-id", profile)
	_, err = v.Unary()(metadata.NewIncomingContext(spoof, raw), req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	unknown := proto.Clone(req).(*chatv1.CreateChatRequest)
	unknown.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
	_, err = v.Unary()(incoming(token()), unknown, info, handler)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	for _, kind := range []string{"", "sdk-account"} {
		_, err = v.Unary()(incoming(issue(kind)), req, info, handler)
		require.Equal(t, codes.Unauthenticated, status.Code(err), "explicit eligible account type required before preparation")
	}
	epochErr = errors.New("epoch revoked")
	_, err = v.Unary()(incoming(token()), req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	epochErr = ErrDependencyUnavailable
	_, err = v.Unary()(incoming(token()), req, info, handler)
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = v.Unary()(metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token(), "x-request-id", req.RequestId)), req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, writes, "all denied requests stop before the receiver")
}

func TestCreatorBotBindsExactSpaceWithoutHumanAuthority(t *testing.T) {
	req := &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_GROUP, SpaceId: proto.String(uuid.NewString()), Name: proto.String("bot-created"), RequestId: uuid.NewString()}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	bot := uuid.NewString()
	p := principal.Principal{Kind: "bot_actor", Issuer: "bot", Subject: "bot:" + bot, Audience: "chat", RPC: chatv1.ChatService_CreateChat_FullMethodName, RequestID: req.RequestId, RequestHash: hash,
		ActorProfileID: uuid.NewString(), BotID: bot, SpaceID: req.GetSpaceId(), InstallationID: uuid.NewString(), BotScope: "TEXT_CHAT_CREATE_IN_SPACE", ExpiresAt: time.Now().Add(30 * time.Second)}
	actor, err := Creator(principal.WithVerified(context.Background(), p), req)
	require.NoError(t, err)
	require.NotNil(t, actor.Bot)
	require.Nil(t, actor.User)
	require.Equal(t, p.ActorProfileID, actor.ProfileID.String())
	changed := proto.Clone(req).(*chatv1.CreateChatRequest)
	changed.SpaceId = proto.String(uuid.NewString())
	p.RequestHash, err = principal.RequestHash(changed)
	require.NoError(t, err)
	_, err = Creator(principal.WithVerified(context.Background(), p), changed)
	require.ErrorIs(t, err, ErrInitiator)
	p.RequestHash = hash
	p.AccountID = uuid.NewString()
	_, err = Creator(principal.WithVerified(context.Background(), p), req)
	require.ErrorIs(t, err, ErrInitiator)
}

func TestOrdinaryCreateMiddlewareDeniesEvenVerifiedContext(t *testing.T) {
	called := false
	handler := func(context.Context, any) (any, error) { called = true; return nil, nil }
	_, err := OrdinaryCreateUnary()(principal.WithVerified(context.Background(), principal.Principal{Kind: "delegated_user"}), &chatv1.CreateChatRequest{}, &grpc.UnaryServerInfo{FullMethod: chatv1.ChatService_CreateChat_FullMethodName}, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called)
	_, err = OrdinaryCreateUnary()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: chatv1.ChatService_GetChat_FullMethodName}, handler)
	require.NoError(t, err)
	require.True(t, called)
}
