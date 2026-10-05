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
	if !sampleDiagnosticControlAt(t, o, time.Now()) {
		t.Fatal("control sample expired the observer window")
	}
}

func sampleDiagnosticControlAt(t *testing.T, o *composeFcmObserver, sampledAt time.Time) bool {
	t.Helper()
	c, ok := o.readControl()
	o.mu.Lock()
	continueCollecting := o.sampleControlLocked(c, ok, sampledAt)
	o.mu.Unlock()
	return continueCollecting
}

func expireDiagnosticAt(o *composeFcmObserver, now time.Time) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.expireLocked(now)
}

func captureComposeFcmOutput(t *testing.T, emit func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal("diagnostic output capture setup failed")
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()
	emit()
	if err := writer.Close(); err != nil {
		t.Fatal("diagnostic output capture close failed")
	}
	os.Stdout = previous
	output, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal("diagnostic output capture read failed")
	}
	return string(output)
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

func TestComposeFcmObserverAdmissionClassificationIsBoundedAndAnonymous(t *testing.T) {
	newObserver := func() *composeFcmObserver {
		return &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
	}
	terminal := func(o *composeFcmObserver) string {
		return captureComposeFcmOutput(t, func() {
			o.mu.Lock()
			defer o.mu.Unlock()
			o.printUnknownLocked("unknown")
		})
	}
	validControl := composeFcmControl{
		MessageID: diagnosticTestMessage, ChatID: diagnosticTestChat,
		SenderProfileID: diagnosticTestSender, RecipientID: diagnosticTestTarget,
	}
	validBegin := func(o *composeFcmObserver) {
		o.begin("event-admission", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	}

	for _, tc := range []struct {
		name string
		want string
		set  func(*composeFcmObserver)
	}{
		{"none", "admission=none", func(*composeFcmObserver) {}},
		{"invalid", "admission=invalid", func(o *composeFcmObserver) {
			o.controlState = composeFcmControlInvalid
			validBegin(o)
		}},
		{"window", "admission=window", func(o *composeFcmObserver) {
			o.control, o.controlState = validControl, composeFcmControlValid
			o.postControlStarted = time.Now().Add(-composeFcmWindow)
			validBegin(o)
		}},
		{"tuple", "admission=tuple", func(o *composeFcmObserver) {
			o.control, o.controlState = validControl, composeFcmControlValid
			o.control.ChatID = "other-chat"
			validBegin(o)
		}},
		{"identity", "admission=identity", func(o *composeFcmObserver) {
			o.begin("", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
		}},
		{"mixed", "admission=mixed", func(o *composeFcmObserver) {
			o.controlState = composeFcmControlInvalid
			validBegin(o)
			o.begin("", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
		}},
		{"counter saturation", "admission=overflow", func(o *composeFcmObserver) {
			o.controlState = composeFcmControlInvalid
			for i := 0; i <= composeFcmAdmissionCountLimit; i++ {
				validBegin(o)
			}
		}},
		{"candidate capacity", "admission=overflow", func(o *composeFcmObserver) {
			for i := 0; i <= composeFcmCandidateLimit; i++ {
				eventID := string(rune('a' + i))
				o.begin(eventID, diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := newObserver()
			tc.set(o)
			output := terminal(o)
			if !strings.Contains(output, "valid=false") || !strings.Contains(output, tc.want) ||
				!strings.Contains(output, "recipient_present=unknown") || strings.Contains(output, diagnosticTestTarget) ||
				!strings.Contains(output, "member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown") {
				t.Fatalf("admission output did not remain anonymous/fail-closed: %q", output)
			}
		})
	}

	t.Run("closed window records only window admission", func(t *testing.T) {
		o := newObserver()
		o.control, o.controlState = composeFcmControl{
			MessageID: diagnosticTestMessage, ChatID: diagnosticTestChat,
			SenderProfileID: diagnosticTestSender, RecipientID: diagnosticTestTarget,
		}, composeFcmControlValid
		o.postControlStarted = time.Now().Add(-composeFcmWindow)
		o.begin("event-expiry", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
		output := terminal(o)
		if !strings.Contains(output, "admission=window") || !strings.Contains(output, "recipient_present=unknown") {
			t.Fatalf("window expiry did not remain anonymous: %q", output)
		}
	})
}

func TestComposeFcmObserverRejectedAdmissionFailsClosedForLaterTrace(t *testing.T) {
	o := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
	o.control = composeFcmControl{
		ChatID: diagnosticTestChat, SenderProfileID: diagnosticTestSender, RecipientID: diagnosticTestTarget,
	}
	o.controlState = composeFcmControlValid

	if rejected := o.begin("rejected-event", diagnosticTestMessage, "other-chat", diagnosticTestSender); rejected != nil {
		t.Fatal("wrong-tuple event was admitted")
	}
	trace := o.begin("matched-event", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	if trace == nil {
		t.Fatal("matching event after the rejected event was not admitted")
	}
	targetID := uuid.MustParse(diagnosticTestTarget)
	trace.members([]chatmembers.Member{{ProfileID: diagnosticTestTarget, InboxBucket: "main"}}, nil)
	trace.Decision(targetID, true, true, "offline", "ok")
	trace.Tokens(targetID, 1, 1, "ok")
	trace.DispatcherReturned(targetID, "fcm")
	o.finish(trace, "ack")

	output := captureComposeFcmOutput(t, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.printTraceLocked(trace)
	})
	unknownFacts := "member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown"
	if !strings.Contains(output, "valid=false reason=unknown admission=tuple") || !strings.Contains(output, unknownFacts) ||
		strings.Contains(output, diagnosticTestTarget) || strings.Contains(output, "valid=true") {
		t.Fatalf("rejection followed by a matched trace did not fail closed: %q", output)
	}
}

func TestComposeFcmObserverUnreadActualRouteRemainsUnchangedAndUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	senderID, recipientID, chatID, messageID, eventID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	trace := o.begin(eventID.String(), messageID.String(), chatID.String(), senderID.String())
	if trace == nil || !trace.provisional || trace.target != "" {
		t.Fatal("initial-unread event was not buffered")
	}
	forwardedToken := store.DeviceToken{Token: "private-test-token", PushService: "fcm"}
	recorder := &observerCaptureFCM{}
	pusher := &dispatch.MessagePusher{
		Tokens:   messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {forwardedToken}}},
		Pusher:   &dispatch.PushDispatcher{FCM: recorder},
		Grouping: grouping.NewMemoryStore(),
	}
	event := &eventsv1.MessageStreamEvent{EventId: eventID.String(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
		MessageId: messageID.String(), ChatId: chatID.String(), SenderProfileId: senderID.String(),
	}}}
	err := routeMessageNotificationObserved(trace.context(context.Background()), &consumer.MessageEventHandler{Router: delivery.DecideRouting},
		stubChatMembers{ids: []string{senderID.String(), recipientID.String()}}, pusher, pushenrich.NoopResolver{}, event, trace)
	if err != nil || recorder.calls != 1 || recorder.profile != recipientID || recorder.token != forwardedToken || recorder.payload.Data["message_id"] != messageID.String() {
		t.Fatal("provisional observer changed the ordinary route or sender delegation")
	}
	o.finish(trace, "ack")
	writeDiagnosticControlValues(t, path, messageID.String(), chatID.String(), senderID.String(), recipientID.String())
	if !sampleDiagnosticControlAt(t, o, time.Now()) || o.postControlStarted.IsZero() {
		t.Fatal("valid control did not begin the post-route observation window")
	}
	match, ambiguous := o.candidateForMessage(messageID.String())
	if ambiguous || match != trace || match.target != "" || !match.provisional {
		t.Fatal("later control sample retained recipient identity or lost the provisional event")
	}
	output := captureComposeFcmOutput(t, func() {
		if !expireDiagnosticAt(o, o.postControlStarted.Add(composeFcmWindow)) {
			t.Fatal("expired post-control window did not finish the observer")
		}
	})
	if !strings.Contains(output, "valid=false reason=incomplete") || !strings.Contains(output, "recipient_present=unknown") ||
		!strings.Contains(output, "attempts=0 member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown") ||
		strings.Contains(output, recipientID.String()) || strings.Contains(output, messageID.String()) || strings.Contains(output, forwardedToken.Token) {
		t.Fatal("provisional match did not fail closed without identifier output")
	}
	if !o.emitted || len(o.candidates) != 0 || match.memberOK != "unknown" || match.memberRows != 0 || match.present != "unknown" || match.tokenRows != 0 || match.fcmTokens != 0 || match.dispatcherReturns != 0 || match.route != "ack" {
		t.Fatal("provisional callback claimed target-specific outcome facts")
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

func TestComposeFcmObserverUsesSeparatePreAndPostControlDeadlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	if o.begin("event-window", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
		t.Fatal("initial-unread candidate was not buffered")
	}
	preStart := o.preControlStarted
	if expireDiagnosticAt(o, preStart.Add(composeFcmWindow-time.Nanosecond)) {
		t.Fatal("pre-control window expired before its boundary")
	}
	writeDiagnosticControl(t, path, "")
	postStart := preStart.Add(composeFcmWindow - time.Nanosecond)
	if !sampleDiagnosticControlAt(t, o, postStart) || !o.preControlStarted.IsZero() || o.postControlStarted != postStart {
		t.Fatal("valid control did not begin an independent post-control window")
	}
	if expireDiagnosticAt(o, postStart.Add(composeFcmWindow-time.Nanosecond)) {
		t.Fatal("post-control window expired before its boundary")
	}
	output := captureComposeFcmOutput(t, func() {
		expireDiagnosticAt(o, postStart.Add(composeFcmWindow))
	})
	if !strings.Contains(output, "valid=false reason=unknown") || len(o.candidates) != 0 {
		t.Fatal("post-control boundary did not fail closed and clear candidates")
	}
	pre := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
	if pre.begin("event-pre-boundary", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
		t.Fatal("initial-unread boundary candidate was not buffered")
	}
	preOutput := captureComposeFcmOutput(t, func() {
		expireDiagnosticAt(pre, pre.preControlStarted.Add(composeFcmWindow))
	})
	if !strings.Contains(preOutput, "valid=false reason=expired") || len(pre.candidates) != 0 {
		t.Fatal("pre-control boundary did not fail closed and clear candidates")
	}
}

func TestComposeFcmObserverZeroCandidateExpiryEmitsOnePhaseSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	// A callback can enter after subscription bind succeeds but before the bound
	// marker runs. Preserve that observed order rather than inferring the marker.
	o.callbackEntered()
	arm := time.Now()
	if !sampleDiagnosticControlAt(t, o, arm) {
		t.Fatal("valid control did not arm post-control window")
	}
	o.consumerBound()
	o.decodeSucceeded()
	o.routeStarted()
	o.routeReturned()

	output := captureComposeFcmOutput(t, func() {
		if !expireDiagnosticAt(o, arm.Add(composeFcmWindow)) {
			t.Fatal("post-control expiry was not observed")
		}
	})
	if strings.Count(output, "compose_fcm_diag ") != 1 || !strings.Contains(output, "valid=false reason=unknown admission=none candidates=0") {
		t.Fatalf("zero-candidate expiry did not emit exactly one unknown record: %q", output)
	}
	if !strings.Contains(output, "member_result=unknown member_count=0 recipient_present=unknown inbox=unknown base_push=unknown final_push=unknown presence=unknown policy=unknown token_rows=0 fcm_tokens=0 dispatcher_returns=0 route=unknown") {
		t.Fatalf("zero-candidate expiry exposed target facts: %q", output)
	}
	if !strings.Contains(output, "phase_evidence=known consumer_bound_before_expiry=false callback_entered_before_expiry=true decode_succeeded_before_expiry=false route_started_before_expiry=false route_returned_before_expiry=false consumer_bound_at_expiry=true callback_entered_at_expiry=true decode_succeeded_at_expiry=true route_started_at_expiry=true route_returned_at_expiry=true") {
		t.Fatalf("phase snapshots did not preserve observed ordering: %q", output)
	}
	if strings.Contains(output, diagnosticTestChat) || strings.Contains(output, diagnosticTestSender) || strings.Contains(output, diagnosticTestTarget) {
		t.Fatalf("phase record exposed identities: %q", output)
	}
}

func TestComposeFcmObserverLifecycleSamplesAreIndependentAndOnceOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	arm := time.Now()
	output := captureComposeFcmOutput(t, func() {
		if !sampleDiagnosticControlAt(t, o, arm) {
			t.Fatal("initial invalid sample stopped collection")
		}
		if !sampleDiagnosticControlAt(t, o, arm.Add(500*time.Microsecond)) {
			t.Fatal("repeated invalid sample stopped collection")
		}
		writeDiagnosticControl(t, path, "")
		if !sampleDiagnosticControlAt(t, o, arm.Add(time.Millisecond)) {
			t.Fatal("later valid sample did not arm collection")
		}
		if !sampleDiagnosticControlAt(t, o, arm.Add(2*time.Millisecond)) {
			t.Fatal("repeat valid sample stopped collection")
		}
		if !expireDiagnosticAt(o, arm.Add(time.Millisecond+composeFcmWindow)) {
			t.Fatal("existing post-control expiry was not observed")
		}
	})
	for _, marker := range []string{
		"compose_fcm_lifecycle control_sample=invalid",
		"compose_fcm_lifecycle control_sample=valid",
		"compose_fcm_lifecycle window_expiry=completed",
	} {
		if strings.Count(output, marker) != 1 {
			t.Fatalf("lifecycle marker was not emitted exactly once: %q", output)
		}
	}
}

func TestComposeFcmObserverPartialPhaseSnapshotsFailClosed(t *testing.T) {
	t.Run("missing arm snapshot", func(t *testing.T) {
		o := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace), controlState: composeFcmControlValid, postControlStarted: time.Now()}
		o.callbackEntered()
		output := captureComposeFcmOutput(t, func() {
			expireDiagnosticAt(o, o.postControlStarted.Add(composeFcmWindow))
		})
		if !strings.Contains(output, "phase_evidence=unknown consumer_bound_before_expiry=unknown callback_entered_before_expiry=unknown decode_succeeded_before_expiry=unknown route_started_before_expiry=unknown route_returned_before_expiry=unknown consumer_bound_at_expiry=unknown callback_entered_at_expiry=unknown decode_succeeded_at_expiry=unknown route_started_at_expiry=unknown route_returned_at_expiry=unknown") {
			t.Fatalf("partial phase snapshot fabricated a value: %q", output)
		}
	})

	t.Run("no post-control window", func(t *testing.T) {
		o := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
		o.callbackEntered()
		output := captureComposeFcmOutput(t, func() {
			o.mu.Lock()
			o.printUnknownLocked("unknown")
			o.mu.Unlock()
		})
		if !strings.Contains(output, "phase_evidence=unknown") || strings.Contains(output, "callback_entered_before_expiry=true") {
			t.Fatalf("missing window did not leave phase facts unknown: %q", output)
		}
	})
}

