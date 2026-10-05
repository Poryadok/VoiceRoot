package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/integrationtest"
)

type notificationConsumerRestartSpec struct {
	name           string
	service        string
	stream         string
	filter         string
	deliverSubject string
	subject        string
	recipientID    uuid.UUID
	event          proto.Message
	expected       push.Payload
	recordDebug    bool
	safeAssertions bool
	prepare        func(*testing.T)
	verify         func(*testing.T)
	run            func(context.Context, string, *store.DeviceTokenStore, *dispatch.PushDispatcher) error
}

type notificationConsumerRestartFixture struct {
	serverURL string
	js        nats.JetStreamContext
	tokens    *store.DeviceTokenStore
}

type composeFCMCorrelation struct {
	MessageID       string `json:"message_id"`
	ChatID          string `json:"chat_id"`
	SenderProfileID string `json:"sender_profile_id"`
}

// TestComposeFcmEventCorrelationProbe is opt-in from the failing disposable
// Compose smoke only. It reads retained JetStream messages and consumer info;
// it never creates or changes a stream, consumer, subscription, or message.
func TestComposeFcmEventCorrelationProbe(t *testing.T) {
	correlationPath := os.Getenv("VOICE_FCM_DIAGNOSTIC_FILE")
	if correlationPath == "" {
		return
	}
	fileInfo, err := os.Lstat(correlationPath)
	if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm()&0o077 != 0 || fileInfo.Size() > 4096 {
		composeFCMProbeUnavailable("correlation_file")
		return
	}
	correlationBytes, err := os.ReadFile(correlationPath)
	if err != nil {
		composeFCMProbeUnavailable("correlation_file")
		return
	}
	var correlation composeFCMCorrelation
	if json.Unmarshal(correlationBytes, &correlation) != nil || correlation.MessageID == "" || correlation.ChatID == "" || correlation.SenderProfileID == "" {
		composeFCMProbeUnavailable("correlation_invalid")
		return
	}
	if _, err := uuid.Parse(correlation.MessageID); err != nil {
		composeFCMProbeUnavailable("correlation_invalid")
		return
	}
	if _, err := uuid.Parse(correlation.ChatID); err != nil {
		composeFCMProbeUnavailable("correlation_invalid")
		return
	}
	if _, err := uuid.Parse(correlation.SenderProfileID); err != nil {
		composeFCMProbeUnavailable("correlation_invalid")
		return
	}
	natsURL := os.Getenv("NATS_URL")
	parsedURL, err := url.Parse(natsURL)
	if err != nil || parsedURL.Scheme != "nats" || parsedURL.Hostname() != "127.0.0.1" || parsedURL.User != nil || parsedURL.Path != "" || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		composeFCMProbeUnavailable("endpoint_rejected")
		return
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil || port < 1 || port > 65535 {
		composeFCMProbeUnavailable("endpoint_rejected")
		return
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second), nats.MaxReconnects(0))
	if err != nil {
		composeFCMProbeUnavailable("nats_connect")
		return
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		composeFCMProbeUnavailable("jetstream")
		return
	}
	streamInfo, err := js.StreamInfo(jsStreamMessageEvents)
	if err != nil || streamInfo == nil {
		composeFCMProbeUnavailable("stream_info")
		return
	}
	const maxMessages = uint64(256)
	first, last := streamInfo.State.FirstSeq, streamInfo.State.LastSeq
	start := first
	if last >= first && last-first+1 > maxMessages {
		start = last - maxMessages + 1
	}
	coverage := "complete_retained"
	if last >= first && start != first {
		coverage = "bounded"
	}
	if last < first {
		coverage = "unknown"
	}
	var matches uint64
	var eventSequence uint64
	scanCtx, cancelScan := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelScan()
	for sequence := start; coverage != "unknown" && sequence <= last; sequence++ {
		if scanCtx.Err() != nil {
			coverage = "unknown"
			break
		}
		message, getErr := js.GetMsg(jsStreamMessageEvents, sequence, nats.Context(scanCtx))
		if getErr != nil {
			coverage = "unknown"
			break
		}
		var event eventsv1.MessageStreamEvent
		if proto.Unmarshal(message.Data, &event) != nil {
			coverage = "unknown"
			break
		}
		messageSent := event.GetMessageSent()
		if messageSent != nil && messageSent.GetMessageId() == correlation.MessageID && messageSent.GetChatId() == correlation.ChatID && messageSent.GetSenderProfileId() == correlation.SenderProfileID {
			matches++
			eventSequence = message.Sequence
		}
		if sequence == ^uint64(0) {
			break
		}
	}
	if matches != 1 {
		fmt.Printf("compose_fcm_probe available=true stage=scan event_match_count=%d event_scan=%s consumer_info_available=unknown delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown\n", matches, coverage)
		return
	}
	consumerInfoAvailable := "unknown"
	deliveredAtOrAfter := "unknown"
	ackFloorAtOrAfter := "unknown"
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, consumer.SharedDurable("message"))
		if infoErr == nil && info != nil {
			consumerInfoAvailable = "true"
			deliveredAtOrAfter = sequenceAtOrAfter(info.Delivered.Stream, eventSequence)
			ackFloorAtOrAfter = sequenceAtOrAfter(info.AckFloor.Stream, eventSequence)
			if ackFloorAtOrAfter == "true" {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Printf("compose_fcm_probe available=true stage=consumer_info event_match_count=1 event_scan=%s consumer_info_available=%s delivered_seq_ge_event=%s ack_floor_seq_ge_event=%s\n", coverage, consumerInfoAvailable, deliveredAtOrAfter, ackFloorAtOrAfter)
}

func sequenceAtOrAfter(actual, target uint64) string {
	if actual >= target {
		return "true"
	}
	return "false"
}

func TestSequenceAtOrAfterDoesNotConflateUnavailableWithFalse(t *testing.T) {
	require.Equal(t, "true", sequenceAtOrAfter(12, 12))
	require.Equal(t, "true", sequenceAtOrAfter(13, 12))
	require.Equal(t, "false", sequenceAtOrAfter(11, 12))
}

func composeFCMProbeUnavailable(stage string) {
	fmt.Printf("compose_fcm_probe available=false stage=%s event_match_count=0 event_scan=unknown consumer_info_available=unknown delivered_seq_ge_event=unknown ack_floor_seq_ge_event=unknown\n", stage)
}

type notificationRestartPush struct {
	profileID uuid.UUID
	payload   push.Payload
}

type notificationRestartFCM struct {
	sent chan notificationRestartPush
}

type notificationRestartChatServer struct {
	chatv1.UnimplementedChatServiceServer
	senderID       string
	recipientID    string
	chatID         string
	listCalls      atomic.Int32
	callerMatches  atomic.Bool
	requestMatches atomic.Bool
}

func (s *notificationRestartChatServer) ListMembers(ctx context.Context, req *chatv1.ListMembersRequest) (*chatv1.ListMembersResponse, error) {
	s.listCalls.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	callers := md.Get("x-voice-internal-caller")
	s.callerMatches.Store(len(callers) == 1 && callers[0] == "notification")
	s.requestMatches.Store(req.GetChatId() == s.chatID && req.GetPage().GetCursor() == "")
	bucket := "main"
	return &chatv1.ListMembersResponse{MemberList: &chatv1.MemberList{Members: []*chatv1.ChatMember{
		{ProfileId: s.senderID, InboxBucket: &bucket},
		{ProfileId: s.recipientID, InboxBucket: &bucket},
	}}}, nil
}

type notificationRestartTokenProbe struct {
	store           *store.DeviceTokenStore
	expectedProfile uuid.UUID
	listCalls       atomic.Int32
	profileMatches  atomic.Bool
}

func (p *notificationRestartTokenProbe) ListByProfile(ctx context.Context, profileID uuid.UUID) ([]store.DeviceToken, error) {
	p.listCalls.Add(1)
	p.profileMatches.Store(profileID == p.expectedProfile)
	return p.store.ListByProfile(ctx, profileID)
}

func (p *notificationRestartTokenProbe) DeleteByToken(ctx context.Context, token string) error {
	return p.store.DeleteByToken(ctx, token)
}

func (s notificationRestartFCM) Send(_ context.Context, profileID uuid.UUID, _ store.DeviceToken, payload fcm.PushPayload) error {
	s.sent <- notificationRestartPush{profileID: profileID, payload: push.Payload(payload)}
	return nil
}

type notificationConsumerRun struct {
	cancel context.CancelFunc
	done   chan error
	exited chan struct{}
	once   sync.Once
	err    error
	joined bool
}

func (r *notificationConsumerRun) stop() (error, bool) {
	r.once.Do(func() {
		r.cancel()
		select {
		case <-r.exited:
			r.err = <-r.done
			r.joined = true
		case <-time.After(5 * time.Second):
			r.joined = false
		}
	})
	return r.err, r.joined
}

func TestNotificationJetStreamRestartDrainsBacklogForEachReadyDurable(t *testing.T) {
	fixture := newNotificationConsumerRestartFixture(t)
	for _, spec := range []notificationConsumerRestartSpec{
		socialFriendRequestRestartSpec(),
		voiceMemberJoinedRestartSpec(),
		matchFoundRestartSpec(),
		storyMentionRestartSpec(),
		messageSentRestartSpec(),
	} {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			runNotificationConsumerRestartProof(t, fixture, spec)
		})
	}
}

