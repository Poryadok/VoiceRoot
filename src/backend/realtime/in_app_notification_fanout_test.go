package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func notificationPayload(t *testing.T, env fanoutEnvelope) map[string]string {
	t.Helper()
	if env.Op != "notification" {
		t.Fatalf("op=%q want notification", env.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(env.D, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestInAppNotificationFanouts_NewMessage(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	senderID := uuid.NewString()
	recipientA := uuid.NewString()
	recipientB := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: senderID,
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, []string{senderID, recipientA, recipientB}, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 2 {
		t.Fatalf("fanouts=%d want 2", len(fanouts))
	}

	byProfile := map[string]map[string]string{}
	for _, f := range fanouts {
		byProfile[f.ProfileID] = notificationPayload(t, f.Envelope)
	}

	for _, recipientID := range []string{recipientA, recipientB} {
		d, found := byProfile[recipientID]
		if !found {
			t.Fatalf("missing recipient %s", recipientID)
		}
		if d["type"] != "new_message" || d["chat_id"] != chatID || d["message_id"] != msgID || d["sender_profile_id"] != senderID {
			t.Fatalf("recipient %s payload=%v", recipientID, d)
		}
	}
	if _, senderNotified := byProfile[senderID]; senderNotified {
		t.Fatal("sender must not receive new_message notification")
	}
}

func TestInAppNotificationFanouts_ReactionNotifiesAuthor(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	authorID := uuid.NewString()
	reactorID := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: reactorID,
				Emoji:     "👍",
				// Contract: add message_author_profile_id = 5 to ReactionAdded in jetstream_events.proto.
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, nil, authorID)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 1 {
		t.Fatalf("fanouts=%d want 1", len(fanouts))
	}

	d := notificationPayload(t, fanouts[0].Envelope)
	if fanouts[0].ProfileID != authorID {
		t.Fatalf("profile=%q want %q", fanouts[0].ProfileID, authorID)
	}
	if d["type"] != "reaction" || d["chat_id"] != chatID || d["message_id"] != msgID || d["reactor_profile_id"] != reactorID || d["emoji"] != "👍" {
		t.Fatalf("payload=%v", d)
	}
}

func TestInAppNotificationFanouts_InvalidProtobuf(t *testing.T) {
	fanouts, ok := inAppNotificationFanouts([]byte{0x01, 0x02}, nil, "")
	if ok || fanouts != nil {
		t.Fatalf("invalid protobuf: ok=%v fanouts=%v", ok, fanouts)
	}
}

func TestInAppNotificationFanouts_UnknownPayload(t *testing.T) {
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e-unknown",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageEdited{
			MessageEdited: &eventsv1.MessageEdited{
				MessageId: uuid.NewString(),
				ChatId:    uuid.NewString(),
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	fanouts, ok := inAppNotificationFanouts(b, nil, "")
	if ok || fanouts != nil {
		t.Fatalf("MessageEdited: ok=%v fanouts=%v", ok, fanouts)
	}
}

func TestInAppNotificationFanouts_NewMessageSkipsEmptyMemberIDs(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e-empty-member",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				MessageId:       msgID,
				ChatId:          chatID,
				SenderProfileId: senderID,
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, []string{"", senderID, recipientID}, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 1 {
		t.Fatalf("fanouts=%d want 1 (empty member skipped)", len(fanouts))
	}
	if fanouts[0].ProfileID != recipientID {
		t.Fatalf("profile=%q want %q", fanouts[0].ProfileID, recipientID)
	}
}

func TestInAppNotificationFanouts_ReactionDegradesTwoMemberDM(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	authorID := uuid.NewString()
	reactorID := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e-dm-degrade",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: reactorID,
				Emoji:     "👍",
				// message_author_profile_id omitted — degrade via 2-member chat list.
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, []string{authorID, reactorID}, "")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 1 {
		t.Fatalf("fanouts=%d want 1", len(fanouts))
	}
	if fanouts[0].ProfileID != authorID {
		t.Fatalf("profile=%q want author %q", fanouts[0].ProfileID, authorID)
	}
}

func TestInAppNotificationFanouts_ReactionNoAuthorWhenCannotDegrade(t *testing.T) {
	reactorID := uuid.NewString()
	otherA := uuid.NewString()
	otherB := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e-no-author",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: uuid.NewString(),
				ChatId:    uuid.NewString(),
				ProfileId: reactorID,
				Emoji:     "👍",
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, []string{reactorID, otherA, otherB}, "")
	if !ok {
		t.Fatal("expected ok (graceful skip)")
	}
	if len(fanouts) != 0 {
		t.Fatalf("fanouts=%v want none when author unknown in group", fanouts)
	}
}

func TestInAppNotificationFanouts_ReactionUsesProtoAuthorID(t *testing.T) {
	authorID := uuid.NewString()
	reactorID := uuid.NewString()
	fallbackAuthor := uuid.NewString()

	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e-proto-author",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId:              uuid.NewString(),
				ChatId:                 uuid.NewString(),
				ProfileId:              reactorID,
				Emoji:                  "🔥",
				MessageAuthorProfileId: authorID,
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, nil, fallbackAuthor)
	if !ok || len(fanouts) != 1 {
		t.Fatalf("ok=%v fanouts=%v", ok, fanouts)
	}
	if fanouts[0].ProfileID != authorID {
		t.Fatalf("profile=%q want proto author %q not fallback %q", fanouts[0].ProfileID, authorID, fallbackAuthor)
	}
}

func TestInAppNotificationFanouts_ReactionSkipsSelfReaction(t *testing.T) {
	authorID := uuid.NewString()
	ev := &eventsv1.MessageStreamEvent{
		EventId:    "e3",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: uuid.NewString(),
				ChatId:    uuid.NewString(),
				ProfileId: authorID,
				Emoji:     "🔥",
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := inAppNotificationFanouts(b, nil, authorID)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 0 {
		t.Fatalf("author self-reaction fanouts=%v", fanouts)
	}
}

func TestDispatchMessageStreamEvent_MessageSentFansOutChatAndNotifications(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	senderID := uuid.NewString()
	recipientID := uuid.NewString()

	hub := newWSHub()
	senderReg := hub.attachConn("inst", "conn-sender", senderID, 16)
	recipientReg := hub.attachConn("inst", "conn-recipient", recipientID, 16)
	hub.addChat(senderReg, chatID)
	hub.addChat(recipientReg, chatID)

	ev := &eventsv1.MessageStreamEvent{
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
	payload, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	dispatchMessageStreamEvent(hub, payload, nil, "")

	senderOps := drainFanoutOps(t, senderReg, 2*time.Second)
	recipientOps := drainFanoutOps(t, recipientReg, 2*time.Second)

	if !containsOp(senderOps, "message_create") {
		t.Fatalf("sender ops=%v", senderOps)
	}
	if containsOp(senderOps, "notification") {
		t.Fatalf("sender got notification ops=%v", senderOps)
	}
	if !containsOp(recipientOps, "message_create") || !containsOp(recipientOps, "notification") {
		t.Fatalf("recipient ops=%v", recipientOps)
	}
}

func TestDispatchMessageStreamEvent_MessageReadNoNotification(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	readerID := uuid.NewString()
	otherID := uuid.NewString()

	hub := newWSHub()
	readerReg := hub.attachConn("inst", "conn-reader", readerID, 16)
	otherReg := hub.attachConn("inst", "conn-other", otherID, 16)
	hub.addChat(readerReg, chatID)
	hub.addChat(otherReg, chatID)

	ev := &eventsv1.MessageStreamEvent{
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
	payload, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	dispatchMessageStreamEvent(hub, payload, nil, "")

	for _, reg := range []*connReg{readerReg, otherReg} {
		ops := drainFanoutOps(t, reg, 2*time.Second)
		if containsOp(ops, "notification") {
			t.Fatalf("mark_read/message_read must not emit notification, ops=%v", ops)
		}
		if !containsOp(ops, "message_read") {
			t.Fatalf("expected message_read, ops=%v", ops)
		}
	}
}

func TestDispatchMessageStreamEvent_ReactionFansOutChatAndAuthorNotification(t *testing.T) {
	chatID := uuid.NewString()
	msgID := uuid.NewString()
	authorID := uuid.NewString()
	reactorID := uuid.NewString()

	hub := newWSHub()
	authorReg := hub.attachConn("inst", "conn-author", authorID, 16)
	reactorReg := hub.attachConn("inst", "conn-reactor", reactorID, 16)
	hub.addChat(authorReg, chatID)
	hub.addChat(reactorReg, chatID)

	ev := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_ReactionAdded{
			ReactionAdded: &eventsv1.ReactionAdded{
				MessageId: msgID,
				ChatId:    chatID,
				ProfileId: reactorID,
				Emoji:     "👍",
			},
		},
	}
	payload, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	// Until proto carries message_author_profile_id, hub dispatch derives author from chat state / event enrichment.
	dispatchMessageStreamEvent(hub, payload, nil, "")

	authorOps := drainFanoutOps(t, authorReg, 2*time.Second)
	reactorOps := drainFanoutOps(t, reactorReg, 2*time.Second)

	if !containsOp(authorOps, "notification") {
		t.Fatalf("author ops=%v", authorOps)
	}
	if !containsOp(reactorOps, "reaction_add") {
		t.Fatalf("reactor ops=%v", reactorOps)
	}
	if containsOp(reactorOps, "notification") {
		t.Fatalf("reactor got notification ops=%v", reactorOps)
	}
}

func containsOp(ops []string, want string) bool {
	for _, op := range ops {
		if op == want {
			return true
		}
	}
	return false
}

func drainFanoutOps(t *testing.T, reg *connReg, wait time.Duration) []string {
	t.Helper()
	deadline := time.After(wait)
	var ops []string
	for {
		select {
		case fe := <-reg.fanout:
			ops = append(ops, fe.Op)
		case <-deadline:
			return ops
		}
	}
}
