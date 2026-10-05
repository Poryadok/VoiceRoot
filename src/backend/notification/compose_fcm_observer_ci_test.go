//go:build voice_compose_fcm_diagnostic

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/fcm"
	"voice/backend/notification/internal/grouping"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/notification/internal/store"
)

const (
	diagnosticTestMessage = "11111111-1111-4111-8111-111111111111"
	diagnosticTestChat    = "22222222-2222-4222-8222-222222222222"
	diagnosticTestSender  = "33333333-3333-4333-8333-333333333333"
	diagnosticTestTarget  = "44444444-4444-4444-8444-444444444444"
)

func writeDiagnosticControl(t *testing.T, path, messageID string) {
	writeDiagnosticControlValues(t, path, messageID, diagnosticTestChat, diagnosticTestSender, diagnosticTestTarget)
}

func writeDiagnosticControlValues(t *testing.T, path, messageID, chatID, senderID, recipientID string) {
	t.Helper()
	data, err := json.Marshal(composeFcmControl{MessageID: messageID, ChatID: chatID, SenderProfileID: senderID, RecipientID: recipientID})
	if err != nil {
		t.Fatal("control encoding failed")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal("control write failed")
	}
}

func cacheDiagnosticControl(t *testing.T, o *composeFcmObserver) {
	t.Helper()
	c, ok := o.readControl()
	o.control = c
	if ok {
		o.controlState = composeFcmControlValid
	} else {
		o.controlState = composeFcmControlInvalid
	}
}

func TestComposeFcmObserverBuffersOnlyBeforeFirstControlSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	if trace := o.begin("event-before-sample", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); trace == nil {
		t.Fatal("event before the first control sample was not buffered")
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal("invalid control fixture write failed")
	}
	cacheDiagnosticControl(t, o)
	if o.controlState != composeFcmControlInvalid || len(o.candidates) != 0 {
		t.Fatal("sampled-invalid control did not clear provisional candidates")
	}
	if trace := o.begin("event-while-invalid", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); trace != nil {
		t.Fatal("event was buffered while control was sampled-invalid")
	}
	writeDiagnosticControl(t, path, "")
	cacheDiagnosticControl(t, o)
	trace := o.begin("event-after-valid", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	if trace == nil {
		t.Fatal("event after valid control sample was not captured")
	}
	writeDiagnosticControl(t, path, diagnosticTestMessage)
	match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
	if ambiguous || match != trace || match.eventID != "event-after-valid" {
		t.Fatal("only the post-valid unique candidate was not attributed")
	}
}

func TestComposeFcmObserverBoundsUntrustedCorrelationKeys(t *testing.T) {
	o := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
	if o.begin(strings.Repeat("e", composeFcmKeyByteLimit+1), diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) != nil ||
		o.begin("event-large-message", strings.Repeat("m", composeFcmKeyByteLimit+1), diagnosticTestChat, diagnosticTestSender) != nil ||
		o.begin("event-large-chat", diagnosticTestMessage, strings.Repeat("c", composeFcmKeyByteLimit+1), diagnosticTestSender) != nil ||
		len(o.candidates) != 0 {
		t.Fatal("oversized correlation key was retained")
	}
}

func TestComposeFcmObserverUnreadCandidatesRetainOutcomeAndExpireClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	trace := o.begin("event-unread", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	if trace == nil {
		t.Fatal("initial-unread event was not buffered")
	}
	if o.begin("event-decoy", "55555555-5555-4555-8555-555555555555", "other-chat", "other-sender") == nil {
		t.Fatal("bounded pre-control decoy was not retained for fail-closed disambiguation")
	}
	trace.memberOK, trace.memberRows, trace.present, trace.route = "ok", 2, "true", "ack"
	writeDiagnosticControl(t, path, diagnosticTestMessage)
	cacheDiagnosticControl(t, o)
	match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
	if ambiguous || match != trace || match.memberOK != "ok" || match.memberRows != 2 || match.present != "true" || match.route != "ack" {
		t.Fatal("buffered trace facts did not survive later control attribution")
	}
	o.started = time.Now().Add(-composeFcmWindow)
	if o.begin("event-expired", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) != nil {
		t.Fatal("expired attribution window accepted another candidate")
	}
}