func newNotificationConsumerRestartFixture(t *testing.T) *notificationConsumerRestartFixture {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "notificationdb", "")
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "resolve notification test helper path")
	migrationPath := filepath.Join(filepath.Dir(sourceFile), "..", "migrations", "notification_db", "000001_init.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)

	server, err := natsserver.NewServer(&natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1})
	require.NoError(t, err)
	go server.Start()
	t.Cleanup(func() { server.Shutdown() })
	require.True(t, server.ReadyForConnections(10*time.Second), "embedded JetStream server becomes ready")

	provisioner, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { provisioner.Close() })
	js, err := provisioner.JetStream()
	require.NoError(t, err)
	return &notificationConsumerRestartFixture{
		serverURL: server.ClientURL(), js: js, tokens: &store.DeviceTokenStore{Pool: pool},
	}
}

func runNotificationConsumerRestartProof(t *testing.T, fixture *notificationConsumerRestartFixture, spec notificationConsumerRestartSpec) {
	t.Helper()
	require.NotEmpty(t, spec.name)
	require.NotEmpty(t, spec.service)
	require.NotEmpty(t, spec.stream)
	require.NotEmpty(t, spec.filter)
	require.NotEmpty(t, spec.deliverSubject)
	require.NotEmpty(t, spec.subject)
	require.NotNil(t, spec.event)
	require.NotNil(t, spec.run)
	if spec.prepare != nil {
		spec.prepare(t)
	}

	_, err := fixture.tokens.Register(context.Background(), spec.recipientID, "web", "be199-"+spec.service+"-"+uuid.NewString(), "fcm")
	require.NoError(t, err)
	_, err = fixture.js.AddStream(&nats.StreamConfig{Name: spec.stream, Subjects: []string{spec.filter}})
	require.NoError(t, err)
	durable := consumer.SharedDurable(spec.service)
	_, err = fixture.js.AddConsumer(spec.stream, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: spec.filter, DeliverSubject: spec.deliverSubject,
		AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy,
	})
	require.NoError(t, err)

	deliveries := make(chan notificationRestartPush, 4)
	var sender fcm.Sender = notificationRestartFCM{sent: deliveries}
	debugRecorderEnabled := false
	if spec.recordDebug {
		var configErr error
		debugRecorderEnabled, configErr = parseDebugHTTPEnabled(func(name string) (string, bool) {
			if name == debugHTTPEnabledEnv {
				return "true", true
			}
			return "", false
		}, true)
		require.NoError(t, configErr)
		require.True(t, debugRecorderEnabled)
		sender = maybeRecordFCMSender(sender, debugRecorderEnabled)
	}
	dispatcher := &dispatch.PushDispatcher{FCM: sender}
	start := func() *notificationConsumerRun {
		runCtx, cancel := context.WithCancel(context.Background())
		readiness := newNotificationConsumerReadiness(spec.service)
		runner := &notificationConsumerRun{cancel: cancel, done: make(chan error, 1), exited: make(chan struct{})}
		go func() {
			defer close(runner.exited)
			runner.done <- spec.run(withNotificationConsumerReadiness(runCtx, readiness, spec.service), fixture.serverURL, fixture.tokens, dispatcher)
		}()
		t.Cleanup(func() {
			if _, joined := runner.stop(); !joined {
				t.Errorf("%s consumer did not stop during cleanup", spec.service)
			}
		})
		require.Eventually(t, readiness.ready, 5*time.Second, 10*time.Millisecond,
			"%s consumer binds its preprovisioned durable before events publish", spec.service)
		return runner
	}

	first := start()
	firstErr, firstJoined := first.stop()
	require.True(t, firstJoined, "%s first consumer is joined before publishing", spec.service)
	require.ErrorIs(t, firstErr, context.Canceled, "%s consumer exits normally on cancellation", spec.service)
	var (
		unbindAttempts   int
		unbindErrorClass = "not observed"
		lastPushBound    = true
	)
	unboundObserved := assert.Eventually(t, func() bool {
		unbindAttempts++
		info, infoErr := fixture.js.ConsumerInfo(spec.stream, durable)
		if infoErr != nil {
			unbindErrorClass = fmt.Sprintf("%T", infoErr)
			lastPushBound = true
			return false
		}
		unbindErrorClass = "none"
		lastPushBound = info.PushBound
		return !info.PushBound
	}, 5*time.Second, 20*time.Millisecond)
	if !unboundObserved {
		t.Fatalf("%s stopped consumer unbind observation failed before backlog publish: query_attempts=%d query_error_class=%q push_bound=%t",
			spec.service, unbindAttempts, unbindErrorClass, lastPushBound)
	}

	encoded, err := proto.Marshal(spec.event)
	require.NoError(t, err)
	publishAck, err := fixture.js.Publish(spec.subject, encoded)
	require.NoError(t, err)
	type consumerInfoObservation struct {
		querySucceeded bool
		numPending     int64
		numAckPending  int64
		errorType      string
	}
	var observationMu sync.Mutex
	lastCompletedObservation := consumerInfoObservation{
		numPending:    -1,
		numAckPending: -1,
		errorType:     "not observed",
	}
	pendingObserved := assert.Eventually(t, func() bool {
		info, infoErr := fixture.js.ConsumerInfo(spec.stream, durable)
		observation := consumerInfoObservation{}
		if infoErr != nil {
			observation.numPending, observation.numAckPending = -1, -1
			observation.errorType = fmt.Sprintf("%T", infoErr)
		} else {
			observation.querySucceeded = true
			observation.numPending = int64(info.NumPending)
			observation.numAckPending = int64(info.NumAckPending)
		}
		observationMu.Lock()
		lastCompletedObservation = observation
		observationMu.Unlock()
		return infoErr == nil && info.NumPending == 1 && info.NumAckPending == 0
	}, 5*time.Second, 20*time.Millisecond)
	if !pendingObserved {
		observationMu.Lock()
		lastCompleted := lastCompletedObservation
		observationMu.Unlock()
		t.Fatalf("%s event stays pending while its durable consumer is stopped; last completed ConsumerInfo observation query_succeeded=%t num_pending=%d num_ack_pending=%d error_type=%q",
			spec.service, lastCompleted.querySucceeded, lastCompleted.numPending, lastCompleted.numAckPending, lastCompleted.errorType)
	}

	second := start()
	select {
	case got := <-deliveries:
		if spec.safeAssertions {
			require.True(t, got.profileID == spec.recipientID, "%s push is delivered to the configured recipient", spec.service)
			require.True(t, got.payload.Data != nil && got.payload.Data["type"] == spec.expected.Data["type"], "%s push follows the expected notification type", spec.service)
		} else {
			require.Equal(t, spec.recipientID, got.profileID, "%s push is delivered to the documented recipient", spec.service)
			require.Equal(t, spec.expected, got.payload, "%s push payload follows the existing event contract", spec.service)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%s consumer did not deliver the backed-up event", spec.service)
	}
	require.Eventually(t, func() bool {
		info, infoErr := fixture.js.ConsumerInfo(spec.stream, durable)
		return infoErr == nil && info.NumPending == 0 && info.NumAckPending == 0 &&
			info.AckFloor.Stream >= publishAck.Sequence && info.NumRedelivered == 0
	}, 5*time.Second, 20*time.Millisecond, "%s successful push is acknowledged and backlog drains", spec.service)
	select {
	case duplicate := <-deliveries:
		if spec.safeAssertions {
			t.Fatalf("%s delivered a duplicate push after the durable ACK", spec.service)
		}
		t.Fatalf("%s delivered a duplicate push after the durable ACK: %#v", spec.service, duplicate)
	default:
	}
	if spec.recordDebug {
		request := httptest.NewRequest(http.MethodGet, "/debug/recorded-pushes?profile_id="+spec.recipientID.String(), nil)
		response := httptest.NewRecorder()
		notificationHTTPHandlerWithReadinessAndDebug(serviceName, nil, debugRecorderEnabled).ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, "%s capture is available through the gated recorder route", spec.service)
	}
	secondErr, secondJoined := second.stop()
	require.True(t, secondJoined, "%s restarted consumer is joined before fixture shutdown", spec.service)
	require.ErrorIs(t, secondErr, context.Canceled, "%s restarted consumer exits normally on cancellation", spec.service)
	if spec.verify != nil {
		spec.verify(t)
	}
}