func TestComposeFcmObserverPhaseMethodsAreNilSafeAndMonotonic(t *testing.T) {
	var absent *composeFcmObserver
	absent.consumerBound()
	absent.callbackEntered()
	absent.decodeSucceeded()
	absent.routeStarted()
	absent.routeReturned()

	o := &composeFcmObserver{}
	o.callbackEntered()
	o.callbackEntered()
	o.consumerBound()
	o.decodeSucceeded()
	o.routeStarted()
	o.routeReturned()
	if !o.phases.consumerBound || !o.phases.callbackEntered || !o.phases.decodeSucceeded || !o.phases.routeStarted || !o.phases.routeReturned {
		t.Fatal("repeated phase observations were not retained monotonically")
	}
}

func TestComposeFcmObserverPhaseSnapshotsDecodeFailureAndRouteError(t *testing.T) {
	t.Run("decode failure stops before route", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "control.json")
		writeDiagnosticControl(t, path, "")
		o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
		o.callbackEntered()
		arm := time.Now()
		if !sampleDiagnosticControlAt(t, o, arm) {
			t.Fatal("valid control did not arm post-control window")
		}
		output := captureComposeFcmOutput(t, func() {
			expireDiagnosticAt(o, arm.Add(composeFcmWindow))
		})
		if !strings.Contains(output, "phase_evidence=known consumer_bound_before_expiry=false callback_entered_before_expiry=true decode_succeeded_before_expiry=false route_started_before_expiry=false route_returned_before_expiry=false consumer_bound_at_expiry=false callback_entered_at_expiry=true decode_succeeded_at_expiry=false route_started_at_expiry=false route_returned_at_expiry=false") {
			t.Fatalf("decode failure crossed an unobserved route phase: %q", output)
		}
	})

	t.Run("route error still records return without error detail", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "control.json")
		messageID, chatID, senderID, recipientID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		writeDiagnosticControlValues(t, path, messageID.String(), chatID.String(), senderID.String(), recipientID.String())
		o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
		if !sampleDiagnosticControlAt(t, o, time.Now()) {
			t.Fatal("valid control did not arm post-control window")
		}
		trace := o.begin("event-route-error", messageID.String(), chatID.String(), senderID.String())
		if trace == nil {
			t.Fatal("correlated event was not recorded")
		}
		recorder := &observerCaptureFCM{err: errors.New("private route failure")}
		pusher := &dispatch.MessagePusher{
			Tokens:   messageTokenRepo{byProfile: map[uuid.UUID][]store.DeviceToken{recipientID: {{Token: "private-test-token", PushService: "fcm"}}}},
			Pusher:   &dispatch.PushDispatcher{FCM: recorder},
			Grouping: grouping.NewMemoryStore(),
		}
		event := &eventsv1.MessageStreamEvent{EventId: "event-route-error", Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: messageID.String(), ChatId: chatID.String(), SenderProfileId: senderID.String(),
		}}}
		o.callbackEntered()
		o.decodeSucceeded()
		o.routeStarted()
		members := stubChatMembers{rows: []chatmembers.Member{{ProfileID: senderID.String(), InboxBucket: "main"}, {ProfileID: recipientID.String(), InboxBucket: "main"}}}
		routeErr := routeMessageNotificationObserved(trace.context(context.Background()), &consumer.MessageEventHandler{Router: delivery.DecideRouting}, members, pusher, pushenrich.NoopResolver{}, event, trace)
		o.routeReturned()
		if routeErr == nil || recorder.calls != 1 {
			t.Fatal("route error did not preserve the real sender attempt")
		}
		o.finish(trace, "nak")
		output := captureComposeFcmOutput(t, func() {
			expireDiagnosticAt(o, o.postControlStarted.Add(composeFcmWindow))
		})
		if !strings.Contains(output, "valid=true reason=matched") || !strings.Contains(output, "route=nak phase_evidence=known") ||
			!strings.Contains(output, "route_returned_at_expiry=true") || strings.Contains(output, "private route failure") {
			t.Fatalf("route return phase leaked details or changed the result: %q", output)
		}
	})
}

