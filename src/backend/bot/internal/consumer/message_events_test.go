package consumer_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/bot/internal/consumer"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func startBotStore(t *testing.T) *store.BotStore {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000001_init.up.sql")
	b, err := os.ReadFile(root)
	require.NoError(t, err)
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botdb", "")
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
	return &store.BotStore{Pool: pool}
}

func TestHandleMessageSent_EnqueuesForPollingBot(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)

	owner := uuid.New()
	botRow, _, err := st.CreateBot(ctx, owner, "MsgBot", "desc", `["TEXT_CHAT_SEND_MESSAGES"]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode = true WHERE id = $1`, botRow.ID)
	require.NoError(t, err)

	chatID := uuid.New()
	spaceID := uuid.New()
	_, err = st.InstallInSpace(ctx, botRow.ID, spaceID, owner, nil)
	require.NoError(t, err)
	require.NoError(t, st.SetChatEnabled(ctx, botRow.ID, chatID, spaceID, owner, true))

	sender := uuid.New()
	ev := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				ChatId:          chatID.String(),
				MessageId:       uuid.NewString(),
				SenderProfileId: sender.String(),
			},
		},
	}
	data, err := proto.Marshal(ev)
	require.NoError(t, err)

	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, data))

	ids, types, _, err := st.ListPendingEvents(ctx, botRow.ID, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, "message", types[0])
}
