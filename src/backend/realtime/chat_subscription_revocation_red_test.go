package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
)

type recordingDeliveryAckPublisher struct {
	calls chan struct{}
}

func (p *recordingDeliveryAckPublisher) PublishDeliveryAck(context.Context, string, string, string) error {
	p.calls <- struct{}{}
	return nil
}

func addAuthorizedTestDMChat(t *testing.T, hub *wsHub, reg *connReg, chatID, accountA, accountB string) {
	t.Helper()
	pair, ok := canonicalAccountPair(accountA, accountB)
	require.True(t, ok)
	version := hub.beginDMPairCheck(pair)
	require.True(t, hub.finishDMPairCheck(pair, version, reg, chatID, true))
}

func TestChatMemberRemovalRevokesEveryLocalTabAndSubscriptionGatedActions(t *testing.T) {
	oldTypingIdleTimeout := typingIdleTimeout
	typingIdleTimeout = 250 * time.Millisecond
	t.Cleanup(func() { typingIdleTimeout = oldTypingIdleTimeout })

	for _, change := range []string{"removed", "left"} {
		for _, typingCase := range []string{"canonical", "uppercase"} {
			t.Run(change+"/"+typingCase, func(t *testing.T) {
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
				typingChatID := chatID
				if typingCase == "uppercase" {
					typingChatID = uppercaseChatID
				}
				if err := desktop.WriteJSON(map[string]any{"op": "typing_start", "d": map[string]any{"chat_id": typingChatID}}); err != nil {
					t.Fatalf("typing_start (%s) before %s: %v", typingCase, change, err)
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
				// Profile-scope presence sync to the other local tab remains valid; the
				// revoked chat subscription must only prevent chat-scoped peer fan-out.
				if got := readACLEnvelope(t, mobile); got.Op != "presence_update" {
					t.Fatalf("local profile presence sync = %+v", got)
				}
				_ = peer.SetReadDeadline(time.Now().Add(typingIdleTimeout + 250*time.Millisecond))
				if _, _, err := peer.ReadMessage(); err == nil {
					t.Errorf("peer received presence_update or typing idle stop after %s UUID subscription revocation", typingCase)
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
}

func TestSocialBlockEventRevokesOpenDMEverywhereAndPreservesOtherChats(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	dmChatID, groupChatID := uuid.NewString(), uuid.NewString()
	jsServer := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(jsServer.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamSocialEvents, Subjects: []string{"social.>"}}); err != nil {
		t.Fatal(err)
	}

	hubs := []*wsHub{newWSHub(), newWSHub()}
	for i, hub := range hubs {
		profileID := uuid.NewString()
		accountID := accountA
		if i == 1 {
			accountID = accountB
		}
		reg := hub.attachAccountConn("instance", "conn", accountID, profileID, 1)
		addAuthorizedTestDMChat(t, hub, reg, dmChatID, accountA, accountB)
		hub.addChat(reg, groupChatID)
		sub, err := subscribeSocialEvents(js, hub, "social-block-instance-"+strconv.Itoa(i), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sub.Unsubscribe() })
	}

	eventBytes, err := proto.Marshal(&eventsv1.SocialStreamEvent{
		EventId: uuid.NewString(), OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.SocialStreamEvent_UserBlocked{UserBlocked: &eventsv1.UserBlocked{
			BlockerAccountId: accountA, BlockedAccountId: accountB,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("social.user_blocked", eventBytes); err != nil {
		t.Fatal(err)
	}

	for i, hub := range hubs {
		require.Eventually(t, func() bool {
			hub.mu.RLock()
			defer hub.mu.RUnlock()
			_, dmRegistered := hub.byChat[dmChatID]
			_, groupRegistered := hub.byChat[groupChatID]
			return !dmRegistered && groupRegistered
		}, 5*time.Second, 10*time.Millisecond, "instance %d must revoke only the blocked DM", i)
		hub.mu.RLock()
		for reg := range hub.byChat[groupChatID] {
			if _, subscribed := reg.chats[dmChatID]; subscribed {
				hub.mu.RUnlock()
				t.Fatalf("instance %d retained blocked DM in connection authority", i)
			}
			if _, subscribed := reg.chats[groupChatID]; !subscribed {
				hub.mu.RUnlock()
				t.Fatalf("instance %d lost non-DM subscription", i)
			}
		}
		hub.mu.RUnlock()
	}
}

func TestSocialBlockRevocationGatesOpenWebSocketActions(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, profileB := uuid.NewString(), uuid.NewString()
	chatID, messageID := uuid.NewString(), uuid.NewString()
	hub := newWSHub()
	hub.subscriptionChecker = newChatSubscriptionPolicy(
		&policyChatClient{
			chat:    &chatv1.Chat{Id: chatID, Type: chatv1.ChatType_CHAT_TYPE_DM},
			members: []*chatv1.ChatMember{{ProfileId: profileA}, {ProfileId: profileB}},
		},
		&policyUserClient{accounts: map[string]string{profileA: accountA, profileB: accountB}},
		&policySocialClient{blocked: make(map[string]bool)},
		hub,
	)
	deliveryPublisher := &recordingDeliveryAckPublisher{calls: make(chan struct{}, 1)}
	h := newServiceHandlerWithPresence(serviceName, staticTokenValidator{
		"a": {UserID: accountA, ProfileID: profileA},
		"b": {UserID: accountB, ProfileID: profileB},
	}, perProfileBootstrapLister{profileA: {chatID}, profileB: {chatID}}, hub, nil, "social-block-ws-test", nil, deliveryPublisher, readinessDeps{})
	wsServer := httptest.NewServer(h)
	t.Cleanup(wsServer.Close)
	a := dialACLTestConn(t, wsServer, "a", profileA)
	b := dialACLTestConn(t, wsServer, "b", profileB)
	for _, conn := range []*websocket.Conn{a, b} {
		if got := readACLEnvelope(t, conn); got.Op != "subscription_sync" {
			t.Fatalf("bootstrap = %+v", got)
		}
	}

	jsServer := startRealtimeJSTestServer(t)
	nc, err := nats.Connect(jsServer.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{Name: jsStreamSocialEvents, Subjects: []string{"social.>"}}); err != nil {
		t.Fatal(err)
	}
	sub, err := subscribeSocialEvents(js, hub, "social-block-ws-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })
	eventBytes, err := proto.Marshal(&eventsv1.SocialStreamEvent{
		EventId: uuid.NewString(), OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.SocialStreamEvent_UserBlocked{UserBlocked: &eventsv1.UserBlocked{
			BlockerAccountId: accountA, BlockedAccountId: accountB,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("social.user_blocked", eventBytes); err != nil {
		t.Fatal(err)
	}
	require.Eventually(t, func() bool {
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		_, registered := hub.byChat[chatID]
		return !registered
	}, 5*time.Second, 10*time.Millisecond)

	for _, tc := range []struct {
		op   string
		d    map[string]any
		code string
	}{
		{op: "typing_start", d: map[string]any{"chat_id": chatID}, code: "invalid_typing"},
		{op: "mark_read", d: map[string]any{"chat_id": chatID, "message_id": messageID}, code: "invalid_mark_read"},
		{op: "delivery_ack", d: map[string]any{"chat_id": chatID, "message_id": messageID, "sender_profile_id": profileB}, code: "invalid_delivery_ack"},
	} {
		if err := a.WriteJSON(map[string]any{"op": tc.op, "d": tc.d}); err != nil {
			t.Fatalf("%s write: %v", tc.op, err)
		}
		env := readACLEnvelope(t, a)
		if env.Op != "error" {
			t.Fatalf("%s after block = %+v", tc.op, env)
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(env.D, &body); err != nil || body.Code != tc.code {
			t.Fatalf("%s denial body=%s err=%v", tc.op, env.D, err)
		}
	}
	select {
	case <-deliveryPublisher.calls:
		t.Fatal("blocked delivery_ack published durable side effect")
	default:
	}
	_ = b.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	if _, _, err := b.ReadMessage(); err == nil {
		t.Fatal("blocked peer received chat-scoped side effect")
	}
}

func TestOpenDMWebSocketFailsClosedWhenSocialBlockEventIsLost(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, profileB := uuid.NewString(), uuid.NewString()
	chatID, messageID := uuid.NewString(), uuid.NewString()
	hub := newWSHub()
	social := &policySocialClient{blocked: make(map[string]bool)}
	hub.subscriptionChecker = newChatSubscriptionPolicy(
		&policyChatClient{
			chat: &chatv1.Chat{Id: chatID, Type: chatv1.ChatType_CHAT_TYPE_DM},
			members: []*chatv1.ChatMember{
				{ProfileId: profileA},
				{ProfileId: profileB},
			},
		},
		&policyUserClient{accounts: map[string]string{profileA: accountA, profileB: accountB}},
		social,
		hub,
	)
	deliveryPublisher := &recordingDeliveryAckPublisher{calls: make(chan struct{}, 1)}
	h := newServiceHandlerWithPresence(serviceName, staticTokenValidator{
		"a-desktop": {UserID: accountA, ProfileID: profileA},
		"a-mobile":  {UserID: accountA, ProfileID: profileA},
		"b":         {UserID: accountB, ProfileID: profileB},
	}, perProfileBootstrapLister{profileA: {chatID}, profileB: {chatID}}, hub, nil, "lost-social-block-event-test", nil, deliveryPublisher, readinessDeps{})
	wsServer := httptest.NewServer(h)
	t.Cleanup(wsServer.Close)
	aDesktop := dialACLTestConn(t, wsServer, "a-desktop", profileA)
	aMobile := dialACLTestConn(t, wsServer, "a-mobile", profileA)
	b := dialACLTestConn(t, wsServer, "b", profileB)
	for _, conn := range []*websocket.Conn{aDesktop, aMobile, b} {
		if got := readACLEnvelope(t, conn); got.Op != "subscription_sync" {
			t.Fatalf("bootstrap = %+v", got)
		}
	}

	// Model the committed Social block after PublishUserBlocked failed: the
	// authoritative IsBlocked read changes, but no revocation event reaches this
	// Realtime instance and every socket remains open with its old subscription.
	social.blocked[accountA+"/"+accountB] = true
	for _, tc := range []struct {
		op   string
		d    map[string]any
		code string
	}{
		{op: "typing_start", d: map[string]any{"chat_id": chatID}, code: "invalid_typing"},
		{op: "mark_read", d: map[string]any{"chat_id": chatID, "message_id": messageID}, code: "invalid_mark_read"},
		{op: "delivery_ack", d: map[string]any{"chat_id": chatID, "message_id": messageID, "sender_profile_id": profileB}, code: "invalid_delivery_ack"},
	} {
		require.NoError(t, aDesktop.WriteJSON(map[string]any{"op": tc.op, "d": tc.d}))
		env := readACLEnvelope(t, aDesktop)
		require.Equal(t, "error", env.Op, "%s must fail closed without a Social event", tc.op)
		var body struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(env.D, &body))
		require.Equal(t, tc.code, body.Code)
	}
	select {
	case <-deliveryPublisher.calls:
		t.Fatal("delivery_ack published after the authoritative Social block")
	default:
	}
	for name, conn := range map[string]*websocket.Conn{"same-profile tab": aMobile, "blocked peer": b} {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(250*time.Millisecond)))
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatalf("%s received a client side effect after the authoritative Social block", name)
		}
	}
}

func TestDMPolicyIndexIsBoundedByLocalSubscriptionLifecycle(t *testing.T) {
	accountA, accountB := uuid.NewString(), uuid.NewString()
	profileA, chatID := uuid.NewString(), uuid.NewString()
	hub := newWSHub()
	reg := hub.attachAccountConn("instance", "conn", accountA, profileA, 1)
	addAuthorizedTestDMChat(t, hub, reg, chatID, accountA, accountB)

	hub.removeChat(reg, chatID)
	hub.mu.RLock()
	rememberedAfterUnsubscribe := len(hub.dmPairByChat)
	pairsAfterUnsubscribe := len(hub.dmChatsByPair)
	hub.mu.RUnlock()
	require.Zero(t, rememberedAfterUnsubscribe, "last unsubscribe must release the chat/pair index")
	require.Zero(t, pairsAfterUnsubscribe, "unsubscribe must not leave pair index state")

	// An event for a pair with no local or in-flight DM work must be O(1) and
	// must not retain global deny history inside Realtime.
	hub.revokeAccountPairDMChats(accountA, accountB)
	hub.mu.RLock()
	rememberedAfterIrrelevantEvent := len(hub.dmPairByChat)
	pairsAfterIrrelevantEvent := len(hub.dmChatsByPair)
	hub.mu.RUnlock()
	require.Zero(t, rememberedAfterIrrelevantEvent)
	require.Zero(t, pairsAfterIrrelevantEvent, "irrelevant Social events must not grow a global pair set")

	firstTab := hub.attachAccountConn("instance", "first-tab", accountA, profileA, 1)
	secondTab := hub.attachAccountConn("instance", "second-tab", accountA, profileA, 1)
	addAuthorizedTestDMChat(t, hub, firstTab, chatID, accountA, accountB)
	addAuthorizedTestDMChat(t, hub, secondTab, chatID, accountA, accountB)
	hub.removeChat(firstTab, chatID)
	hub.mu.RLock()
	require.Len(t, hub.dmPairByChat, 1, "pair index must remain while another tab is subscribed")
	require.Len(t, hub.dmChatsByPair, 1)
	hub.mu.RUnlock()
	hub.unregisterConn(secondTab)
	hub.mu.RLock()
	require.Empty(t, hub.dmPairByChat, "last disconnect must release reverse DM index")
	require.Empty(t, hub.dmChatsByPair, "last disconnect must release pair index")
	hub.mu.RUnlock()
}
