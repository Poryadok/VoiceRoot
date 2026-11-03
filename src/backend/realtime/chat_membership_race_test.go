package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
)

type pausedMembershipChatClient struct {
	policyChatClient
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *pausedMembershipChatClient) GetChat(ctx context.Context, req *chatv1.GetChatRequest, opts ...grpc.CallOption) (*chatv1.GetChatResponse, error) {
	// Capture the authorized snapshot before the membership commit/event, then
	// delay its return to Realtime until the consumer has processed revocation.
	resp, err := c.policyChatClient.GetChat(ctx, req, opts...)
	c.once.Do(func() { close(c.started); <-c.release })
	return resp, err
}

func TestLateChatSubscriptionCannotResurrectRevokedMembership(t *testing.T) {
	for _, change := range []string{"removed", "left"} {
		for _, kind := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL, chatv1.ChatType_CHAT_TYPE_DM} {
			t.Run(change+"/"+kind.String(), func(t *testing.T) {
				hub := newWSHub()
				account, profile, peerAccount, peerProfile, chat := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
				reg := hub.attachAccountConn("race", "delayed", account, profile, 8)
				peer := hub.attachAccountConn("race", "peer", peerAccount, peerProfile, 8)
				hub.addChat(peer, chat)
				client := &pausedMembershipChatClient{policyChatClient: policyChatClient{
					chat:    &chatv1.Chat{Id: chat, Type: kind},
					members: []*chatv1.ChatMember{{ProfileId: profile}, {ProfileId: peerProfile}},
				}, started: make(chan struct{}), release: make(chan struct{})}
				var release sync.Once
				resume := func() { release.Do(func() { close(client.release) }) }
				t.Cleanup(resume)
				hub.subscriptionChecker = newChatSubscriptionPolicy(client, &policyUserClient{accounts: map[string]string{peerProfile: peerAccount}}, &policySocialClient{}, hub)
				jsServer := startRealtimeJSTestServer(t)
				nc, err := nats.Connect(jsServer.ClientURL())
				require.NoError(t, err)
				t.Cleanup(nc.Close)
				js, err := nc.JetStream()
				require.NoError(t, err)
				_, err = js.AddStream(&nats.StreamConfig{Name: jsStreamChatEvents, Subjects: []string{"chat.>"}})
				require.NoError(t, err)
				preprovisionRealtimeConsumer(t, js, jsStreamChatEvents, chatConsumerDurableName("membership-race"), "chat.>")
				sub, err := subscribeChatEvents(js, hub, "membership-race", nil)
				require.NoError(t, err)
				t.Cleanup(func() { _ = sub.Unsubscribe() })
				result := make(chan error, 1)
				go func() {
					result <- hub.authorizeAndAddChat(context.Background(), reg, account, profile, strings.ToUpper(chat))
				}()
				select {
				case <-client.started:
				case <-time.After(5 * time.Second):
					t.Fatal("GetChat did not start")
				}
				data, err := proto.Marshal(&eventsv1.ChatStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.ChatStreamEvent_ChatMemberChanged{ChatMemberChanged: &eventsv1.ChatMemberChanged{ChatId: chat, ProfileId: profile, Change: change}}})
				require.NoError(t, err)
				_, err = js.Publish("chat.member_changed", data)
				require.NoError(t, err)
				select {
				case env := <-reg.fanout:
					require.Equal(t, "chat_update", env.Op)
				case <-time.After(5 * time.Second):
					t.Fatal("membership consumer did not process event")
				}
				resume()
				select {
				case err = <-result:
				case <-time.After(5 * time.Second):
					t.Fatal("subscribe did not finish")
				}
				require.Error(t, err, "stale authorized snapshot must not register after revoke")
				require.False(t, hub.hasChat(reg, chat))
				require.False(t, hub.authorizeClientSideEffect(context.Background(), reg, chat))
				hub.broadcastToChat(chat, fanoutEnvelope{Op: "message_update", D: json.RawMessage(`{}`)}, nil, "")
				select {
				case env := <-reg.fanout:
					t.Fatalf("revoked profile received %s", env.Op)
				default:
				}
				select {
				case env := <-peer.fanout:
					require.Equal(t, "message_update", env.Op)
				default:
					t.Fatal("unaffected peer lost fanout")
				}
				// A fresh authoritative grant may subscribe; the fence is not a
				// permanent local membership deny cache.
				require.NoError(t, hub.authorizeAndAddChat(context.Background(), reg, account, profile, chat))
				require.True(t, hub.hasChat(reg, chat))
			})
		}
	}
}