func TestComposeFcmObserverRejectsConflictingExactPairAndCrossTupleCollision(t *testing.T) {
	t.Run("same-pair-conflicting-tuple", func(t *testing.T) {
		o := &composeFcmObserver{candidates: make(map[string]*composeFcmTrace)}
		trace := o.begin("same-event", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
		if trace == nil || o.begin("same-event", diagnosticTestMessage, "other-chat", "other-sender") != nil || !o.ambiguous {
			t.Fatal("same event pair with contradictory tuple was not rejected as ambiguous")
		}
	})
	t.Run("same-pair-conflicts-after-valid-control", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "control.json")
		writeDiagnosticControl(t, path, "")
		o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
		cacheDiagnosticControl(t, o)
		if o.begin("same-event", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
			t.Fatal("candidate with the control tuple was not retained")
		}
		if o.begin("same-event", diagnosticTestMessage, "other-chat", "other-sender") != nil || !o.ambiguous {
			t.Fatal("valid control tuple guard hid an exact-pair collision")
		}
	})
	t.Run("different-event-cross-tuple", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "control.json")
		o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
		if o.begin("event-one", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil ||
			o.begin("event-two", diagnosticTestMessage, "other-chat", "other-sender") == nil {
			t.Fatal("initial-unread collision candidates were not retained within bounds")
		}
		writeDiagnosticControl(t, path, diagnosticTestMessage)
		cacheDiagnosticControl(t, o)
		match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
		if match != nil || !ambiguous {
			t.Fatal("different EventIds for one message were not ambiguous across tuples")
		}
	})
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
	err         error
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
	return r.err
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
