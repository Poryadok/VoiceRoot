package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

type recordingDeliveryAckPublisher struct {
	calls chan struct{}
}

func (p *recordingDeliveryAckPublisher) PublishDeliveryAck(context.Context, string, string, string) error {
	p.calls <- struct{}{}
	return nil
}

func TestChatMemberRemovalRevokesEveryLocalTabAndSubscriptionGatedActions(t *testing.T) {
	oldTypingIdleTimeout := typingIdleTimeout
	typingIdleTimeout = 250 * time.Millisecond
	t.Cleanup(func() { typingIdleTimeout = oldTypingIdleTimeout })

	for _, change := range []string{"removed", "left"} {
		t.Run(change, func(t *testing.T) {
			accountID, profileID, peerAccountID, peerProfileID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
			chatID, messageID, senderID := uuid.NewString(), uuid.NewString(), uuid.NewString()
			hub := permitAllTestSubscriptions(newWSHub())
			deliveryPublisher := &recordingDeliveryAckPublisher{calls: make(chan struct{}, 1)}
			h := newServiceHandlerWithPresence(serviceName, staticTokenValidator{
				"desktop": {UserID: accountID, ProfileID: profileID},
				"mobile":  {UserID: accountID, ProfileID: profileID},
				"peer":    {UserID: peerAccountID, ProfileID: peerProfileID},
			}, perProfileBootstrapLister{peerProfileID: {chatID}}, hub, nil, "acl-revoke-test", nil, deliveryPublisher, readinessDeps{})
			wsServer := httptest.NewServer(h)
			t.Cleanup(wsServer.Close)
			desktop := dialACLTestConn(t, wsServer, "desktop", profileID)
			mobile := dialACLTestConn(t, wsServer, "mobile", profileID)
			peer := dialACLTestConn(t, wsServer, "peer", peerProfileID)
			for _, c := range []*websocket.Conn{desktop, mobile, peer} {
				if got := readACLEnvelope(t, c); got.Op != "subscription_sync" || got.S != 2 {
					t.Fatalf("bootstrap subscription = %+v", got)
				}
			}
			uppercaseChatID := strings.ToUpper(chatID)
			for _, c := range []*websocket.Conn{desktop, mobile} {
				if err := c.WriteJSON(map[string]any{"op": "subscribe", "d": map[string]any{"chat_id": uppercaseChatID}}); err != nil {
					t.Fatalf("uppercase subscribe before %s: %v", change, err)
				}
				if got := readACLEnvelope(t, c); got.Op != "subscribe_ack" || got.S != 3 {
					t.Fatalf("uppercase subscribe ack = %+v", got)
				}
			}
			jsServer := startRealtimeJSTestServer(t)
			nc, err := nats.Connect(jsServer.ClientURL())
			if err != nil {
				t.Fatalf("connect nats: %v", err)
			}
			t.Cleanup(nc.Close)
			js, err := nc.JetStream()
			if err != nil {
				t.Fatalf("jetstream: %v", err)
			}
			if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamChatEvents, Subjects: []string{"chat.>"}}); err != nil {
				t.Fatalf("add chat event stream: %v", err)
			}
			sub, err := subscribeChatEvents(js, hub, "acl-revoke-test", nil)
			if err != nil {
				t.Fatalf("subscribe chat events: %v", err)
			}
			t.Cleanup(func() { _ = sub.Unsubscribe() })
			if err := desktop.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": chatID}}); err != nil {
				t.Fatalf("typing_start before %s: %v", change, err)
			}
			for _, c := range []*websocket.Conn{mobile, peer} {
				if got := readACLEnvelope(t, c); got.Op != "typing" {
					t.Fatalf("typing start fan-out = %+v", got)
				}
			}

			eventBytes, err := proto.Marshal(&eventsv1.ChatStreamEvent{
				EventId: uuid.NewString(), OccurredAt: timestamppb.Now(),
				Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{ChatMemberChanged: &eventsv1.ChatMemberChanged{
					ChatId: chatID, ProfileId: profileID, Change: change,
				}},
			})
			if err != nil {
				t.Fatalf("marshal member event: %v", err)
			}
			if _, err := js.Publish("chat.member_changed", eventBytes); err != nil {
				t.Fatalf("publish member event: %v", err)
			}
			if got := readACLEnvelope(t, desktop); got.Op != "chat_update" || got.S != 4 {
				t.Fatalf("desktop membership update = %+v", got)
			}
			if got := readACLEnvelope(t, mobile); got.Op != "chat_update" || got.S != 5 {
				t.Fatalf("mobile membership update = %+v", got)
			}

			hub.mu.RLock()
			members, chatStillRegistered := hub.byChat[chatID]
			if !chatStillRegistered || len(members) != 1 {
				t.Errorf("chat registrations after removed/left = %d, want exactly peer tab", len(members))
			} else {
				for reg := range members {
					if reg.profileID != peerProfileID {
						t.Errorf("chat registration profile = %q, want peer profile %q", reg.profileID, peerProfileID)
					}
					if _, subscribed := reg.chats[chatID]; !subscribed {
						t.Error("peer registration lost connReg.chats authority")
					}
				}
			}
			hub.mu.RUnlock()
			hub.mu.RLock()
			regs := make([]*connReg, 0, len(hub.byProfile[profileID]))
			for reg := range hub.byProfile[profileID] {
				regs = append(regs, reg)
				if _, stillSubscribed := reg.chats[chatID]; stillSubscribed {
					t.Errorf("removed/left profile retained %s in connReg.chats for local tab %s", chatID, reg.connID)
				}
			}
			hub.mu.RUnlock()
			if len(regs) != 2 {
				t.Errorf("local profile registrations = %d, want both desktop and mobile tabs", len(regs))
			}

			for _, tc := range []struct {
				name string
				op   string
				d    map[string]any
				code string
			}{
				{"typing", "typing_start", map[string]any{"chat_id": chatID}, "invalid_typing"},
				{"mark_read", "mark_read", map[string]any{"chat_id": chatID, "message_id": messageID}, "invalid_mark_read"},
				{"delivery_ack", "delivery_ack", map[string]any{"chat_id": chatID, "message_id": messageID, "sender_profile_id": senderID}, "invalid_delivery_ack"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					for _, c := range []*websocket.Conn{desktop, mobile} {
						if err := c.WriteJSON(map[string]any{"op": tc.op, "d": tc.d}); err != nil {
							t.Fatalf("%s write: %v", tc.op, err)
						}
						env := readACLEnvelope(t, c)
						if env.Op != "error" {
							t.Fatalf("%s after %s = %+v, want local-subscription denial", tc.op, change, env)
						}
						var body struct {
							Code string `json:"code"`
						}
						if err := json.Unmarshal(env.D, &body); err != nil || body.Code != tc.code {
							t.Fatalf("%s denial body = %s, err=%v", tc.op, env.D, err)
						}
					}
				})
			}
			select {
			case <-deliveryPublisher.calls:
				t.Error("delivery_ack after removal/left published a durable side effect")
			default:
			}

			// This is the real client presence_update path, not only a direct hub
			// call. The peer remains subscribed; it must not observe presence from
			// a profile whose membership event revoked its local subscription.
			if err := desktop.WriteJSON(map[string]any{"op": "presence_update", "d": map[string]any{"status": "dnd"}}); err != nil {
				t.Fatalf("presence_update after %s: %v", change, err)
			}
			time.Sleep(typingIdleTimeout + 40*time.Millisecond)
			_ = peer.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			if _, _, err := peer.ReadMessage(); err == nil {
				t.Error("peer received presence_update or typing idle stop from a locally revoked profile")
			}

			// Run this last: a Gorilla read timeout makes the connection unreadable
			// for later assertions. A revoked tab receives neither message nor
			// presence fan-out through the chat-scoped hub paths.
			hub.broadcastToChat(chatID, fanoutEnvelope{Op: "message_create", D: json.RawMessage(`{}`)}, nil, "")
			hub.broadcastPresenceInChatExcept(chatID, "other-profile", "other-instance", "other-conn", json.RawMessage(`{}`))
			for _, c := range []*websocket.Conn{desktop, mobile} {
				_ = c.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
				if _, _, err := c.ReadMessage(); err == nil {
					t.Error("revoked local tab received a chat-scoped message/presence fan-out")
				}
			}

		})
	}
}
