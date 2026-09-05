package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestChatEventBytesToFanout_CreatedAndMemberChanged(t *testing.T) {
	chatID := uuid.NewString()
	profileID := uuid.NewString()

	created := &eventsv1.ChatStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatCreated{
			ChatCreated: &eventsv1.ChatCreated{ChatId: chatID, Type: "dm"},
		},
	}
	b, err := proto.Marshal(created)
	if err != nil {
		t.Fatal(err)
	}
	pid, fe, ok := chatEventBytesToFanout(b)
	if !ok || pid != "" || fe.Op != "chat_update" {
		t.Fatalf("created: pid=%q ok=%v op=%q", pid, ok, fe.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["type"] != "dm" {
		t.Fatalf("created d=%v", d)
	}

	changed := &eventsv1.ChatStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{
			ChatMemberChanged: &eventsv1.ChatMemberChanged{
				ChatId:    chatID,
				ProfileId: profileID,
				Change:    "joined",
			},
		},
	}
	b, _ = proto.Marshal(changed)
	pid, fe, ok = chatEventBytesToFanout(b)
	if !ok || pid != profileID || fe.Op != "chat_update" {
		t.Fatalf("member: pid=%q ok=%v op=%q", pid, ok, fe.Op)
	}
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["chat_id"] != chatID || d["profile_id"] != profileID || d["change"] != "joined" {
		t.Fatalf("member d=%v", d)
	}
}

