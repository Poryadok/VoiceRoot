package consumer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

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
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db", "000003_message_delivery_outbox.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
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

	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	ids, types, _, err := st.ListPendingEvents(ctx, botRow.ID, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, "message", types[0])
	require.NoError(t, h.HandleMessageSent(ctx, data))
	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	ids, _, _, err = st.ListPendingEvents(ctx, botRow.ID, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
}

func TestHandleMessageSent_PollingFailureDoesNotStarveWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	chatID, spaceID, owner := uuid.New(), uuid.New(), uuid.New()
	poll, _, err := st.CreateBot(ctx, owner, "Poll", "", `[]`, uuid.New())
	require.NoError(t, err)
	web, _, err := st.CreateBot(ctx, owner, "Web", "", `[]`, uuid.New())
	require.NoError(t, err)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode = true WHERE id = $1`, poll.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2 WHERE id = $1`, web.ID, server.URL)
	require.NoError(t, err)
	for _, bot := range []store.BotRow{poll, web} {
		_, err = st.InstallInSpace(ctx, bot.ID, spaceID, owner, []uuid.UUID{chatID})
		require.NoError(t, err)
	}
	_, err = st.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION reject_poll_delivery() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.bot_id = '`+poll.ID.String()+`' THEN RAISE EXCEPTION 'poll unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_poll BEFORE INSERT ON bot_event_log FOR EACH ROW EXECUTE FUNCTION reject_poll_delivery()`)
	require.NoError(t, err)
	messageID := uuid.NewString()
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: messageID, SenderProfileId: uuid.NewString()}}})
	require.NoError(t, err)
	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, data))
	for i := 0; i < 2; i++ {
		_, _ = h.ProcessNext(ctx)
	}
	require.EqualValues(t, 1, calls.Load())
	require.NoError(t, h.HandleMessageSent(ctx, data))
	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, calls.Load())
}

