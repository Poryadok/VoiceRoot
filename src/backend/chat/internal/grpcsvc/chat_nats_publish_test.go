package grpcsvc

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	chatv1 "voice.app/voice/chat/v1"
)

type spyChatEvents struct {
	mu              sync.Mutex
	created         [][2]string // chat_id, type
	memberChanged   [][3]string // chat_id, profile_id, change
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

func (s *spyChatEvents) snapshot() (created [][2]string, memberChanged [][3]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]string(nil), s.created...), append([][3]string(nil), s.memberChanged...)
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
