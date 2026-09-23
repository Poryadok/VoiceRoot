package userevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestJetStreamPublisher_UserContractPayloads(t *testing.T) {
	server := startEmbeddedUserJSTestServer(t)
	nc, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{
		Name: streamName,
		Subjects: []string{
			subjectProfileUpdated,
			subjectProfileVerified,
			subjectProfileSwitched,
			subjectGameDetected,
			subjectSettingsChanged,
		},
	})
	require.NoError(t, err)

	pub, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pub.Close()) })

	tests := []struct {
		name    string
		subject string
		publish func() error
		assert  func(*eventsv1.UserStreamEvent)
	}{
		{
			name:    "profile updated",
			subject: subjectProfileUpdated,
			publish: func() error {
				return pub.PublishProfileUpdated(context.Background(), "profile-1", []string{"display_name", "bio"})
			},
			assert: func(env *eventsv1.UserStreamEvent) {
				require.Equal(t, "profile-1", env.GetProfileUpdated().GetProfileId())
				require.Equal(t, []string{"display_name", "bio"}, env.GetProfileUpdated().GetChangedFields())
			},
		},
		{
			name:    "profile verified",
			subject: subjectProfileVerified,
			publish: func() error { return pub.PublishVerified(context.Background(), "profile-2", "organization") },
			assert: func(env *eventsv1.UserStreamEvent) {
				require.Equal(t, "profile-2", env.GetProfileVerified().GetProfileId())
				require.Equal(t, "organization", env.GetProfileVerified().GetVerificationType())
			},
		},
		{
			name:    "profile switched retains legacy and complete identities",
			subject: subjectProfileSwitched,
			publish: func() error {
				return pub.PublishProfileSwitched(context.Background(), "account-1", "profile-old", "profile-new")
			},
			assert: func(env *eventsv1.UserStreamEvent) {
				switched := env.GetProfileSwitched()
				require.Equal(t, "account-1", switched.GetAccountId())
				require.Equal(t, "profile-new", switched.GetProfileId(), "legacy profile_id aliases new_profile_id")
				require.Equal(t, "profile-old", switched.GetOldProfileId())
				require.Equal(t, "profile-new", switched.GetNewProfileId())
			},
		},
		{
			name:    "game detected",
			subject: subjectGameDetected,
			publish: func() error { return pub.PublishGameDetected(context.Background(), "profile-3", "Dota 2") },
			assert: func(env *eventsv1.UserStreamEvent) {
				require.Equal(t, "profile-3", env.GetGameDetected().GetProfileId())
				require.Equal(t, "Dota 2", env.GetGameDetected().GetGameName())
			},
		},
		{
			name:    "settings changed",
			subject: subjectSettingsChanged,
			publish: func() error {
				return pub.PublishSettingsChanged(context.Background(), "profile-4", []string{"show_read_receipts"}, `[{"key":"show_read_receipts","value":false}]`)
			},
			assert: func(env *eventsv1.UserStreamEvent) {
				require.Equal(t, "profile-4", env.GetSettingsChanged().GetProfileId())
				require.Equal(t, []string{"show_read_receipts"}, env.GetSettingsChanged().GetChangedKeys())
				require.Equal(t, `[{"key":"show_read_receipts","value":false}]`, env.GetSettingsChanged().GetChangedKeysJson())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := pub.nc.SubscribeSync(tt.subject)
			require.NoError(t, err)
			t.Cleanup(func() { _ = sub.Unsubscribe() })
			require.NoError(t, pub.nc.Flush())
			require.NoError(t, tt.publish())
			msg, err := sub.NextMsg(5 * time.Second)
			require.NoError(t, err)
			var env eventsv1.UserStreamEvent
			require.NoError(t, proto.Unmarshal(msg.Data, &env))
			require.NotEmpty(t, env.GetEventId())
			tt.assert(&env)
		})
	}
}