func messageSentRestartSpec() notificationConsumerRestartSpec {
	senderID, recipientID := uuid.New(), uuid.New()
	messageID, chatID := uuid.NewString(), uuid.NewString()
	chatServer := &notificationRestartChatServer{
		senderID: senderID.String(), recipientID: recipientID.String(), chatID: chatID,
	}
	var memberLister *chatmembers.GRPCLister
	var tokenProbe *notificationRestartTokenProbe
	return notificationConsumerRestartSpec{
		name: "message_sent", service: "message", stream: jsStreamMessageEvents, filter: jsSubjectMessageEvents,
		deliverSubject: "_INBOX.voice.notification.message", subject: "message.sent", recipientID: recipientID,
		event: &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: messageID, ChatId: chatID, SenderProfileId: senderID.String(),
		}}},
		expected: push.Payload{Data: map[string]string{"type": "new_message"}}, recordDebug: true, safeAssertions: true,
		prepare: func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			server := grpc.NewServer()
			chatv1.RegisterChatServiceServer(server, chatServer)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() {
				server.Stop()
				_ = listener.Close()
			})
			memberLister, err = chatmembers.NewGRPCLister(listener.Addr().String())
			require.NoError(t, err)
		},
		verify: func(t *testing.T) {
			require.EqualValues(t, 1, chatServer.listCalls.Load(), "production Chat member adapter makes one request")
			require.True(t, chatServer.callerMatches.Load(), "production Chat member adapter uses the Notification S2S caller")
			require.True(t, chatServer.requestMatches.Load(), "production Chat member adapter requests the event chat's first member page")
			require.EqualValues(t, 1, tokenProbe.listCalls.Load(), "one target token lookup occurs after sender exclusion")
			require.True(t, tokenProbe.profileMatches.Load(), "the selected member is the non-sender recipient")
		},
		run: func(ctx context.Context, natsURL string, tokens *store.DeviceTokenStore, dispatcher *dispatch.PushDispatcher) error {
			tokenProbe = &notificationRestartTokenProbe{store: tokens, expectedProfile: recipientID}
			pusher := &dispatch.MessagePusher{
				Tokens: tokenProbe, Pusher: dispatcher, Grouping: grouping.NewMemoryStore(),
			}
			return runMessageEventsConsumer(ctx, natsURL, tokens, memberLister, pusher, pushenrich.NoopResolver{}, nil)
		},
	}
}
