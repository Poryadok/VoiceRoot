package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

// JetStream message.events → Realtime hub → WebSocket client (docs/microservices/realtime-service.md).

func TestWSReceivesMessageSentUpdateDeleteFromJetStreamNATS(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      jsStreamMessageEvents,
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	hub := newWSHub()
	wsInst := "nats-ws-inst"
	jsInst := "nats-js-consumer"
	v := staticTokenValidator{"tok": {UserID: "user-1", ProfileID: "profile-1"}}
	srv := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub, nil, wsInst))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, jsInst) }()
	time.Sleep(250 * time.Millisecond)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "profile-1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	readEnv := func() wsEnvelope {
		t.Helper()
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var env wsEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			t.Fatalf("json: %v", err)
		}
		return env
	}

	if op := readEnv(); op.Op != "hello" || op.S != 1 {
		t.Fatalf("hello = %+v", op)
	}

	chatID := "55555555-5555-5555-5555-555555555555"
	msgID := "66666666-6666-6666-6666-666666666666"
	sender := "77777777-7777-7777-7777-777777777777"

	if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": chatID}}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if op := readEnv(); op.Op != "subscribe_ack" || op.S != 2 {
		t.Fatalf("subscribe_ack = %+v", op)
	}

	publish := func(subject string, ev *eventsv1.MessageStreamEvent) {
		t.Helper()
		b, err := proto.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := js.Publish(subject, b); err != nil {
			t.Fatalf("publish %s: %v", subject, err)
		}
		time.Sleep(150 * time.Millisecond)
	}

	publish("message.sent", &eventsv1.MessageStreamEvent{
		EventId:    "e-sent",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: sender,
			},
		},
	})
	mc := readEnv()
	if mc.Op != "message_create" || mc.S != 3 {
		t.Fatalf("message_create = %+v", mc)
	}
	var dCreate map[string]any
	if err := json.Unmarshal(mc.D, &dCreate); err != nil {
		t.Fatal(err)
	}
	if dCreate["chat_id"] != chatID || dCreate["message_id"] != msgID || dCreate["sender_profile_id"] != sender {
		t.Fatalf("message_create d = %v", dCreate)
	}

	publish("message.edited", &eventsv1.MessageStreamEvent{
		EventId:    "e-edited",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{MessageId: msgID, ChatId: chatID},
		},
	})
	mu := readEnv()
	if mu.Op != "message_update" || mu.S != 4 {
		t.Fatalf("message_update = %+v", mu)
	}

	publish("message.deleted", &eventsv1.MessageStreamEvent{
		EventId:    "e-deleted",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{MessageId: msgID, ChatId: chatID},
		},
	})
	md := readEnv()
	if md.Op != "message_delete" || md.S != 5 {
		t.Fatalf("message_delete = %+v", md)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("consumer did not exit")
	}
}

func TestWSNoMessageCreateFromNATSWhenNotSubscribedToChat(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      jsStreamMessageEvents,
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	hub := newWSHub()
	v := staticTokenValidator{"tok": {UserID: "u1", ProfileID: "p1"}}
	srv := httptest.NewServer(newServiceHandler(serviceName, v, nil, hub, nil, "ws-only"))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = runMessageEventsConsumer(ctx, hub, natsURL, "js-only") }()
	time.Sleep(250 * time.Millisecond)

	u := wsEndpoint(t, srv)
	hdr := wsUpgradeHeaders("tok")
	hdr.Set("X-Profile-Id", "p1")
	c, _, err := websocket.DefaultDialer.Dial(u, hdr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, helloData, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("hello: %v", err)
	}
	var hello wsEnvelope
	if err := json.Unmarshal(helloData, &hello); err != nil || hello.Op != "hello" {
		t.Fatalf("hello = %+v err=%v", hello, err)
	}

	chatID := "88888888-8888-8888-8888-888888888888"
	msgID := "99999999-9999-9999-9999-999999999999"
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			},
		},
	}
	b, _ := proto.Marshal(ev)
	if _, err := js.Publish("message.sent", b); err != nil {
		t.Fatalf("publish: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := c.WriteJSON(map[string]any{"op": "heartbeat"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, hbData, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("heartbeat_ack: %v", err)
	}
	var hb wsEnvelope
	if err := json.Unmarshal(hbData, &hb); err != nil || hb.Op != "heartbeat_ack" {
		t.Fatalf("expected heartbeat_ack, got %+v err=%v", hb, err)
	}

	_ = c.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	_, _, err = c.ReadMessage()
	if err == nil {
		t.Fatal("expected no WS payload for unsubscribed chat; got a frame")
	}
}