func TestComposeFcmObserverRejectsUniqueCandidateForDifferentControlTuple(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	if o.begin("event-other-tuple", diagnosticTestMessage, "other-chat", "other-sender") == nil {
		t.Fatal("initial-unread candidate was not buffered")
	}
	writeDiagnosticControl(t, path, diagnosticTestMessage)
	cacheDiagnosticControl(t, o)
	match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
	if match != nil || ambiguous {
		t.Fatal("candidate with a different response chat/sender tuple was attributed")
	}
}

func TestComposeFcmObserverCorrelatesBothResponseOrdersAndEventRetries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	cacheDiagnosticControl(t, o)
	first := o.begin("event-a", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	if first == nil {
		t.Fatal("pre-response event was not recorded")
	}
	if retry := o.begin("event-a", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); retry != first || first.attempts != 2 {
		t.Fatal("same message and event pair was not classified as retry")
	}
	if distinct := o.begin("event-b", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); distinct == nil || distinct == first {
		t.Fatal("distinct event identity was not retained separately")
	}
	writeDiagnosticControl(t, path, diagnosticTestMessage)
	o.mu.Lock()
	match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
	o.mu.Unlock()
	if match != nil || !ambiguous {
		t.Fatal("different event identities for the response message were not ambiguous")
	}

	writeDiagnosticControl(t, path, diagnosticTestMessage)
	responseFirst := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	cacheDiagnosticControl(t, responseFirst)
	if responseFirst.begin("event-c", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
		t.Fatal("response-before-consumer event was not recorded")
	}
	responseFirst.mu.Lock()
	match, ambiguous = responseFirst.candidateForMessage(diagnosticTestMessage)
	responseFirst.mu.Unlock()
	if match == nil || ambiguous || match.eventID != "event-c" {
		t.Fatal("response-before-consumer event did not correlate uniquely")
	}
}

func TestComposeFcmObserverCapacityFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	cacheDiagnosticControl(t, o)
	for i := 0; i < composeFcmCandidateLimit; i++ {
		messageID := diagnosticTestMessage[:35] + string(rune('a'+i))
		if o.begin("event-"+string(rune('a'+i)), messageID, diagnosticTestChat, diagnosticTestSender) == nil {
			t.Fatal("candidate rejected before capacity")
		}
	}
	if o.begin("overflow", "55555555-5555-4555-8555-555555555555", diagnosticTestChat, diagnosticTestSender) != nil || !o.overflow {
		t.Fatal("capacity exhaustion was not recorded as fail-closed overflow")
	}
}

func TestComposeFcmObserverBeginUsesCachedControlWithoutFilesystemIO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	cacheDiagnosticControl(t, o)
	if err := os.Remove(path); err != nil {
		t.Fatal("control removal failed")
	}
	if o.begin("event-cached", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
		t.Fatal("handler path performed a filesystem-dependent control read")
	}
}

func TestComposeFcmObserverRejectsMalformedAndSymlinkControl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "control.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal("malformed control setup failed")
	}
	o := &composeFcmObserver{path: path}
	if _, ok := o.readControl(); ok {
		t.Fatal("malformed control was accepted")
	}
	target := filepath.Join(dir, "target.json")
	writeDiagnosticControl(t, target, "")
	if err := os.Remove(path); err != nil {
		t.Fatal("control cleanup failed")
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal("symlink setup failed")
	}
	if _, ok := o.readControl(); ok {
		t.Fatal("symlink control was accepted")
	}
}

