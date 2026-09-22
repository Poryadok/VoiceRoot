package main

import (
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestProfileFanoutDisconnectsOnOverflow_AllLifecycleOps(t *testing.T) {
	t.Parallel()
	for _, op := range []string{
		"call_incoming", "call_accepted", "call_declined", "call_missed",
		"call_ended", "call_started", "screen_share_started", "screen_share_stopped",
		"dm_peer_deleted",
	} {
		if !profileFanoutDisconnectsOnOverflow(op) {
			t.Fatalf("%s must close an overflowing lifecycle recipient", op)
		}
	}
	for _, op := range []string{"voice_state_update", "typing", "presence_update"} {
		if profileFanoutDisconnectsOnOverflow(op) {
			t.Fatalf("%s must remain lossy", op)
		}
	}
}

func TestEventConsumersAckMalformedPayloadAfterAttempt(t *testing.T) {
	t.Parallel()
	for _, consume := range []struct {
		name string
		fn   func(*nats.Msg, *wsHub, func(*nats.Msg) error)
	}{
		{
			name: "voice",
			fn: func(msg *nats.Msg, hub *wsHub, ack func(*nats.Msg) error) {
				consumeVoiceEventMessage(msg, hub, nil, ack)
			},
		},
		{
			name: "message",
			fn: func(msg *nats.Msg, hub *wsHub, ack func(*nats.Msg) error) {
				consumeMessageEventMessage(msg, hub, nil, ack)
			},
		},
	} {
		t.Run(consume.name, func(t *testing.T) {
			acks := 0
			consume.fn(&nats.Msg{Data: []byte("not a protobuf")}, newWSHub(), func(*nats.Msg) error {
				acks++
				return nil
			})
			if acks != 1 {
				t.Fatalf("acks=%d want 1", acks)
			}
		})
	}
}

func TestVoiceConsumerAcksAfterHealthyFanoutAttempt(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	profileID := "profile"
	reg := hub.attachConn("i", "healthy", profileID, 1)
	payload, err := proto.Marshal(&eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{
				RoomId:          "room",
				CalleeProfileId: profileID,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ackAfterDelivery := false
	consumeVoiceEventMessage(&nats.Msg{Data: payload}, hub, nil, func(*nats.Msg) error {
		select {
		case got := <-reg.fanout:
			if got.Op != "call_incoming" {
				t.Fatalf("op=%q", got.Op)
			}
			ackAfterDelivery = true
		default:
			t.Fatal("ack happened before local fanout attempt")
		}
		return nil
	})
	if !ackAfterDelivery {
		t.Fatal("ack was not attempted")
	}
}

func TestMessageConsumerAcksAfterHealthyFanoutAttempt(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	chatID := "11111111-1111-4111-8111-111111111111"
	reg := hub.attachConn("i", "healthy", "profile", 1)
	hub.addChat(reg, chatID)
	payload, err := proto.Marshal(&eventsv1.MessageStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{ChatId: chatID, MessageId: "message", SenderProfileId: "sender"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	acks := 0
	consumeMessageEventMessage(&nats.Msg{Data: payload}, hub, nil, func(*nats.Msg) error {
		acks++
		select {
		case got := <-reg.fanout:
			if got.Op != "message_create" {
				t.Fatalf("op=%q want message_create", got.Op)
			}
		default:
			t.Fatal("ack happened before message local fanout attempt")
		}
		return nil
	})
	if acks != 1 {
		t.Fatalf("acks=%d want 1", acks)
	}
}

func TestVoiceConsumerAcksAfterSlowOverflowAndHealthyDelivery(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	profileID := "profile"
	slow := hub.attachConn("i", "slow", profileID, 1)
	healthy := hub.attachConn("i", "healthy", profileID, 1)
	slow.fanout <- fanoutEnvelope{Op: "older"}
	payload, err := proto.Marshal(&eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{RoomId: "room", CalleeProfileId: profileID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	acks := 0
	consumeVoiceEventMessage(&nats.Msg{Data: payload}, hub, nil, func(*nats.Msg) error {
		acks++
		select {
		case <-slow.overflow:
		default:
			t.Fatal("ack happened before slow recipient overflow attempt")
		}
		select {
		case got := <-healthy.fanout:
			if got.Op != "call_incoming" {
				t.Fatalf("healthy op=%q want call_incoming", got.Op)
			}
		default:
			t.Fatal("ack happened before healthy recipient fanout attempt")
		}
		return nil
	})
	if acks != 1 {
		t.Fatalf("acks=%d want 1", acks)
	}
}

func TestProfileFanoutOverflowOnlySignalsSlowRecipient(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	slow := hub.attachConn("i", "slow", "profile", 1)
	healthy := hub.attachConn("i", "healthy", "profile", 1)
	slow.fanout <- fanoutEnvelope{Op: "older"}
	env := fanoutEnvelope{Op: "call_started", D: json.RawMessage(`{}`)}
	started := time.Now()
	hub.broadcastToProfile("profile", env, nil, "")
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("fanout waited for full recipient: %v", elapsed)
	}
	select {
	case <-slow.overflow:
	case <-time.After(time.Second):
		t.Fatal("slow recipient did not receive overflow signal")
	}
	select {
	case got := <-healthy.fanout:
		if got.Op != env.Op {
			t.Fatalf("healthy op=%q want=%q", got.Op, env.Op)
		}
	case <-time.After(time.Second):
		t.Fatal("healthy recipient did not receive lifecycle fanout")
	}
	select {
	case <-slow.overflow:
		// A closed signal remains readable; enqueue must not create a second close.
	default:
		t.Fatal("overflow signal was not retained")
	}
}

func TestProfileFanoutIsZeroWaitAndHealthyRecipientKeepsFIFO(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	slow := hub.attachConn("i", "slow", "profile", 1)
	healthy := hub.attachConn("i", "healthy", "profile", 2)
	slow.fanout <- fanoutEnvelope{Op: "older"}
	first := fanoutEnvelope{Op: "call_started", D: json.RawMessage(`{"sequence":1}`)}
	second := fanoutEnvelope{Op: "call_ended", D: json.RawMessage(`{"sequence":2}`)}
	completed := make(chan struct{})
	go func() {
		hub.broadcastToProfile("profile", first, nil, "")
		hub.broadcastToProfile("profile", second, nil, "")
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fanout goroutine exceeded zero-wait deadline")
	}
	for i, want := range []fanoutEnvelope{first, second} {
		select {
		case got := <-healthy.fanout:
			if got.Op != want.Op || string(got.D) != string(want.D) {
				t.Fatalf("healthy frame %d = %+v want %+v", i, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("healthy frame %d missing", i)
		}
	}
}

func TestProfileFanoutRecordsOnlyBoundedMetricOutcomes(t *testing.T) {
	reg := prometheus.NewRegistry()
	previous := rtMetrics
	t.Cleanup(func() { rtMetrics = previous })
	metrics := initRealtimeMetrics(reg)
	hub := newWSHub()
	enqueued := hub.attachConn("i", "enqueued", "profile", 2)
	dropped := hub.attachConn("i", "dropped", "profile", 1)
	overflow := hub.attachConn("i", "overflow", "profile", 1)
	dropped.fanout <- fanoutEnvelope{Op: "old"}
	overflow.fanout <- fanoutEnvelope{Op: "old"}
	hub.broadcastToProfile("profile", fanoutEnvelope{Op: "voice_state_update"}, nil, "")
	hub.broadcastToProfile("profile", fanoutEnvelope{Op: "call_started"}, nil, "")
	if got := testutil.ToFloat64(metrics.fanoutEnqueueTotal.WithLabelValues("enqueued")); got != 2 {
		t.Fatalf("enqueued=%v want 2", got)
	}
	if got := testutil.ToFloat64(metrics.fanoutEnqueueTotal.WithLabelValues("dropped")); got != 2 {
		t.Fatalf("dropped=%v want 2", got)
	}
	if got := testutil.ToFloat64(metrics.fanoutEnqueueTotal.WithLabelValues("disconnect_on_overflow")); got != 2 {
		t.Fatalf("disconnect_on_overflow=%v want 2", got)
	}
	if count := testutil.CollectAndCount(metrics.fanoutEnqueueTotal); count != 3 {
		t.Fatalf("outcome label count=%d want 3", count)
	}
	_ = enqueued
}

func TestWSClosesLifecycleOverflowWith1013ExactlyOnce(t *testing.T) {
	hub := permitAllTestSubscriptions(newWSHub())
	srv := httptest.NewServer(newServiceHandler(serviceName, staticTokenValidator{
		"token": {UserID: "account", ProfileID: "profile"},
	}, nil, hub, nil, "test-instance", readinessDeps{}))
	t.Cleanup(srv.Close)
	headers := wsUpgradeHeaders("token")
	headers.Set("X-Profile-Id", "profile")
	conn, _, err := websocket.DefaultDialer.Dial(wsEndpoint(t, srv), headers)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var hello wsEnvelope
	if err := conn.ReadJSON(&hello); err != nil || hello.Op != "hello" {
		t.Fatalf("hello=%+v err=%v", hello, err)
	}

	var target *connReg
	deadline := time.Now().Add(time.Second)
	for target == nil && time.Now().Before(deadline) {
		hub.mu.RLock()
		for reg := range hub.byProfile["profile"] {
			target = reg
		}
		hub.mu.RUnlock()
		if target == nil {
			time.Sleep(time.Millisecond)
		}
	}
	if target == nil {
		t.Fatal("connection was not registered")
	}
	var closeFrames atomic.Int32
	conn.SetCloseHandler(func(code int, text string) error {
		closeFrames.Add(1)
		if code != 1013 || text != "fanout_overflow" {
			t.Errorf("close=%d %q want 1013 fanout_overflow", code, text)
		}
		return nil
	})
	// Freeze the pump on the first queued event. This makes saturation and the
	// overflow signal deterministic instead of racing the WS writer.
	enteredWrite := make(chan struct{}, 1)
	releaseWrite := make(chan struct{})
	target.setFanoutWriteHook(func() {
		enteredWrite <- struct{}{}
		<-releaseWrite
	})
	var closeWrites atomic.Int32
	target.setOverflowCloseHook(func(code int, reason string) {
		if code != 1013 || reason != "fanout_overflow" {
			t.Errorf("close write=%d %q want 1013 fanout_overflow", code, reason)
		}
		closeWrites.Add(1)
	})
	target.fanout <- fanoutEnvelope{Op: "queued", D: json.RawMessage(`{}`)}
	select {
	case <-enteredWrite:
	case <-time.After(time.Second):
		t.Fatal("WS pump did not block on queued fanout")
	}
	for len(target.fanout) < cap(target.fanout) {
		target.fanout <- fanoutEnvelope{Op: "queued", D: json.RawMessage(`{}`)}
	}
	hub.broadcastToProfile("profile", fanoutEnvelope{Op: "call_started", D: json.RawMessage(`{}`)}, nil, "")
	close(releaseWrite)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			break
		}
	}
	if got := closeWrites.Load(); got != 1 {
		t.Fatalf("close writes=%d want 1", got)
	}
	if got := closeFrames.Load(); got != 1 {
		t.Fatalf("close frames=%d want 1", got)
	}
}

func TestProfileFanoutLossyOverflowDoesNotSignalClose(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	reg := hub.attachConn("i", "slow", "profile", 1)
	reg.fanout <- fanoutEnvelope{Op: "older"}
	hub.broadcastToProfile("profile", fanoutEnvelope{Op: "voice_state_update"}, nil, "")
	select {
	case <-reg.overflow:
		t.Fatal("lossy operation must not close recipient")
	default:
	}
}

func TestProfileFanoutGuardPreventsOverflowSignal(t *testing.T) {
	t.Parallel()
	hub := newWSHub()
	reg := hub.attachConn("i", "guarded", "profile", 1)
	reg.fanout <- fanoutEnvelope{Op: "older"}
	reg.setWriteGuard(func() bool { return false })
	hub.broadcastToProfile("profile", fanoutEnvelope{Op: "call_started"}, nil, "")
	select {
	case <-reg.overflow:
		t.Fatal("guard-denied fanout must not signal overflow")
	default:
	}
}