func TestChatEventBytesToFanout_DmPeerDeleted(t *testing.T) {
	chatID := uuid.NewString()
	recipientProfileID := uuid.NewString()

	data, err := proto.Marshal(&eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_DmPeerDeleted{
			DmPeerDeleted: &eventsv1.DmPeerDeleted{
				ChatId:             chatID,
				RecipientProfileId: recipientProfileID,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	profileID, env, ok := chatEventBytesToFanout(data)
	if !ok || profileID != recipientProfileID || env.Op != "dm_peer_deleted" {
		t.Fatalf("profileID=%q ok=%v op=%q", profileID, ok, env.Op)
	}
	var payload map[string]string
	if err := json.Unmarshal(env.D, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 2 || payload["chat_id"] != chatID || payload["recipient_profile_id"] != recipientProfileID {
		t.Fatalf("payload=%v", payload)
	}
}

func TestChatEventBytesToFanout_DmPeerDeletedRejectsMissingOrMalformedIDs(t *testing.T) {
	validChatID := uuid.NewString()
	validRecipientProfileID := uuid.NewString()

	for _, tc := range []struct {
		name               string
		chatID             string
		recipientProfileID string
		payloadIsNil       bool
	}{
		{name: "nil payload", payloadIsNil: true},
		{name: "missing chat ID", recipientProfileID: validRecipientProfileID},
		{name: "missing recipient profile ID", chatID: validChatID},
		{name: "malformed chat ID", chatID: "not-a-uuid", recipientProfileID: validRecipientProfileID},
		{name: "malformed recipient profile ID", chatID: validChatID, recipientProfileID: "not-a-uuid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := &eventsv1.ChatStreamEvent{
				EventId:    uuid.NewString(),
				OccurredAt: timestamppb.Now(),
				Payload: &eventsv1.ChatStreamEvent_DmPeerDeleted{
					DmPeerDeleted: &eventsv1.DmPeerDeleted{
						ChatId:             tc.chatID,
						RecipientProfileId: tc.recipientProfileID,
					},
				},
			}
			if tc.payloadIsNil {
				event.Payload = &eventsv1.ChatStreamEvent_DmPeerDeleted{}
			}
			data, err := proto.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			if profileID, env, ok := chatEventBytesToFanout(data); ok || profileID != "" || env.Op != "" || len(env.D) != 0 {
				t.Fatalf("profileID=%q env=%+v ok=%v", profileID, env, ok)
			}
		})
	}
}

func TestChatConsumerDurableName_ReplacesHyphens(t *testing.T) {
	if got := chatConsumerDurableName("a-b"); got != "rt_ab_chat" {
		t.Fatalf("got %q", got)
	}
}

func TestRunChatEventsConsumer_ErrorsWhenHubOrNATSURLMissing(t *testing.T) {
	ctx := context.Background()
	if err := runChatEventsConsumer(ctx, nil, "nats://127.0.0.1:4222", "x", nil); err == nil {
		t.Fatal("expected error for nil hub")
	}
	hub := newWSHub()
	if err := runChatEventsConsumer(ctx, hub, "", "x", nil); err == nil {
		t.Fatal("expected error for empty NATS URL")
	}
}

func TestRunChatEventsConsumer_JetStreamToProfile(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      jsStreamChatEvents,
		Subjects:  []string{"chat.>"},
		Retention: nats.LimitsPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}

	hub := newWSHub()
	profileID := uuid.NewString()
	chatID := uuid.NewString()
	reg := hub.attachConn("inst", "conn-1", profileID, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runChatEventsConsumer(ctx, hub, natsURL, "chat-test-inst", nil) }()
	time.Sleep(300 * time.Millisecond)

	changed := &eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{
			ChatMemberChanged: &eventsv1.ChatMemberChanged{
				ChatId:    chatID,
				ProfileId: profileID,
				Change:    "joined",
			},
		},
	}
	b, _ := proto.Marshal(changed)
	if _, err := js.Publish("chat.member_changed", b); err != nil {
		t.Fatal(err)
	}

	select {
	case env := <-reg.fanout:
		if env.Op != "chat_update" {
			t.Fatalf("op=%q", env.Op)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for chat_update fan-out")
	}
	hub.mu.RLock()
	_, joinedSubscribed := reg.chats[chatID]
	_, joinedRegistered := hub.byChat[chatID]
	hub.mu.RUnlock()
	if joinedSubscribed || joinedRegistered {
		t.Fatal("joined membership event must not auto-subscribe the local connection")
	}
	cancel()
	<-errCh
}

func TestSubscribeChatEvents_DmPeerDeletedTargetsRecipientProfile(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{
		Name: jsStreamChatEvents, Subjects: []string{"chat.>"}, Retention: nats.LimitsPolicy,
	}); err != nil {
		t.Fatal(err)
	}

	hub := newWSHub()
	chatID := uuid.NewString()
	recipientProfileID := uuid.NewString()
	recipientFirst := hub.attachConn("inst", "recipient-first", recipientProfileID, 1)
	recipientSecond := hub.attachConn("inst", "recipient-second", recipientProfileID, 1)
	nonRecipient := hub.attachConn("inst", "non-recipient", uuid.NewString(), 1)
	hub.addChat(nonRecipient, chatID)

	sub, err := subscribeChatEvents(js, hub, "dm-peer-deleted-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	data, err := proto.Marshal(&eventsv1.ChatStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.ChatStreamEvent_DmPeerDeleted{
			DmPeerDeleted: &eventsv1.DmPeerDeleted{
				ChatId:             chatID,
				RecipientProfileId: recipientProfileID,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("chat.dm_peer_deleted", data); err != nil {
		t.Fatal(err)
	}

	for _, recipient := range []*connReg{recipientFirst, recipientSecond} {
		select {
		case env := <-recipient.fanout:
			if env.Op != "dm_peer_deleted" {
				t.Fatalf("recipient op=%q", env.Op)
			}
			var payload map[string]string
			if err := json.Unmarshal(env.D, &payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload["chat_id"] != chatID || payload["recipient_profile_id"] != recipientProfileID {
				t.Fatalf("recipient payload=%v", payload)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for targeted dm_peer_deleted fan-out")
		}
	}

	select {
	case env := <-nonRecipient.fanout:
		t.Fatalf("non-recipient received %+v", env)
	case <-time.After(250 * time.Millisecond):
	}
}
