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
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/reportprincipal"
	"voice/backend/pkg/principal"
)

type reportPolicyStore struct {
	calls         int
	allowed       bool
	err           error
	chat, profile uuid.UUID
	created       time.Time
}

func (s *reportPolicyStore) ManagedChatMessageEntitled(_ context.Context, chat, profile uuid.UUID, at time.Time) (bool, error) {
	s.calls++
	s.chat = chat
	s.profile = profile
	s.created = at
	return s.allowed, s.err
}

func TestReportPolicySignedMessagingBodyReplayAndNativeRestriction(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	seen := map[string]bool{}
	verifier := reportprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(_ context.Context, _, jti string, _ time.Time) error {
		if seen[jti] {
			return status.Error(codes.Unauthenticated, "replay")
		}
		seen[jti] = true
		return nil
	}}
	request := &chatv1.CheckMessageReadEntitlementRequest{ChatId: uuid.NewString(), ProfileId: uuid.NewString(), MessageCreatedAt: timestamppb.New(time.Now().UTC())}
	store := &reportPolicyStore{allowed: true}
	service := &ReportPolicyGRPC{Store: store}
	info := &grpc.UnaryServerInfo{FullMethod: reportprincipal.Method}
	invoke := func(ctx context.Context, r *chatv1.CheckMessageReadEntitlementRequest) (any, error) {
		return verifier.Unary()(ctx, r, info, func(ctx context.Context, v any) (any, error) {
			return service.CheckMessageReadEntitlement(ctx, v.(*chatv1.CheckMessageReadEntitlementRequest))
		})
	}
	sign := func(r *chatv1.CheckMessageReadEntitlementRequest) context.Context {
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		id := uuid.NewString()
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: reportprincipal.Method, RequestID: id, RequestHash: hash})
		require.NoError(t, err)
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	}
	ctx := sign(request)
	response, err := invoke(ctx, request)
	require.NoError(t, err)
	require.True(t, response.(*chatv1.CheckMessageReadEntitlementResponse).Entitled)
	require.Equal(t, 1, store.calls)
	require.Equal(t, request.ChatId, store.chat.String())
	require.Equal(t, request.ProfileId, store.profile.String())
	require.True(t, request.MessageCreatedAt.AsTime().Equal(store.created))
	_, err = invoke(ctx, request)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, store.calls)
	changed := proto.Clone(request).(*chatv1.CheckMessageReadEntitlementRequest)
	changed.ProfileId = uuid.NewString()
	_, err = invoke(sign(request), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, 1, store.calls)
	store.allowed = false
	response, err = invoke(sign(request), request)
	require.NoError(t, err)
	require.False(t, response.(*chatv1.CheckMessageReadEntitlementResponse).Entitled)
	store.err = status.Error(codes.NotFound, "native private detail")
	response, err = invoke(sign(request), request)
	require.Nil(t, response)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NotContains(t, err.Error(), "private detail")
	unknown := proto.Clone(request).(*chatv1.CheckMessageReadEntitlementRequest)
	unknown.MessageCreatedAt.ProtoReflect().SetUnknown([]byte{0x78, 1})
	_, err = invoke(sign(unknown), unknown)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestReportPolicyRawAndGenericVerifiedContextsNeverReachStore(t *testing.T) {
	store := &reportPolicyStore{allowed: true}
	service := &ReportPolicyGRPC{Store: store}
	r := &chatv1.CheckMessageReadEntitlementRequest{ChatId: uuid.NewString(), ProfileId: uuid.NewString(), MessageCreatedAt: timestamppb.Now()}
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	for _, ctx := range []context.Context{context.Background(), metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "messaging")), principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "chat", RPC: reportprincipal.Method, RequestID: uuid.NewString(), RequestHash: hash})} {
		_, err := service.CheckMessageReadEntitlement(ctx, r)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	require.Zero(t, store.calls)
}
