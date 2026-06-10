package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func startRealtimeJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestMessageEventBytesToFanout_SentEditedDeleted(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	sender := uuid.NewString()

	sent := &eventsv1.MessageStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: sender,
			},
		},
	}
	b, err := proto.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	gotChat, fe, ok := messageEventBytesToFanout(b)
	if !ok || gotChat != chatID {
		t.Fatalf("sent: ok=%v chat=%q", ok, gotChat)
	}
	if fe.Op != "message_create" {
		t.Fatalf("op=%q", fe.Op)
	}
	var d map[string]any
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID || d["sender_profile_id"] != sender {
		t.Fatalf("d=%v", d)
	}

	edited := &eventsv1.MessageStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{MessageId: msgID, ChatId: chatID},
		},
	}
	b, _ = proto.Marshal(edited)
	gotChat, fe, ok = messageEventBytesToFanout(b)
	if !ok || gotChat != chatID || fe.Op != "message_update" {
		t.Fatalf("edited: ok=%v chat=%q op=%q", ok, gotChat, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID {
		t.Fatalf("edited d=%v", d)
	}

	deleted := &eventsv1.MessageStreamEvent{
		EventId:    "e3",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageDeleted{
			MessageDeleted: &eventsv1.MessageDeleted{MessageId: msgID, ChatId: chatID},
		},
	}
	b, _ = proto.Marshal(deleted)
	gotChat, fe, ok = messageEventBytesToFanout(b)
	if !ok || gotChat != chatID || fe.Op != "message_delete" {
		t.Fatalf("deleted: ok=%v chat=%q op=%q", ok, gotChat, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID {
		t.Fatalf("deleted d=%v", d)
	}

	read := &eventsv1.MessageStreamEvent{
		EventId:    "e4",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageRead{
			MessageRead: &eventsv1.MessageRead{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: uuid.NewString(),
			},
		},
	}
	b, _ = proto.Marshal(read)
	gotChat, fe, ok = messageEventBytesToFanout(b)
	if !ok || gotChat != chatID || fe.Op != "message_read" {
		t.Fatalf("read: ok=%v chat=%q op=%q", ok, gotChat, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID || d["profile_id"] == "" {
		t.Fatalf("read d=%v", d)
	}
}

// TestMessageEventBytesToFanout_ReactionAdd documents PLAN Phase 4 / realtime-service.md:
// message.reaction_added → WebSocket reaction_add for live counter updates.
func TestMessageEventBytesToFanout_ReactionAdd(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	profileID := uuid.NewString()
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: profileID,
				Emoji:     "👍",
			},
		},
	}
	b, _ := proto.Marshal(ev)
	gotChat, fe, ok := messageEventBytesToFanout(b)
	if !ok || gotChat != chatID || fe.Op != "reaction_add" {
		t.Fatalf("reaction_add: ok=%v chat=%q op=%q", ok, gotChat, fe.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID || d["profile_id"] != profileID || d["emoji"] != "👍" {
		t.Fatalf("reaction_add d=%v", d)
	}
}

func TestMessageEventBytesToFanout_ReactionRemove(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	profileID := uuid.NewString()
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionRemoved{
			ReactionRemoved: &eventsv1.ReactionRemoved{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: profileID,
				Emoji:     "🔥",
			},
		},
	}
	b, _ := proto.Marshal(ev)
	gotChat, fe, ok := messageEventBytesToFanout(b)
	if !ok || gotChat != chatID || fe.Op != "reaction_remove" {
		t.Fatalf("reaction_remove: ok=%v chat=%q op=%q", ok, gotChat, fe.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["message_id"] != msgID || d["profile_id"] != profileID || d["emoji"] != "🔥" {
		t.Fatalf("reaction_remove d=%v", d)
	}
}

func TestMessageEventBytesToFanout_ReactionSkipped(t *testing.T) {
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: uuid.NewString(),
				ProfileId: uuid.NewString(),
				Emoji:     "👍",
			},
		},
	}
	b, _ := proto.Marshal(ev)
	_, _, ok := messageEventBytesToFanout(b)
	if ok {
		t.Fatal("expected reaction to be skipped for WS fan-out")
	}
}

func TestConsumerDurableName_ReplacesHyphensInInstanceID(t *testing.T) {
	if got := consumerDurableName("a-b-c"); got != "rt_abc_msg" {
		t.Fatalf("got %q", got)
	}
	if got := consumerDurableName(""); got != "rt_unknown_msg" {
		t.Fatalf("empty instance: got %q", got)
	}
}

func TestRunMessageEventsConsumer_ErrorsWhenHubOrNATSURLMissing(t *testing.T) {
	ctx := context.Background()
	if err := runMessageEventsConsumer(ctx, nil, "nats://127.0.0.1:4222", "x", nil); err == nil {
		t.Fatal("expected error for nil hub")
	}
	hub := newWSHub()
	if err := runMessageEventsConsumer(ctx, hub, "", "x", nil); err == nil {
		t.Fatal("expected error for empty NATS URL")
	}
	if err := runMessageEventsConsumer(ctx, hub, "   ", "x", nil); err == nil {
		t.Fatal("expected error for whitespace NATS URL")
	}
}

func TestMessageEventBytesToFanout_InvalidOrIncompletePayload(t *testing.T) {
	_, _, ok := messageEventBytesToFanout([]byte{0x01, 0x02})
	if ok {
		t.Fatal("invalid protobuf should be skipped")
	}

	sentNil := &eventsv1.MessageStreamEvent{
		EventId:    "e",
		OccurredAt: timestamppb.Now(),
		Payload:    &eventsv1.MessageStreamEvent_MessageSent{MessageSent: nil},
	}
	b, _ := proto.Marshal(sentNil)
	_, _, ok = messageEventBytesToFanout(b)
	if ok {
		t.Fatal("nil MessageSent should be skipped")
	}

	sentNoChat := &eventsv1.MessageStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{MessageId: uuid.NewString(), ChatId: ""},
		},
	}
	b, _ = proto.Marshal(sentNoChat)
	_, _, ok = messageEventBytesToFanout(b)
	if ok {
		t.Fatal("empty chat_id should be skipped")
	}
}