func TestComposeFcmObserverReadControlFailsClosedForUnavailableInputs(t *testing.T) {
	dir := t.TempDir()
	t.Run("absent", func(t *testing.T) {
		o := &composeFcmObserver{path: filepath.Join(dir, "absent.json")}
		if _, ok := o.readControl(); ok {
			t.Fatal("absent control was accepted")
		}
	})
	t.Run("oversized", func(t *testing.T) {
		path := filepath.Join(dir, "oversized.json")
		if err := os.WriteFile(path, make([]byte, 4097), 0o600); err != nil {
			t.Fatal("oversized control setup failed")
		}
		o := &composeFcmObserver{path: path}
		if _, ok := o.readControl(); ok {
			t.Fatal("oversized control was accepted")
		}
	})
	t.Run("unreadable", func(t *testing.T) {
		path := filepath.Join(dir, "unreadable.json")
		if err := os.WriteFile(path, []byte(`{"message_id":""}`), 0o600); err != nil {
			t.Fatal("unreadable control setup failed")
		}
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal("unreadable control permission setup failed")
		}
		if _, ok := (&composeFcmObserver{path: path}).readControl(); ok {
			t.Fatal("unreadable control was accepted")
		}
		// Root can bypass file mode bits, so use Linux's regular, owner-only
		// proc mem file to exercise the ReadFile error path in that case.
		if os.Geteuid() == 0 {
			procMem := "/proc/self/mem"
			info, err := os.Lstat(procMem)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
				t.Fatal("unreadable control fixture unavailable")
			}
			if _, ok := (&composeFcmObserver{path: procMem}).readControl(); ok {
				t.Fatal("unreadable proc control was accepted")
			}
		}
	})
}

type observerContextKey struct{}

type observerCaptureFCM struct {
	calls       int
	profile     uuid.UUID
	token       store.DeviceToken
	payload     fcm.PushPayload
	contextMark any
}

type observerRetryMembers struct {
	calls  atomic.Int32
	sender string
	target string
}

func (m *observerRetryMembers) ListMemberProfileIDs(context.Context, string) ([]string, error) {
	return []string{m.sender, m.target}, nil
}

func (m *observerRetryMembers) ListMembers(context.Context, string) ([]chatmembers.Member, error) {
	if m.calls.Add(1) == 1 {
		return nil, errors.New("private route retry fixture")
	}
	return []chatmembers.Member{{ProfileID: m.sender, InboxBucket: "main"}, {ProfileID: m.target, InboxBucket: "main"}}, nil
}

type observerAckFCM struct{ calls atomic.Int32 }

func (s *observerAckFCM) Send(context.Context, uuid.UUID, store.DeviceToken, fcm.PushPayload) error {
	s.calls.Add(1)
	return nil
}

func (r *observerCaptureFCM) Send(ctx context.Context, profile uuid.UUID, token store.DeviceToken, payload fcm.PushPayload) error {
	r.calls++
	r.profile, r.token, r.payload = profile, token, payload
	r.contextMark = ctx.Value(observerContextKey{})
	return nil
}

