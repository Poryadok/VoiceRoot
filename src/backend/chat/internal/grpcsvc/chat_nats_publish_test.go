package grpcsvc

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
)

type spyChatEvents struct {
	mu            sync.Mutex
	created       [][2]string // chat_id, type
	updated       []chatUpdatedEvent
	memberChanged [][3]string // chat_id, profile_id, change
}

type chatUpdatedEvent struct {
	chatID        string
	changedFields []string
}

func (s *spyChatEvents) PublishChatCreated(_ context.Context, chatID, chatType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, [2]string{chatID, chatType})
	return nil
}

func (s *spyChatEvents) PublishChatMemberChanged(_ context.Context, chatID, profileID, change string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.memberChanged = append(s.memberChanged, [3]string{chatID, profileID, change})
	return nil
}

func (s *spyChatEvents) PublishChatUpdated(_ context.Context, chatID string, changedFields []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updated = append(s.updated, chatUpdatedEvent{chatID: chatID, changedFields: append([]string(nil), changedFields...)})
	return nil
}

func (*spyChatEvents) PublishChatDeleted(context.Context, *eventsv1.ChatStreamEvent) error {
	return nil
}

func (s *spyChatEvents) snapshot() (created [][2]string, memberChanged [][3]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]string(nil), s.created...), append([][3]string(nil), s.memberChanged...)
}

func (s *spyChatEvents) updatedSnapshot() []chatUpdatedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]chatUpdatedEvent, len(s.updated))
	for i, event := range s.updated {
		out[i] = chatUpdatedEvent{chatID: event.chatID, changedFields: append([]string(nil), event.changedFields...)}
	}
	return out
}

// TestChatGRPC_ChatEvents_NewDMPublishesOnce documents chat-service.md / jetstream_events.proto:
// new DM emits chat.created (dm) and two chat.member_changed joined; idempotent GetDM does not re-emit.
func TestChatGRPC_ChatEvents_NewDMPublishesOnce(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	spy := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithChatEventsPublisher(spy))
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	r1, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := r1.GetChat().GetId()

	cr, mc := spy.snapshot()
	require.Len(t, cr, 1)
	require.Equal(t, chatID, cr[0][0])
	require.Equal(t, "dm", cr[0][1])
	require.Len(t, mc, 2)
	require.Equal(t, [][3]string{
		{chatID, profA.String(), "joined"},
		{chatID, profB.String(), "joined"},
	}, mc)

	_, err = client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	cr, mc = spy.snapshot()
	require.Len(t, cr, 1)
	require.Len(t, mc, 2)
}

func TestChatGRPC_UpdateChatPublishesChangedFieldsOnlyAfterSuccessfulMutation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner, memberA, memberB := ids[0], ids[1], ids[2]
	spy := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithChatEventsPublisher(spy))
	t.Cleanup(cleanup)
	chat := createStandaloneGroup(t, client, profiles, owner, "Before", memberA, memberB)

	name, topic := "After", "planning"
	threadsEnabled, allowMainFeed := true, true
	_, err := client.UpdateChat(ctxFor(t, profiles, owner), &chatv1.UpdateChatRequest{
		ChatId: chat.GetId(), Name: &name, Topic: &topic,
		ThreadsEnabled: &threadsEnabled, AllowUserMainFeed: &allowMainFeed,
	})
	require.NoError(t, err)
	require.Equal(t, []chatUpdatedEvent{{
		chatID: chat.GetId(), changedFields: []string{"name", "topic", "threads_enabled", "allow_user_main_feed"},
	}}, spy.updatedSnapshot())

	// An all-fields-absent request is the existing store no-op and must not
	// manufacture an update event.
	_, err = client.UpdateChat(ctxFor(t, profiles, owner), &chatv1.UpdateChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.Len(t, spy.updatedSnapshot(), 1)

	// A member without the owner/admin capability is rejected before publish.
	deniedTopic := "denied"
	_, err = client.UpdateChat(ctxFor(t, profiles, memberA), &chatv1.UpdateChatRequest{
		ChatId: chat.GetId(), Topic: &deniedTopic,
	})
	require.Error(t, err)
	require.Len(t, spy.updatedSnapshot(), 1)
}

func TestChatGRPC_AcceptDMRequestPublishesInboxChangeToBothProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA, accB := uuid.New(), uuid.New()
	profA, profB := uuid.New(), uuid.New()
	spy := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, pool, mapProfileAccounts{profA: accA, profB: accB}, nil, nil, WithChatEventsPublisher(spy))
	t.Cleanup(cleanup)

	created, err := client.CreateDM(withAccountProfileCtx(ctx, accA, profA), &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := created.GetChat().GetId()
	_, err = client.AcceptDMRequest(withAccountProfileCtx(ctx, accB, profB), &chatv1.AcceptDMRequestRequest{ChatId: chatID})
	require.NoError(t, err)
	_, changes := spy.snapshot()
	require.Contains(t, changes, [3]string{chatID, profA.String(), "inbox_bucket_changed"})
	require.Contains(t, changes, [3]string{chatID, profB.String(), "inbox_bucket_changed"})
}
