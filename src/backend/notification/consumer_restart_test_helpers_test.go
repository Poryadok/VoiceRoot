package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

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
	run            func(context.Context, string, *store.DeviceTokenStore, *dispatch.PushDispatcher) error
}

type notificationConsumerRestartFixture struct {
	serverURL string
	js        nats.JetStreamContext
	tokens    *store.DeviceTokenStore
}

type notificationRestartPush struct {
	profileID uuid.UUID
	payload   push.Payload
}

type notificationRestartFCM struct {
	sent chan notificationRestartPush
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
}

func messageSentRestartSpec() notificationConsumerRestartSpec {
	senderID, recipientID := uuid.New(), uuid.New()
	messageID, chatID := uuid.NewString(), uuid.NewString()
	return notificationConsumerRestartSpec{
		name: "message_sent", service: "message", stream: jsStreamMessageEvents, filter: jsSubjectMessageEvents,
		deliverSubject: "_INBOX.voice.notification.message", subject: "message.sent", recipientID: recipientID,
		event: &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: messageID, ChatId: chatID, SenderProfileId: senderID.String(),
		}}},
		expected: push.Payload{Data: map[string]string{"type": "new_message"}}, recordDebug: true, safeAssertions: true,
		run: func(ctx context.Context, natsURL string, tokens *store.DeviceTokenStore, dispatcher *dispatch.PushDispatcher) error {
			members := stubChatMembers{rows: []chatmembers.Member{
				{ProfileID: senderID.String(), InboxBucket: "main"},
				{ProfileID: recipientID.String(), InboxBucket: "main"},
			}}
			pusher := &dispatch.MessagePusher{
				Tokens: tokens, Pusher: dispatcher, Grouping: grouping.NewMemoryStore(),
			}
			return runMessageEventsConsumer(ctx, natsURL, tokens, members, pusher, pushenrich.NoopResolver{}, nil)
		},
	}
}
