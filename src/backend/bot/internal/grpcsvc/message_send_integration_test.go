package grpcsvc

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
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"

	"voice/backend/bot/internal/testdb"
)

func sendNativeAdmin(t *testing.T, req proto.Message, method string) installprincipal.Administrator {
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
	token, err := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
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
func sendMessageStore(t *testing.T) (*store.BotStore, uuid.UUID, store.MembershipRecord, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := testdb.StartPostgres(t, ctx, "botsend", "")
	files, err := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	require.NoError(t, err)
	for _, name := range files {
		raw, e := os.ReadFile(name)
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(raw))
		require.NoError(t, e)
	}
	_, err = pool.Exec(ctx, "CREATE TABLE schema_migrations(version bigint,dirty boolean); INSERT INTO schema_migrations VALUES(9,false)")
	require.NoError(t, err)
	st := &store.BotStore{Pool: pool}
	var source uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT source_generation FROM bot_space_membership_source").Scan(&source))
	bot, _, err := st.CreateBot(ctx, uuid.New(), "send-only", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	chat := uuid.New()
	typ := chatv1.ChatType_CHAT_TYPE_GROUP
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString(), AllowedChats: []*chatv1.ChatRef{{Id: chat.String(), Type: &typ}}}
	record, err := st.PrepareInstall(ctx, source, sendNativeAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName), req)
	require.NoError(t, err)
	record, err = st.ClaimMembershipAttempt(ctx, source, record, time.Second)
	require.NoError(t, err)
	item, err := st.PrepareMembershipChat(ctx, source, record, chat)
	require.NoError(t, err)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(item.Request)
	require.NoError(t, err)
	hash := sha256.Sum256(raw)
	// Controlled Chat owner response for this Bot-store component fixture.
	item.Response = &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: hash[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: record.SpaceID.String(), ActualSpaceId: record.SpaceID.String(), OperationId: item.Request.OperationId, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, Exists: true, Live: true, ChatType: typ, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}}
	require.NoError(t, st.SaveMembershipChat(ctx, source, record, item))
	record, err = st.RecordAddAcknowledged(ctx, source, record)
	require.NoError(t, err)
	_, err = st.CompleteInstallation(ctx, source, record)
	require.NoError(t, err)
	return st, source, record, chat
}

type protectedSendFixture struct {
	messagingv1.MessagingServiceClient
	send func(context.Context, *messagingv1.SendBotMessageRequest) (*messagingv1.SendBotMessageResponse, error)
}

func (f protectedSendFixture) SendBotMessage(ctx context.Context, req *messagingv1.SendBotMessageRequest, _ ...grpc.CallOption) (*messagingv1.SendBotMessageResponse, error) {
	return f.send(ctx, req)
}

func TestSpaceSlashAndDeferredSendUseProtectedInstallation(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL")
	}
	st, _, record, chat := sendMessageStore(t)
	bot, err := st.GetBotByID(t.Context(), record.BotID)
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "send-test", PrivateKey: key})
	require.NoError(t, err)
	service := &BotGRPC{Store: st, PrincipalIssuer: signer}
	thread := uuid.NewString()
	calls := 0
	service.ProtectedMessaging = protectedSendFixture{send: func(ctx context.Context, request *messagingv1.SendBotMessageRequest) (*messagingv1.SendBotMessageResponse, error) {
		calls++
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-voice-profile-id"))
		require.Empty(t, md.Get("x-voice-user-id"))
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		_, e = principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "messaging", ExpectedRPC: messagingv1.MessagingService_SendBotMessage_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, e)
		allowed, e := st.CheckCurrentMessageSendAuthority(ctx, request.Binding)
		require.NoError(t, e)
		require.True(t, allowed)
		require.Equal(t, chat.String(), request.Message.Chat.Id)
		require.Equal(t, thread, request.Message.GetThreadParentId())
		blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		require.Error(t, st.SetChatEnabled(blocked, record.BotID, chat, record.SpaceID, record.AdministratorProfile, false), "disable cannot overtake the actual Messaging RPC")
		if calls == 2 {
			require.NotEmpty(t, request.Message.GetClientMessageId())
		}
		return &messagingv1.SendBotMessageResponse{Message: &messagingv1.Message{Id: uuid.NewString(), Chat: request.Message.Chat, SenderProfileId: record.ActorProfileID.String()}}, nil
	}}
	ctx := metadata.NewOutgoingContext(t.Context(), metadata.Pairs("x-voice-profile-id", uuid.NewString(), "x-voice-user-id", uuid.NewString()))
	ref := &chatv1.ChatRef{Id: chat.String(), Type: chatTypePtr(chatv1.ChatType_CHAT_TYPE_GROUP)}
	_, err = service.postMessage(ctx, bot, ref, "slash", thread)
	require.NoError(t, err)
	_, err = service.postInteractionMessage(ctx, bot, ref, "deferred-token", "later", thread)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.NoError(t, st.SetChatEnabled(t.Context(), record.BotID, chat, record.SpaceID, record.AdministratorProfile, false))
	_, err = service.postMessage(ctx, bot, ref, "revoked", thread)
	require.Error(t, err)
	require.Equal(t, 2, calls)
}