func TestHandleMessageSent_DatabaseFailureLeavesSourceForReplay(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner, chatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "RetryBot", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode = true WHERE id = $1`, bot.ID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, spaceID, owner, []uuid.UUID{chatID})
	require.NoError(t, err)
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: uuid.NewString(), SenderProfileId: uuid.NewString()}}})
	require.NoError(t, err)
	h := &consumer.MessageHandler{Store: st}
	_, err = st.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION reject_intake() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intake unavailable'; END $$; CREATE TRIGGER reject_intake BEFORE INSERT ON bot_message_deliveries FOR EACH ROW EXECUTE FUNCTION reject_intake()`)
	require.NoError(t, err)
	require.ErrorContains(t, h.HandleMessageSent(ctx, data), "intake unavailable")
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER reject_intake ON bot_message_deliveries`)
	require.NoError(t, err)
	require.NoError(t, h.HandleMessageSent(ctx, data))
	worked, err := h.ProcessNext(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
}

func TestQueuedMessageRechecksCurrentBotAccessAndWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner, chatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	var oldCalls, newCalls atomic.Int32
	oldServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { oldCalls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer oldServer.Close()
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { newCalls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer newServer.Close()
	web, _, err := st.CreateBot(ctx, owner, "AccessWeb", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2 WHERE id = $1`, web.ID, oldServer.URL)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, web.ID, spaceID, owner, []uuid.UUID{chatID})
	require.NoError(t, err)
	makeMessage := func() []byte {
		data, marshalErr := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: uuid.NewString(), SenderProfileId: uuid.NewString()}}})
		require.NoError(t, marshalErr)
		return data
	}
	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, makeMessage()))
	require.NoError(t, st.SetChatEnabled(ctx, web.ID, chatID, spaceID, owner, false))
	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	require.Zero(t, oldCalls.Load(), "disabled chat must not receive queued webhook")
	require.NoError(t, st.SetChatEnabled(ctx, web.ID, chatID, spaceID, owner, true))
	require.NoError(t, h.HandleMessageSent(ctx, makeMessage()))
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2, webhook_secret = 'rotated' WHERE id = $1`, web.ID, newServer.URL)
	require.NoError(t, err)
	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	require.Zero(t, oldCalls.Load(), "stale webhook URL must not be used")
	require.EqualValues(t, 1, newCalls.Load())
	require.NoError(t, h.HandleMessageSent(ctx, makeMessage()))
	require.NoError(t, st.UninstallFromSpace(ctx, web.ID, spaceID))
	_, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, newCalls.Load(), "uninstalled bot must not receive queued webhook")
}

func TestSlowWebhookDoesNotBlockOtherRecipientOrRaceRevocation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner, chatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	started, release := make(chan struct{}), make(chan struct{})
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { close(started); <-release; w.WriteHeader(http.StatusOK) }))
	defer slowServer.Close()
	var healthyCalls atomic.Int32
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { healthyCalls.Add(1); w.WriteHeader(http.StatusOK) }))
	defer healthyServer.Close()
	slow, _, err := st.CreateBot(ctx, owner, "Slow", "", `[]`, uuid.New())
	require.NoError(t, err)
	healthy, _, err := st.CreateBot(ctx, owner, "Healthy", "", `[]`, uuid.New())
	require.NoError(t, err)
	for _, entry := range []struct {
		id  uuid.UUID
		url string
	}{{slow.ID, slowServer.URL}, {healthy.ID, healthyServer.URL}} {
		_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2 WHERE id = $1`, entry.id, entry.url)
		require.NoError(t, err)
		_, err = st.InstallInSpace(ctx, entry.id, spaceID, owner, []uuid.UUID{chatID})
		require.NoError(t, err)
	}
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: uuid.NewString(), SenderProfileId: uuid.NewString()}}})
	require.NoError(t, err)
	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, data))
	_, err = st.Pool.Exec(ctx, `UPDATE bot_message_deliveries SET next_attempt_at = now() - interval '1 hour' WHERE bot_id = $1`, slow.ID)
	require.NoError(t, err)
	deliveryDone := make(chan error, 1)
	go func() { _, deliverErr := h.ProcessNext(ctx); deliveryDone <- deliverErr }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("slow webhook not started")
	}
	revokeDone := make(chan error, 1)
	go func() { revokeDone <- st.SetChatEnabled(ctx, slow.ID, chatID, spaceID, owner, false) }()
	select {
	case <-revokeDone:
		t.Fatal("revocation committed during webhook POST")
	case <-time.After(100 * time.Millisecond):
	}
	worked, err := h.ProcessNext(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	require.EqualValues(t, 1, healthyCalls.Load())
	close(release)
	require.NoError(t, <-deliveryDone)
	require.NoError(t, <-revokeDone)
}

func TestPermanentWebhookFailureIsDurablyFailed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner, chatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	bot, _, err := st.CreateBot(ctx, owner, "BadWebhook", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url = $2 WHERE id = $1`, bot.ID, server.URL)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, spaceID, owner, []uuid.UUID{chatID})
	require.NoError(t, err)
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: uuid.NewString(), SenderProfileId: uuid.NewString()}}})
	require.NoError(t, err)
	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, data))
	worked, err := h.ProcessNext(ctx)
	require.True(t, worked)
	require.ErrorContains(t, err, "webhook status 400")
	var status string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status FROM bot_message_deliveries WHERE bot_id = $1`, bot.ID).Scan(&status))
	require.Equal(t, "failed", status)
	worked, err = h.ProcessNext(ctx)
	require.NoError(t, err)
	require.False(t, worked)
}

func TestQueuedPollingMessageIsCanceledAfterChatDisable(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStore(t)
	owner, chatID, spaceID := uuid.New(), uuid.New(), uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "AccessPoll", "", `[]`, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode = true WHERE id = $1`, bot.ID)
	require.NoError(t, err)
	_, err = st.InstallInSpace(ctx, bot.ID, spaceID, owner, []uuid.UUID{chatID})
	require.NoError(t, err)
	data, err := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: uuid.NewString(), SenderProfileId: uuid.NewString()}}})
	require.NoError(t, err)
	h := &consumer.MessageHandler{Store: st}
	require.NoError(t, h.HandleMessageSent(ctx, data))
	require.NoError(t, st.SetChatEnabled(ctx, bot.ID, chatID, spaceID, owner, false))
	worked, err := h.ProcessNext(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	ids, _, _, err := st.ListPendingEvents(ctx, bot.ID, 10)
	require.NoError(t, err)
	require.Empty(t, ids)
}