func TestComposeFcmObserverPreservesRouteSenderArgumentsAndArchivedSuppression(t *testing.T) {
	senderID, recipientID, chatID, messageID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControlValues(t, path, messageID.String(), chatID.String(), senderID.String(), recipientID.String())
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	cacheDiagnosticControl(t, o)
	trace := o.begin("event-route", messageID.String(), chatID.String(), senderID.String())
	if trace == nil {
		t.Fatal("correlated event was not recorded")
	}
	marker := &struct{}{}
	ctx := trace.context(context.WithValue(context.Background(), observerContextKey{}, marker))
	forwardedToken := store.DeviceToken{Token: "private-test-token", PushService: "fcm"}
	recorder := &observerCaptureFCM{}
	pusher := &dispatch.MessagePusher{
		Tokens:   messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {forwardedToken}}},
		Pusher:   &dispatch.PushDispatcher{FCM: recorder},
		Grouping: grouping.NewMemoryStore(),
	}
	event := &eventsv1.MessageStreamEvent{Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
		MessageId: messageID.String(), ChatId: chatID.String(), SenderProfileId: senderID.String(),
	}}}
	err := routeMessageNotificationObserved(ctx, &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, pusher, pushenrich.NoopResolver{}, event, trace)
	if err != nil {
		t.Fatal("observed route changed the ordinary route result")
	}
	if recorder.calls != 1 || recorder.profile != recipientID || recorder.token != forwardedToken || recorder.contextMark != marker || recorder.payload.Data["message_id"] != messageID.String() {
		t.Fatal("observed route changed sender call count or forwarded arguments")
	}
	o.finish(trace, "ack")
	if trace.route != "ack" || trace.dispatcherReturns != 1 {
		t.Fatal("successful route outcome was not preserved")
	}

	archivedTrace := o.begin("event-archived", messageID.String(), chatID.String(), senderID.String())
	archivedMembers := stubChatMembers{rows: []chatmembers.Member{
		{ProfileID: senderID.String(), InboxBucket: "main"},
		{ProfileID: recipientID.String(), InboxBucket: "main", IsArchived: true},
	}}
	if err := routeMessageNotificationObserved(archivedTrace.context(context.Background()), &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		archivedMembers, pusher, pushenrich.NoopResolver{}, event, archivedTrace); err != nil || archivedTrace.finalPush != "false" || recorder.calls != 1 {
		t.Fatal("archived recipient suppression was not reflected in the diagnostic")
	}

	failedTrace := o.begin("event-failed", messageID.String(), chatID.String(), senderID.String())
	failed := routeMessageNotificationObserved(ctx, &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{}, pusher, pushenrich.NoopResolver{}, event, failedTrace)
	if failed == nil {
		t.Fatal("invalid ordinary route unexpectedly succeeded")
	}
	o.finish(failedTrace, "nak")
	if failedTrace.route != "nak" || recorder.calls != 1 {
		t.Fatal("failed route outcome or sender count changed")
	}
}

