package main

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestJetstreamDeliveryAckPublisher_PublishesMessageDeliveryAck(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)

	pub, err := newJetstreamDeliveryAckPublisher(nc)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      messageEventsStreamName,
		Subjects:  []string{"message.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	sub, err := js.SubscribeSync(subjectMessageDeliveryAck)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	chatID := "11111111-1111-1111-1111-111111111111"
	msgID := "22222222-2222-2222-2222-222222222222"
	recipientID := "33333333-3333-3333-3333-333333333333"

	if err := pub.PublishDeliveryAck(context.Background(), chatID, msgID, recipientID); err != nil {
		t.Fatalf("PublishDeliveryAck: %v", err)
	}

	msg, err := sub.NextMsg(5 * time.Second)
	if err != nil {
		t.Fatalf("next msg: %v", err)
	}
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(msg.Data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	da, ok := env.GetPayload().(*eventsv1.MessageStreamEvent_DeliveryAck)
	if !ok || da.DeliveryAck == nil {
		t.Fatalf("payload=%T", env.GetPayload())
	}
	if da.DeliveryAck.GetChatId() != chatID ||
		da.DeliveryAck.GetMessageId() != msgID ||
		da.DeliveryAck.GetProfileId() != recipientID {
		t.Fatalf("delivery ack=%+v", da.DeliveryAck)
	}
}
