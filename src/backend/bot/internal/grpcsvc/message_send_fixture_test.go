package grpcsvc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"strings"
	messagingv1 "voice.app/voice/messaging/v1"
	grpcsvc "voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"

	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func fixtureSendAdmin(t *testing.T, req proto.Message, method string, profile uuid.UUID) installprincipal.Administrator {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	token, err := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profile.String(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	verifier := &installprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	var admin installprincipal.Administrator
	_, err = verifier.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		admin, err = installprincipal.Require(ctx, method, req)
		return nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return admin
}

// A controlled Messaging receiver for existing Bot behaviour tests. It verifies
// the exact service credential and current durable Bot tuple before delegating to
// the test's original content/error/idempotency probe. TLS is covered separately.
type fixtureProtectedMessaging struct {
	messagingv1.MessagingServiceServer
	t     *testing.T
	key   *rsa.PublicKey
	store *store.BotStore
}

func (f fixtureProtectedMessaging) SendBotMessage(ctx context.Context, req *messagingv1.SendBotMessageRequest) (*messagingv1.SendBotMessageResponse, error) {
	transport, err := principal.IncomingMetadata(ctx)
	require.NoError(f.t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(f.t, err)
	_, err = principal.VerifyService(ctx, strings.TrimPrefix(transport.BearerToken, "Bearer "), principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "messaging", ExpectedRPC: messagingv1.MessagingService_SendBotMessage_FullMethodName, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return f.key, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
	require.NoError(f.t, err)
	allowed, err := f.store.CheckCurrentMessageSendAuthority(ctx, req.Binding)
	require.NoError(f.t, err)
	require.True(f.t, allowed)
	require.Equal(f.t, req.Binding.ChatId, req.Message.GetChat().GetId())
	out, err := f.SendMessage(ctx, req.Message)
	if err != nil {
		return nil, err
	}
	return &messagingv1.SendBotMessageResponse{Message: out.Message}, nil
}

func protectedMessagingFixture(t *testing.T, svc *grpcsvc.BotGRPC, inner messagingv1.MessagingServiceServer) messagingv1.MessagingServiceServer {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "bot_db", "000008_space_membership_intents.up.sql"))
	require.NoError(t, err)
	_, err = svc.Store.Pool.Exec(t.Context(), string(raw))
	require.NoError(t, err)
	_, err = svc.Store.Pool.Exec(t.Context(), "CREATE TABLE schema_migrations(version bigint,dirty boolean); INSERT INTO schema_migrations VALUES(9,false)")
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	svc.PrincipalIssuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	reader, err := store.NewMessageAuthorityStore(t.Context(), svc.Store.Pool)
	require.NoError(t, err)
	t.Cleanup(reader.Pool.Close)
	return fixtureProtectedMessaging{MessagingServiceServer: inner, t: t, key: &key.PublicKey, store: reader}
}
func installSendFixture(t *testing.T, st *store.BotStore, ctx context.Context, botID, spaceID, profile uuid.UUID, chats []uuid.UUID) (uuid.UUID, error) {
	t.Helper()
	var native bool
	require.NoError(t, st.Pool.QueryRow(ctx, "SELECT to_regclass('bot_space_membership_source') IS NOT NULL").Scan(&native))
	if !native {
		return st.InstallInSpace(ctx, botID, spaceID, profile, chats)
	}
	var source uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, "SELECT source_generation FROM bot_space_membership_source").Scan(&source))
	refs := make([]*chatv1.ChatRef, 0, len(chats))
	typ := chatv1.ChatType_CHAT_TYPE_CHANNEL
	for _, chat := range chats {
		refs = append(refs, &chatv1.ChatRef{Id: chat.String(), Type: &typ})
	}
	req := &botv1.InstallBotInSpaceRequest{BotId: botID.String(), SpaceId: spaceID.String(), AllowedChats: refs, AcknowledgePrivilegedScopes: true}
	record, err := st.PrepareInstall(ctx, source, fixtureSendAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName, profile), req)
	require.NoError(t, err)
	record, err = st.ClaimMembershipAttempt(ctx, source, record, time.Second)
	require.NoError(t, err)
	for _, chat := range chats {
		item, err := st.PrepareMembershipChat(ctx, source, record, chat)
		require.NoError(t, err)
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(item.Request)
		require.NoError(t, err)
		hash := sha256.Sum256(raw)
		// Controlled Chat owner response for this Bot-store component fixture.
		item.Response = &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: hash[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: record.SpaceID.String(), ActualSpaceId: record.SpaceID.String(), OperationId: item.Request.OperationId, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, Exists: true, Live: true, ChatType: typ, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}}
		require.NoError(t, st.SaveMembershipChat(ctx, source, record, item))
	}
	record, err = st.RecordAddAcknowledged(ctx, source, record)
	require.NoError(t, err)
	_, err = st.CompleteInstallation(ctx, source, record)
	require.NoError(t, err)
	return record.InstallationID, nil
}