func TestComposeFcmObserverActualConsumerNakRetryThenAckWithoutResponseWait(t *testing.T) {
	server, err := natsserver.NewServer(&natsserver.Options{JetStream: true, StoreDir: t.TempDir(), Port: -1})
	if err != nil {
		t.Fatal("private JetStream setup failed")
	}
	go server.Start()
	t.Cleanup(server.Shutdown)
	if !server.ReadyForConnections(10 * time.Second) {
		t.Fatal("private JetStream was not ready")
	}
	provisioner, err := nats.Connect(server.ClientURL())
	if err != nil {
		t.Fatal("private JetStream connection failed")
	}
	t.Cleanup(provisioner.Close)
	js, err := provisioner.JetStream()
	if err != nil {
		t.Fatal("private JetStream API failed")
	}
	if _, err = js.AddStream(&nats.StreamConfig{Name: jsStreamMessageEvents, Subjects: []string{jsSubjectMessageEvents}}); err != nil {
		t.Fatal("private message stream setup failed")
	}
	durable := consumer.SharedDurable("message")
	if _, err = js.AddConsumer(jsStreamMessageEvents, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: jsSubjectMessageEvents, DeliverSubject: "_INBOX.voice.notification.message",
		AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy, AckWait: 100 * time.Millisecond,
	}); err != nil {
		t.Fatal("private message durable setup failed")
	}

	senderID, targetID, chatID, messageID, eventID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	controlPath := filepath.Join(t.TempDir(), "correlation.json")
	writeDiagnosticControlValues(t, controlPath, "", chatID.String(), senderID.String(), targetID.String())
	t.Setenv("VOICE_FCM_DIAGNOSTIC_FILE", controlPath)
	members := &observerRetryMembers{sender: senderID.String(), target: targetID.String()}
	fcmRecorder := &observerAckFCM{}
	pusher := &dispatch.MessagePusher{
		Tokens: messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{targetID: {{Token: "private-test-token", PushService: "fcm"}}}},
		Pusher: &dispatch.PushDispatcher{FCM: fcmRecorder}, Grouping: grouping.NewMemoryStore(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	readiness := newNotificationConsumerReadiness("message")
	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- runMessageEventsConsumer(withNotificationConsumerReadiness(ctx, readiness, "message"), server.ClientURL(), &store.DeviceTokenStore{},
			members, pusher, pushenrich.NoopResolver{}, nil)
	}()
	readyDeadline := time.Now().Add(5 * time.Second)
	for !readiness.ready() && time.Now().Before(readyDeadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !readiness.ready() {
		t.Fatal("private message consumer did not bind")
	}
	// Give the independent collector a chance to cache the pre-send tuple; the
	// event path never waits for the file or the response message ID.
	time.Sleep(50 * time.Millisecond)

	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal("private scalar-output capture setup failed")
	}
	previousStdout := os.Stdout
	os.Stdout = stdoutWriter
	defer func() {
		os.Stdout = previousStdout
		_ = stdoutWriter.Close()
		_ = stdout.Close()
	}()
	event := &eventsv1.MessageStreamEvent{EventId: eventID.String(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
		MessageId: messageID.String(), ChatId: chatID.String(), SenderProfileId: senderID.String(),
	}}}
	encoded, err := proto.Marshal(event)
	if err != nil {
		t.Fatal("private message event encoding failed")
	}
	publishAck, err := js.Publish(jsSubjectMessageEvents[:len(jsSubjectMessageEvents)-1]+"sent", encoded)
	if err != nil {
		t.Fatal("private message event publish failed")
	}
	deadline := time.Now().Add(4 * time.Second)
	acked := false
	infoAvailable := false
	infoErrors := 0
	var pending, ackFloor, deliveredConsumer, deliveredStream uint64
	var ackPending, redeliveries int
	for time.Now().Before(deadline) {
		info, infoErr := js.ConsumerInfo(jsStreamMessageEvents, durable)
		if infoErr != nil {
			infoErrors = boundedCount(infoErrors + 1)
		} else {
			infoAvailable = true
			pending, ackPending, ackFloor = info.NumPending, info.NumAckPending, info.AckFloor.Stream
			deliveredConsumer, deliveredStream = info.Delivered.Consumer, info.Delivered.Stream
			redeliveries = info.NumRedelivered
			if pending == 0 && ackPending == 0 && ackFloor >= publishAck.Sequence &&
				deliveredConsumer >= 2 && deliveredStream >= publishAck.Sequence {
				acked = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !acked || members.calls.Load() < 2 || fcmRecorder.calls.Load() != 1 {
		t.Fatalf("handler retry or final durable ACK changed acked=%t member_calls=%d fcm_calls=%d consumer_info_available=%t info_errors=%d pending=%d ack_pending=%d ack_floor_reached=%t delivered_consumer=%d delivered_stream=%d redeliveries=%d",
			acked, boundedCount(int(members.calls.Load())), boundedCount(int(fcmRecorder.calls.Load())), infoAvailable, infoErrors, pending, ackPending, ackFloor >= publishAck.Sequence, deliveredConsumer, deliveredStream, redeliveries)
	}
	// Simulate the Flutter HTTP response arriving only after the consumer has
	// already completed its route and durable ACK.
	writeDiagnosticControlValues(t, controlPath, messageID.String(), chatID.String(), senderID.String(), targetID.String())
	time.Sleep(composeFcmWindow + 100*time.Millisecond)
	if err := stdoutWriter.Close(); err != nil {
		t.Fatal("private scalar-output capture close failed")
	}
	os.Stdout = previousStdout
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal("private scalar-output read failed")
	}
	if !strings.Contains(string(output), "reason=matched") || !strings.Contains(string(output), "attempts=2") || !strings.Contains(string(output), "dispatcher_returns=1 route=ack") {
		t.Fatal("actual consumer event did not correlate through retry to ACK")
	}
	cancel()
	select {
	case <-consumerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("private message consumer did not stop")
	}
}