func TestRunMessageEventsConsumer_JetStreamToHub(t *testing.T) {
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
		Name:      "message_events",
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	chatID := "44444444-4444-4444-4444-444444444444"
	msgID := uuid.NewString()
	sender := uuid.NewString()

	hub := newWSHub()
	reg := hub.attachConn("test-inst", "conn-1", "", 8)
	hub.addChat(reg, chatID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, "test-consumer-inst", nil) }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: sender,
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("message.sent", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case fe := <-reg.fanout:
		if fe.Op != "message_create" {
			t.Fatalf("op=%q", fe.Op)
		}
		var d map[string]any
		if err := json.Unmarshal(fe.D, &d); err != nil {
			t.Fatal(err)
		}
		if d["message_id"] != msgID || d["chat_id"] != chatID {
			t.Fatalf("payload=%v", d)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for fan-out")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not exit")
	}
}

// TestRunMessageEventsConsumer_InAppNotificationOnMessageSent documents PLAN Phase 4:
// MessageSent → message_create to chat subscribers AND personal notification (new_message)
// to every subscribed profile except the sender.
func TestRunMessageEventsConsumer_InAppNotificationOnMessageSent(t *testing.T) {
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
		Name:      "message_events",
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	chatID := "55555555-5555-5555-5555-555555555555"
	msgID := uuid.NewString()
	senderID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recipientID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	hub := newWSHub()
	senderReg := hub.attachConn("test-inst", "conn-sender", senderID, 16)
	recipientReg := hub.attachConn("test-inst", "conn-recipient", recipientID, 16)
	hub.addChat(senderReg, chatID)
	hub.addChat(recipientReg, chatID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, "test-notify-inst", nil) }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: senderID,
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("message.sent", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Sender: message_create only (no personal notification).
	select {
	case fe := <-senderReg.fanout:
		if fe.Op != "message_create" {
			t.Fatalf("sender first op=%q want message_create", fe.Op)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for sender message_create")
	}
	select {
	case fe := <-senderReg.fanout:
		t.Fatalf("sender unexpected second frame op=%q", fe.Op)
	case <-time.After(300 * time.Millisecond):
	}

	// Recipient: message_create then personal notification.
	select {
	case fe := <-recipientReg.fanout:
		if fe.Op != "message_create" {
			t.Fatalf("recipient first op=%q want message_create", fe.Op)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for recipient message_create")
	}
	select {
	case fe := <-recipientReg.fanout:
		if fe.Op != "notification" {
			t.Fatalf("recipient second op=%q want notification", fe.Op)
		}
		var d map[string]string
		if err := json.Unmarshal(fe.D, &d); err != nil {
			t.Fatal(err)
		}
		if d["type"] != "new_message" || d["chat_id"] != chatID || d["message_id"] != msgID || d["sender_profile_id"] != senderID {
			t.Fatalf("notification payload=%v", d)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for recipient notification")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not exit")
	}
}

// TestRunMessageEventsConsumer_MessageReadNoInAppNotification documents PLAN Phase 4:
// MessageRead (mark_read sync) fans out message_read only — no personal notification op.
func TestRunMessageEventsConsumer_MessageReadNoInAppNotification(t *testing.T) {
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
		Name:      "message_events",
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	chatID := "77777777-7777-7777-7777-777777777777"
	msgID := uuid.NewString()
	readerID := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	otherID := "ffffffff-ffff-ffff-ffff-ffffffffffff"

	hub := newWSHub()
	readerReg := hub.attachConn("test-inst", "conn-reader", readerID, 16)
	otherReg := hub.attachConn("test-inst", "conn-other", otherID, 16)
	hub.addChat(readerReg, chatID)
	hub.addChat(otherReg, chatID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, "test-read-notify-inst", nil) }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageRead{
			MessageRead: &eventsv1.MessageRead{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: readerID,
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("message.read", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	for _, reg := range []*connReg{readerReg, otherReg} {
		select {
		case fe := <-reg.fanout:
			if fe.Op != "message_read" {
				t.Fatalf("first op=%q want message_read", fe.Op)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("timeout waiting for message_read")
		}
		select {
		case fe := <-reg.fanout:
			t.Fatalf("unexpected second frame op=%q (no notification)", fe.Op)
		case <-time.After(300 * time.Millisecond):
		}
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not exit")
	}
}

// TestRunMessageEventsConsumer_InAppNotificationOnReactionAdded documents PLAN Phase 4:
// ReactionAdded → reaction_add to chat AND personal notification (reaction) to message author.
func TestRunMessageEventsConsumer_InAppNotificationOnReactionAdded(t *testing.T) {
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
		Name:      "message_events",
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	chatID := "66666666-6666-6666-6666-666666666666"
	msgID := uuid.NewString()
	authorID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	reactorID := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	hub := newWSHub()
	authorReg := hub.attachConn("test-inst", "conn-author", authorID, 16)
	reactorReg := hub.attachConn("test-inst", "conn-reactor", reactorID, 16)
	hub.addChat(authorReg, chatID)
	hub.addChat(reactorReg, chatID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runMessageEventsConsumer(ctx, hub, natsURL, "test-reaction-notify-inst", nil) }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: reactorID,
				Emoji:     "👍",
				// Contract: message_author_profile_id = 5 on ReactionAdded (jetstream_events.proto).
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("message.reaction_added", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case fe := <-authorReg.fanout:
		if fe.Op != "notification" {
			t.Fatalf("author first op=%q want notification", fe.Op)
		}
		var d map[string]string
		if err := json.Unmarshal(fe.D, &d); err != nil {
			t.Fatal(err)
		}
		if d["type"] != "reaction" || d["chat_id"] != chatID || d["message_id"] != msgID {
			t.Fatalf("author notification payload=%v", d)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for author reaction notification")
	}

	select {
	case fe := <-reactorReg.fanout:
		if fe.Op != "reaction_add" {
			t.Fatalf("reactor op=%q want reaction_add", fe.Op)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for reactor reaction_add")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not exit")
	}
}
