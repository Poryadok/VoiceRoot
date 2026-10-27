package consumer_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	botv1 "voice.app/voice/bot/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/bot/internal/consumer"
	"voice/backend/bot/internal/grpcsvc"
)

// This exercises the production bind/callback/workers against real JetStream
// and PostgreSQL. It is component evidence, not a deployed webhook E2E.
func TestMessageEventsJetStreamDurableDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL and NATS containers")
	}
	t.Setenv("BOT_NATS_CREDS_FILE", "")
	t.Setenv("BOT_WEBHOOK_DELIVERY_ATTEMPTS", "1")
	ctx := context.Background()
	st := startBotStore(t)
	url, js := startMessageJetStream(t)
	owner, chat, space := uuid.New(), uuid.New(), uuid.New()
	poll, pollToken, err := st.CreateBot(ctx, owner, "StreamPoll", "", `["TEXT_CHAT_READ_HISTORY"]`, uuid.New())
	require.NoError(t, err)
	web, _, err := st.CreateBot(ctx, owner, "StreamWebhook", "", `["TEXT_CHAT_READ_HISTORY"]`, uuid.New())
	require.NoError(t, err)
	var healthy atomic.Bool
	var failedCalls, successfulCalls atomic.Int32
	requests := make(chan []byte, 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mac := hmac.New(sha256.New, []byte(web.WebhookSecret))
		_, _ = mac.Write([]byte(r.Header.Get("X-Voice-Timestamp") + "."))
		_, _ = mac.Write(body)
		if r.Header.Get("X-Voice-Signature") != "v1="+hex.EncodeToString(mac.Sum(nil)) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		requests <- body
		if !healthy.Load() {
			failedCalls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		successfulCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET is_polling_mode=true WHERE id=$1`, poll.ID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bots SET webhook_url=$2 WHERE id=$1`, web.ID, server.URL)
	require.NoError(t, err)
	for _, bot := range []uuid.UUID{poll.ID, web.ID} {
		_, err = st.InstallInSpace(ctx, bot, space, owner, []uuid.UUID{chat})
		require.NoError(t, err)
	}
	message := uuid.New()
	publish := func(chatID, messageID uuid.UUID) uint64 {
		data, marshalErr := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chatID.String(), MessageId: messageID.String(), SenderProfileId: owner.String()}}})
		require.NoError(t, marshalErr)
		ack, publishErr := js.Publish("message.sent", data)
		require.NoError(t, publishErr)
		require.Equal(t, "message_events", ack.Stream)
		return ack.Sequence
	}
	// Publish before binding: DeliverAll must retain work while Bot is offline.
	sequence := publish(chat, message)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION reject_stream_intake() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intake unavailable'; END $$; CREATE TRIGGER reject_stream_intake BEFORE INSERT ON bot_message_deliveries FOR EACH ROW EXECUTE FUNCTION reject_stream_intake()`)
	require.NoError(t, err)
	start := func() func() {
		stop, startErr := consumer.StartMessageEventsConsumer(ctx, &consumer.MessageHandler{Store: st}, url, nil)
		require.NoError(t, startErr)
		return stop
	}
	stop := start()
	t.Cleanup(func() {
		if stop != nil {
			stop()
		}
	})
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo("message_events", "bot_message_events")
		return infoErr == nil && info.Delivered.Consumer > 1 && info.AckFloor.Stream < sequence
	}, 10*time.Second, 20*time.Millisecond, "DB failure must be redelivered without source ACK")
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER reject_stream_intake ON bot_message_deliveries`)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		var count int
		err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_event_log WHERE bot_id=$1 AND source_message_id=$2`, poll.ID, message).Scan(&count)
		return err == nil && count == 1 && failedCalls.Load() > 0
	}, 10*time.Second, 20*time.Millisecond)
	// The source is ACKed after durable intake, even while a webhook is failing.
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo("message_events", "bot_message_events")
		return infoErr == nil && info.AckFloor.Stream == sequence && info.NumAckPending == 0
	}, 5*time.Second, 20*time.Millisecond)
	stop()
	stop = nil
	var status, deliveryID string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT status,id::text FROM bot_message_deliveries WHERE bot_id=$1 AND message_id=$2`, web.ID, message).Scan(&status, &deliveryID))
	require.Equal(t, "pending", status)
	healthy.Store(true)
	// A new consumer instance must recover the committed failed recipient.
	stop = start()
	require.Eventually(t, func() bool {
		var count int
		err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_message_deliveries WHERE message_id=$1 AND status='delivered'`, message).Scan(&count)
		return err == nil && count == 2
	}, 10*time.Second, 20*time.Millisecond)
	require.EqualValues(t, 1, successfulCalls.Load())
	// Source replay and an unauthorized chat must both be consumed, without
	// another webhook, polling event, or recipient intent.
	publish(chat, message)
	deniedMessage := uuid.New()
	last := publish(uuid.New(), deniedMessage)
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo("message_events", "bot_message_events")
		return infoErr == nil && info.AckFloor.Stream == last && info.NumAckPending == 0
	}, 5*time.Second, 20*time.Millisecond)
	stop()
	stop = nil
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM bot_message_deliveries`).Scan(&count))
	require.Equal(t, 2, count)
	ids, types, _, err := st.ListPendingEvents(ctx, poll.ID, 10)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	require.Equal(t, []string{"message"}, types)
	// A failed application Send keeps the event pending. The next actual gRPC
	// polling request receives those same bytes and consumes it exactly once.
	pollCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-bot-token", pollToken))
	svc := grpcsvc.NewBotGRPC(st, nil)
	sendErr := errors.New("poll transport unavailable")
	require.ErrorIs(t, svc.PollEvents(&botv1.PollEventsRequest{}, &failedMessagePoll{ctx: pollCtx, err: sendErr}), sendErr)
	idsAfter, _, _, err := st.ListPendingEvents(ctx, poll.ID, 10)
	require.NoError(t, err)
	require.Equal(t, ids, idsAfter)
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.InTapHandle(grpcsvc.PollEventsTransportTap()))
	botv1.RegisterBotServiceServer(grpcServer, svc)
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(grpcServer.Stop)
	conn, err := grpc.NewClient("passthrough:///bot", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	callCtx, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-bot-token", pollToken)), 5*time.Second)
	defer cancel()
	stream, err := botv1.NewBotServiceClient(conn).PollEvents(callCtx, &botv1.PollEventsRequest{})
	require.NoError(t, err)
	response, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, ids[0].String(), response.GetBotEvent().GetEventId())
	var event map[string]any
	require.NoError(t, json.Unmarshal([]byte(response.GetBotEvent().GetPayloadJson()), &event))
	require.Equal(t, message.String(), event["message_id"])
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
	idsAfter, _, _, err = st.ListPendingEvents(ctx, poll.ID, 10)
	require.NoError(t, err)
	require.Empty(t, idsAfter)
	require.EqualValues(t, 1, successfulCalls.Load())
	for len(requests) > 0 {
		body := <-requests
		var payload struct {
			Options map[string]any `json:"options"`
		}
		require.NoError(t, json.Unmarshal(body, &payload))
		require.Equal(t, deliveryID, payload.Options["delivery_id"])
		require.Equal(t, message.String(), payload.Options["message_id"])
	}
}

type failedMessagePoll struct {
	grpc.ServerStream
	ctx context.Context
	err error
}

func (s *failedMessagePoll) Context() context.Context             { return s.ctx }
func (s *failedMessagePoll) Send(*botv1.PollEventsResponse) error { return s.err }

func startMessageJetStream(t *testing.T) (string, nats.JetStreamContext) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "nats:2.10-alpine", ExposedPorts: []string{"4222/tcp"}, Cmd: []string{"-js"},
		WaitingFor: wait.ForLog("Server is ready").WithStartupTimeout(time.Minute),
	}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	url := "nats://" + net.JoinHostPort(host, port.Port())
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "message_events", Subjects: []string{"message.sent"}, Storage: nats.FileStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer("message_events", &nats.ConsumerConfig{Durable: "bot_message_events", FilterSubject: "message.sent", DeliverSubject: "_INBOX.voice.bot.bot_message_events", DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: time.Second})
	require.NoError(t, err)
	return url, js
}
