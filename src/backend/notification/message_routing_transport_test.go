package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
	"os"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
	"voice/backend/pkg/notificationrouting"
)

func TestMessageRoutingRealNATSBoundaryAppliesPolicyAndCurrentAuthorization(t *testing.T) {
	policy, err := os.ReadFile("../../../deploy/nats/acl-intent.yaml")
	require.NoError(t, err)
	var intent struct {
		Services map[string]struct {
			Publish   []string `yaml:"publish"`
			Subscribe []string `yaml:"subscribe"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(policy, &intent))
	var users []*server.User
	for _, name := range []string{"notification", "realtime", "messaging"} {
		grant := intent.Services[name]
		users = append(users, &server.User{Username: name, Password: "disposable-test", Permissions: &server.Permissions{Publish: &server.SubjectPermission{Allow: grant.Publish}, Subscribe: &server.SubjectPermission{Allow: grant.Subscribe}}})
	}
	for _, tc := range []struct {
		name             string
		settings         delivery.SettingsSnapshot
		authorized, want bool
	}{
		{"allowed", delivery.SettingsSnapshot{}, true, true},
		{"muted", delivery.SettingsSnapshot{ChatMuted: true}, true, false},
		{"suppressed type", delivery.SettingsSnapshot{SuppressTypes: []delivery.NotificationType{delivery.TypeNewMessage}}, true, false},
		{"removed or blocked", delivery.SettingsSnapshot{}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, Users: users})
			require.NoError(t, e)
			go s.Start()
			require.True(t, s.ReadyForConnections(5*time.Second))
			defer s.Shutdown()
			responder, e := nats.Connect(s.ClientURL(), nats.UserInfo("notification", "disposable-test"))
			require.NoError(t, e)
			defer responder.Close()
			requester, e := nats.Connect(s.ClientURL(), nats.UserInfo("realtime", "disposable-test"))
			require.NoError(t, e)
			defer requester.Close()
			sender, recipient, chat := uuid.NewString(), uuid.NewString(), uuid.NewString()
			data, e := proto.Marshal(&eventsv1.MessageStreamEvent{EventId: uuid.NewString(), Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{ChatId: chat, MessageId: uuid.NewString(), SenderProfileId: sender}}})
			require.NoError(t, e)
			sub, e := bindMessageRoutingResponder(context.Background(), responder, routingMembers{{ProfileID: sender}, {ProfileID: recipient}}, &dispatch.MessagePusher{Presence: presence.OfflineChecker{}, Policy: routingPolicy{recipient: tc.settings}}, nil, routingAuthority(func(_ context.Context, r, _, _ string) (bool, error) { return r == sender || tc.authorized, nil }))
			require.NoError(t, e)
			defer sub.Unsubscribe()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, e := notificationrouting.Request(ctx, notificationrouting.Client{Connection: requester}, data)
			require.NoError(t, e)
			if tc.want {
				require.Equal(t, []notificationrouting.Target{{ProfileID: recipient, Type: "new_message"}}, got)
			} else {
				require.Empty(t, got)
			}
			permissionErrors := make(chan error, 2)
			attacker, e := nats.Connect(s.ClientURL(), nats.UserInfo("messaging", "disposable-test"), nats.ErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, e error) { permissionErrors <- e }))
			require.NoError(t, e)
			defer attacker.Close()
			require.NoError(t, attacker.Publish(notificationrouting.ReplyPrefix+uuid.NewString(), []byte("forged decision")))
			require.NoError(t, attacker.Flush())
			select {
			case err := <-permissionErrors:
				require.Error(t, err)
			case <-time.After(time.Second):
				t.Fatal("non-Notification role could publish routing response")
			}
		})
	}
}

func TestMessageRoutingResponderRejectsMissingSubscriptionGrant(t *testing.T) {
	s, e := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, Users: []*server.User{{Username: "notification", Password: "disposable-test", Permissions: &server.Permissions{Publish: &server.SubjectPermission{Allow: []string{notificationrouting.ReplyPrefix + ">"}}, Subscribe: &server.SubjectPermission{Allow: []string{"_INBOX.voice.notification.>"}}}}}})
	require.NoError(t, e)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	defer s.Shutdown()
	nc, e := nats.Connect(s.ClientURL(), nats.UserInfo("notification", "disposable-test"), nats.ErrorHandler(func(*nats.Conn, *nats.Subscription, error) {}))
	require.NoError(t, e)
	defer nc.Close()
	sub, e := bindMessageRoutingResponder(context.Background(), nc, nil, nil, nil, nil)
	require.Error(t, e)
	require.Nil(t, sub)
}
