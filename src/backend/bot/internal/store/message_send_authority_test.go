package store

import (
	"context"
	"crypto/sha256"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/testdb"
)

func messageSendStore(t *testing.T) (*BotStore, uuid.UUID, MembershipRecord, uuid.UUID) {
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
	st := &BotStore{Pool: pool}
	var source uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT source_generation FROM bot_space_membership_source").Scan(&source))
	bot, _, err := st.CreateBot(ctx, uuid.New(), "send-only", "", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	chat := uuid.New()
	typ := chatv1.ChatType_CHAT_TYPE_GROUP
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString(), AllowedChats: []*chatv1.ChatRef{{Id: chat.String(), Type: &typ}}}
	record, err := st.PrepareInstall(ctx, source, nativeAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName), req)
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

func TestMessageSendAuthorityCurrentTupleAndGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL")
	}
	st, _, record, chat := messageSendStore(t)
	ctx := context.Background()
	var binding *commonv1.BotMessageSendBinding
	require.NoError(t, st.WithMessageSend(ctx, testAuthority(t, st, record.BotID), record.ActorProfileID, chat, func(call context.Context, b *commonv1.BotMessageSendBinding) error {
		binding = proto.Clone(b).(*commonv1.BotMessageSendBinding)
		allowed, err := st.CheckCurrentMessageSendAuthority(call, b)
		require.NoError(t, err)
		require.True(t, allowed, "callback must not deadlock producer guard")
		deadline, ok := call.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 5*time.Second)
		blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		err = st.SetChatEnabled(blocked, record.BotID, chat, record.SpaceID, record.AdministratorProfile, false)
		require.Error(t, err, "disable must wait for actual guarded send")
		return nil
	}))
	allowed, err := st.CheckCurrentMessageSendAuthority(ctx, binding)
	require.NoError(t, err)
	require.True(t, allowed)
	changed := proto.Clone(binding).(*commonv1.BotMessageSendBinding)
	changed.InstallationGeneration++
	allowed, err = st.CheckCurrentMessageSendAuthority(ctx, changed)
	require.NoError(t, err)
	require.False(t, allowed)
	changed = proto.Clone(binding).(*commonv1.BotMessageSendBinding)
	changed.ChatId = uuid.NewString()
	allowed, err = st.CheckCurrentMessageSendAuthority(ctx, changed)
	require.NoError(t, err)
	require.False(t, allowed)
	require.NoError(t, st.SetChatEnabled(ctx, record.BotID, chat, record.SpaceID, record.AdministratorProfile, false))
	allowed, err = st.CheckCurrentMessageSendAuthority(ctx, binding)
	require.NoError(t, err)
	require.False(t, allowed)
	called := false
	require.Error(t, st.WithMessageSend(ctx, testAuthority(t, st, record.BotID), record.ActorProfileID, chat, func(context.Context, *commonv1.BotMessageSendBinding) error { called = true; return nil }))
	require.False(t, called)
}

func TestMessageSendCancellationWaitsGuardAndRevokesTuple(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL")
	}
	st, source, record, chat := messageSendStore(t)
	req := &botv1.UninstallBotFromSpaceRequest{BotId: record.BotID.String(), SpaceId: record.SpaceID.String()}
	admin := nativeAdmin(t, req, botv1.BotService_UninstallBotFromSpace_FullMethodName)
	var binding *commonv1.BotMessageSendBinding
	require.NoError(t, st.WithMessageSend(t.Context(), testAuthority(t, st, record.BotID), record.ActorProfileID, chat, func(ctx context.Context, b *commonv1.BotMessageSendBinding) error {
		binding = proto.Clone(b).(*commonv1.BotMessageSendBinding)
		blocked, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		_, err := st.BeginCancel(blocked, source, admin, record.SpaceID, record.BotID)
		require.Error(t, err, "uninstall must serialize behind admitted RPC")
		allowed, err := st.CheckCurrentMessageSendAuthority(ctx, b)
		require.NoError(t, err)
		require.True(t, allowed, "failed cancellation must not mutate current authority")
		return nil
	}))
	_, err := st.BeginCancel(t.Context(), source, admin, record.SpaceID, record.BotID)
	require.NoError(t, err)
	allowed, err := st.CheckCurrentMessageSendAuthority(t.Context(), binding)
	require.NoError(t, err)
	require.False(t, allowed, "cancellation denies before cleanup finishes")
	called := false
	require.Error(t, st.WithMessageSend(t.Context(), testAuthority(t, st, record.BotID), record.ActorProfileID, chat, func(context.Context, *commonv1.BotMessageSendBinding) error { called = true; return nil }))
	require.False(t, called)
}

func TestMessageSendCallbackIndependentFromSaturatedProducerPool(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL")
	}
	st, _, record, chat := messageSendStore(t)
	cfg := st.Pool.Config().Copy()
	cfg.MaxConns, cfg.MinConns = 1, 0
	producer, err := pgxpool.NewWithConfig(t.Context(), cfg)
	require.NoError(t, err)
	defer producer.Close()
	st.Pool = producer
	reader, err := NewMessageAuthorityStore(t.Context(), producer)
	require.NoError(t, err)
	defer reader.Pool.Close()
	require.NoError(t, st.WithMessageSend(t.Context(), testAuthority(t, st, record.BotID), record.ActorProfileID, chat, func(ctx context.Context, binding *commonv1.BotMessageSendBinding) error {
		require.EqualValues(t, 1, producer.Stat().AcquiredConns())
		allowed, err := reader.CheckCurrentMessageSendAuthority(ctx, binding)
		require.NoError(t, err)
		require.True(t, allowed)
		return nil
	}))
}
