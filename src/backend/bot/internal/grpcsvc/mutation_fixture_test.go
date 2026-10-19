package grpcsvc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/authctx"
	"voice/backend/pkg/principal"
)

// Controlled live Chat facts keep the original whitelist/permission tests
// focused on their own semantics. Real Chat provenance is tested separately.
type mutationFixtureChatScope struct {
	chatv1.ChatScopeAuthorityServiceClient
}

func (mutationFixtureChatScope) GetBotChatScopeAuthority(_ context.Context, r *chatv1.GetBotChatScopeAuthorityRequest, _ ...grpc.CallOption) (*chatv1.GetBotChatScopeAuthorityResponse, error) {
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: digest[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: r.ChatId, ExpectedSpaceId: r.ExpectedSpaceId, ActualSpaceId: r.ExpectedSpaceId, OperationId: r.OperationId, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY}}, nil
}

func mutationFixtureMethod(method string) bool {
	switch method {
	case botv1.BotService_InstallBotInSpace_FullMethodName, botv1.BotService_UninstallBotFromSpace_FullMethodName, botv1.BotService_SetChatWhitelist_FullMethodName, botv1.BotService_SetBotChatEnabled_FullMethodName:
		return true
	}
	return false
}

// These component fixtures verify actual signed method/body/actor credentials.
// Their floor/replay callbacks are controlled; they do not claim the private
// mTLS/Java Auth proof provided by the separate full public composition.
func mutationFixtureSigner(t *testing.T) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "component", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}
func mutationFixtureSignedContext(t *testing.T, ctx context.Context, request proto.Message, method string, issuer *principal.Issuer) context.Context {
	t.Helper()
	incoming := ctx
	if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get("x-voice-user-id")) == 1 && len(md.Get("x-voice-profile-id")) == 1 {
		incoming = metadata.NewIncomingContext(ctx, md)
	}
	account, aok := authctx.AccountID(incoming)
	profile, pok := authctx.ProfileID(incoming)
	if !aok || !pok {
		return ctx
	} // preserve the original unauthenticated cases
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 9, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}
func mutationFixtureVerifiedContext(ctx context.Context, request proto.Message, method string, key *rsa.PublicKey) (context.Context, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return nil, err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, err
	}
	p, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
	if err != nil {
		return nil, err
	}
	return principal.WithVerified(ctx, p), nil
}
func mutationFixtureTransport(t *testing.T) (grpc.ServerOption, grpc.DialOption) {
	t.Helper()
	issuer, key := mutationFixtureSigner(t)
	server := grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if mutationFixtureMethod(info.FullMethod) {
			// An absent credential still reaches the real handler's negative gate.
			if _, err := principal.IncomingMetadata(ctx); err == nil {
				var verifyErr error
				ctx, verifyErr = mutationFixtureVerifiedContext(ctx, req.(proto.Message), info.FullMethod, key)
				if verifyErr != nil {
					return nil, verifyErr
				}
			}
		}
		return next(ctx, req)
	})
	client := grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, conn *grpc.ClientConn, next grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if mutationFixtureMethod(method) {
			ctx = mutationFixtureSignedContext(t, ctx, req.(proto.Message), method, issuer)
		}
		return next(ctx, method, req, reply, conn, opts...)
	})
	return server, client
}
func verifiedMutationActor(t *testing.T, ctx context.Context, request proto.Message, method string) context.Context {
	t.Helper()
	issuer, key := mutationFixtureSigner(t)
	signed := mutationFixtureSignedContext(t, ctx, request, method, issuer)
	md, ok := metadata.FromOutgoingContext(signed)
	require.True(t, ok)
	verified, err := mutationFixtureVerifiedContext(metadata.NewIncomingContext(ctx, md), request, method, key)
	require.NoError(t, err)
	return verified
}
