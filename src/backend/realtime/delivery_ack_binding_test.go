package main

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type stubDeliveryAckMessageReader struct {
	message deliveryAckMessage
	err     error
	calls   chan deliveryAckLookup
}

type deliveryAckLookup struct {
	messageID string
	viewerID  string
}

func (r *stubDeliveryAckMessageReader) GetMessage(_ context.Context, messageID, viewerProfileID string) (deliveryAckMessage, error) {
	r.calls <- deliveryAckLookup{messageID: messageID, viewerID: viewerProfileID}
	return r.message, r.err
}

type deliveryAckPublisherSpy struct {
	calls chan deliveryAckLookup
}

func (p *deliveryAckPublisherSpy) PublishDeliveryAck(_ context.Context, chatID, messageID, recipientID string) error {
	p.calls <- deliveryAckLookup{messageID: chatID + "/" + messageID, viewerID: recipientID}
	return nil
}

func TestDeliveryAckRequiresAuthoritativeVisibleMessageBinding(t *testing.T) {
	tests := []struct {
		name        string
		reader      func(chatID, messageID, senderID string) deliveryAckMessageReader
		payloadChat func(chatID string) string
		payloadID   func(messageID string) string
		payloadFrom func(senderID string) string
	}{
		{
			name:        "missing authoritative reader",
			reader:      func(string, string, string) deliveryAckMessageReader { return nil },
			payloadChat: func(v string) string { return v }, payloadID: func(v string) string { return v }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "missing message ID",
			reader: func(chatID, messageID, senderID string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{message: deliveryAckMessage{ID: messageID, ChatID: chatID, SenderProfileID: senderID}, calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(string) string { return "" }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "nonexistent message",
			reader: func(string, string, string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{err: errors.New("not found"), calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(v string) string { return v }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "future message ID",
			reader: func(chatID, _, senderID string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{err: errors.New("not found"), calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(string) string { return "7fffffff-ffff-4fff-bfff-ffffffffffff" }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "cross chat message",
			reader: func(_, messageID, senderID string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{message: deliveryAckMessage{ID: messageID, ChatID: uuid.NewString(), SenderProfileID: senderID}, calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(v string) string { return v }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "message ID does not match lookup result",
			reader: func(chatID, _, senderID string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{message: deliveryAckMessage{ID: uuid.NewString(), ChatID: chatID, SenderProfileID: senderID}, calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(v string) string { return v }, payloadFrom: func(v string) string { return v },
		},
		{
			name: "spoofed sender",
			reader: func(chatID, messageID, senderID string) deliveryAckMessageReader {
				return &stubDeliveryAckMessageReader{message: deliveryAckMessage{ID: messageID, ChatID: chatID, SenderProfileID: senderID}, calls: make(chan deliveryAckLookup, 1)}
			},
			payloadChat: func(v string) string { return v }, payloadID: func(v string) string { return v }, payloadFrom: func(string) string { return uuid.NewString() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accountID, viewerID, senderAccountID, senderID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			chatID, messageID := uuid.NewString(), uuid.NewString()
			hub := permitAllTestSubscriptions(newWSHub())
			hub.deliveryAckReader = tc.reader(chatID, messageID, senderID)
			publisher := &deliveryAckPublisherSpy{calls: make(chan deliveryAckLookup, 1)}
			server := httptest.NewServer(newServiceHandlerWithPresence(serviceName, staticTokenValidator{
				"viewer": {UserID: accountID, ProfileID: viewerID},
				"sender": {UserID: senderAccountID, ProfileID: senderID},
			}, perProfileBootstrapLister{viewerID: {chatID}, senderID: {chatID}}, hub, nil, "ack-binding-test", nil, publisher, readinessDeps{}))
			t.Cleanup(server.Close)
			viewer := dialACLTestConn(t, server, "viewer", viewerID)
			sender := dialACLTestConn(t, server, "sender", senderID)
			t.Cleanup(func() { _ = viewer.Close(); _ = sender.Close() })
			if got := readACLEnvelope(t, viewer); got.Op != "subscription_sync" {
				t.Fatalf("viewer bootstrap = %+v", got)
			}
			if got := readACLEnvelope(t, sender); got.Op != "subscription_sync" {
				t.Fatalf("sender bootstrap = %+v", got)
			}
			payload := map[string]any{
				"chat_id":           tc.payloadChat(chatID),
				"message_id":        tc.payloadID(messageID),
				"sender_profile_id": tc.payloadFrom(senderID),
			}
			if err := viewer.WriteJSON(map[string]any{"op": "delivery_ack", "d": payload}); err != nil {
				t.Fatalf("write delivery ack: %v", err)
			}
			if got := readACLEnvelope(t, viewer); got.Op != "error" {
				t.Fatalf("invalid acknowledgement response = %+v, want error", got)
			}
			select {
			case got := <-publisher.calls:
				t.Fatalf("invalid ack published durable side effect: %+v", got)
			default:
			}
			_ = sender.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			if _, _, err := sender.ReadMessage(); err == nil {
				t.Fatal("invalid ack produced a sender fanout")
			}
		})
	}
}

func TestDeliveryAckValidMessagePublishesRepeatableRecipientVisibleCursor(t *testing.T) {
	accountID, viewerID, senderAccountID, senderID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	chatID, messageID := uuid.NewString(), uuid.NewString()
	reader := &stubDeliveryAckMessageReader{
		message: deliveryAckMessage{ID: messageID, ChatID: chatID, SenderProfileID: senderID},
		calls:   make(chan deliveryAckLookup, 2),
	}
	hub := permitAllTestSubscriptions(newWSHub())
	hub.deliveryAckReader = reader
	publisher := &deliveryAckPublisherSpy{calls: make(chan deliveryAckLookup, 2)}
	server := httptest.NewServer(newServiceHandlerWithPresence(serviceName, staticTokenValidator{
		"viewer": {UserID: accountID, ProfileID: viewerID},
		"sender": {UserID: senderAccountID, ProfileID: senderID},
	}, perProfileBootstrapLister{viewerID: {chatID}, senderID: {chatID}}, hub, nil, "ack-binding-valid-test", nil, publisher, readinessDeps{}))
	t.Cleanup(server.Close)
	viewer := dialACLTestConn(t, server, "viewer", viewerID)
	sender := dialACLTestConn(t, server, "sender", senderID)
	t.Cleanup(func() { _ = viewer.Close(); _ = sender.Close() })
	_ = readACLEnvelope(t, viewer)
	_ = readACLEnvelope(t, sender)

	for i := 0; i < 2; i++ {
		if err := viewer.WriteJSON(map[string]any{"op": "delivery_ack", "d": map[string]any{
			"chat_id": chatID, "message_id": messageID, "sender_profile_id": senderID,
		}}); err != nil {
			t.Fatalf("write delivery ack: %v", err)
		}
		got := readACLEnvelope(t, sender)
		if got.Op != "message_delivered" {
			t.Fatalf("sender event = %+v, want message_delivered", got)
		}
		select {
		case call := <-reader.calls:
			if call.messageID != messageID || call.viewerID != viewerID {
				t.Fatalf("lookup = %+v, want message %s as verified viewer %s", call, messageID, viewerID)
			}
		case <-time.After(time.Second):
			t.Fatal("authoritative message lookup was not called")
		}
		select {
		case <-publisher.calls:
		case <-time.After(time.Second):
			t.Fatal("valid acknowledgement was not persisted")
		}
	}
}

type deliveryAckMessagingServer struct {
	messagingv1.UnimplementedMessagingServiceServer
	profileID      string
	internalCaller string
	messageID      string
}

func (s *deliveryAckMessagingServer) GetMessage(ctx context.Context, req *messagingv1.GetMessageRequest) (*messagingv1.GetMessageResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	profiles := md.Get(grpcMDVoiceProfileID)
	if len(profiles) == 1 {
		s.profileID = profiles[0]
	}
	callers := md.Get("x-voice-internal-caller")
	if len(callers) > 0 {
		s.internalCaller = callers[0]
	}
	s.messageID = req.GetMessageId()
	return &messagingv1.GetMessageResponse{Message: &messagingv1.Message{
		Id: req.GetMessageId(), Chat: &chatv1.ChatRef{Id: uuid.NewString()}, SenderProfileId: uuid.NewString(), CreatedAt: timestamppb.Now(),
	}}, nil
}

func TestGRPCDeliveryAckReaderUsesVerifiedViewerMetadata(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	messaging := &deliveryAckMessagingServer{}
	messagingv1.RegisterMessagingServiceServer(server, messaging)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///delivery-ack-test", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	reader := newGRPCDeliveryAckMessageReader(conn)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(grpcMDVoiceProfileID, "attacker", "x-voice-internal-caller", "trusted-service"))
	messageID, viewerID := uuid.NewString(), uuid.NewString()
	if _, err := reader.GetMessage(ctx, messageID, viewerID); err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if messaging.messageID != messageID || messaging.profileID != viewerID || messaging.internalCaller != "" {
		t.Fatalf("service observed message=%q profile=%q internal-caller=%q, want %q/%q/empty", messaging.messageID, messaging.profileID, messaging.internalCaller, messageID, viewerID)
	}
}
