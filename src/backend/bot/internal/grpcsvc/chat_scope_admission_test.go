package grpcsvc_test

import (
	"context"
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// Controlled permission/membership dependencies isolate the missing Chat-source
// gate. This is not a public Role/Space interoperability acceptance fixture.
func TestInstallBotInSpaceRequiresChatSourceBeforeDurableIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL admission")
	}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: &fakeRoleClient{}, space: &fakeSpaceClient{}, noScopeAuthority: true})
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	registered, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{Name: "Scope gate", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`})
	require.NoError(t, err)
	space := uuid.New()
	_, err = client.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{BotId: registered.Bot.Id, SpaceId: space.String(), AllowedChats: []*chatv1.ChatRef{{Id: uuid.NewString()}}})
	require.Equal(t, codes.Unavailable, status.Code(err), "missing authoritative Chat source cannot authorize a whitelist")
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_installation_intents WHERE bot_id=$1`, registered.Bot.Id).Scan(&count))
	require.Zero(t, count, "unqualified target cannot create a durable native intent")
}

type firstReadOnlyChatScope struct {
	chatv1.ChatScopeAuthorityServiceClient
	mu    sync.Mutex
	calls int
}

func (s *firstReadOnlyChatScope) GetBotChatScopeAuthority(_ context.Context, r *chatv1.GetBotChatScopeAuthorityRequest, _ ...grpc.CallOption) (*chatv1.GetBotChatScopeAuthorityResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.calls > 1 {
		return nil, status.Error(codes.NotFound, "source removed after original admission")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	return &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: digest[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: r.ChatId, ExpectedSpaceId: r.ExpectedSpaceId, ActualSpaceId: r.ExpectedSpaceId, OperationId: r.OperationId, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY}}, nil
}

func TestExactInstallationRetryUsesOriginalAcceptedChatScope(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL accepted source recovery")
	}
	source := &firstReadOnlyChatScope{}
	client, st, _, cleanup := startBotGRPCWithBotCDeps(t, &botCDeps{role: &fakeRoleClient{}, space: &fakeSpaceClient{}, scope: source})
	defer cleanup()
	ctx := withAccount(context.Background(), uuid.New(), uuid.New())
	registered, err := client.RegisterBot(ctx, &botv1.RegisterBotRequest{Name: "Scope retry", ScopesJson: `["TEXT_CHAT_SEND_MESSAGES"]`})
	require.NoError(t, err)
	space, chat := uuid.New(), uuid.New()
	claim := chatv1.ChatType_CHAT_TYPE_DM
	request := &botv1.InstallBotInSpaceRequest{BotId: registered.Bot.Id, SpaceId: space.String(), AllowedChats: []*chatv1.ChatRef{{Id: chat.String(), Type: &claim}}}
	first, err := client.InstallBotInSpace(ctx, request)
	require.NoError(t, err)
	again, err := client.InstallBotInSpace(ctx, request)
	require.NoError(t, err, "exact operation recovery uses its original accepted source, even after deletion")
	require.Equal(t, first.InstallationId, again.InstallationId)
	source.mu.Lock()
	calls := source.calls
	source.mu.Unlock()
	require.Equal(t, 1, calls)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_chat_scope_observations WHERE bot_id=$1`, registered.Bot.Id).Scan(&count))
	require.Equal(t, 1, count)
}
