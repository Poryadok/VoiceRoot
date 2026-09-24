package userevents

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/store"
)

func TestAccountDeletionDurableConfig(t *testing.T) {
	valid := &nats.ConsumerInfo{Stream: streamName, Name: accountDeletionDurable, Config: nats.ConsumerConfig{
		Durable: accountDeletionDurable, FilterSubject: subjectAccountDeleted,
		DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy,
	}}
	require.NoError(t, validateAccountDeletionDurable(valid))
	invalid := *valid
	invalid.Config.FilterSubject = "user.>"
	require.Error(t, validateAccountDeletionDurable(&invalid))
	invalid = *valid
	invalid.Config.DeliverSubject = "_INBOX.user"
	require.Error(t, validateAccountDeletionDurable(&invalid))
	invalid = *valid
	invalid.Config.AckPolicy = nats.AckNonePolicy
	require.Error(t, validateAccountDeletionDurable(&invalid))
	require.Error(t, validateAccountDeletionDurable(nil))
}

func TestAccountDeletionConsumer_PreprovisionedPullBind(t *testing.T) {
	if testing.Short() {
		t.Skip("embedded NATS integration")
	}
	s := startEmbeddedUserJSTestServer(t)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{"user.>"}})
	require.NoError(t, err)
	c := &AccountDeletionConsumer{js: js, profiles: &store.ProfileStore{}}
	require.Error(t, c.Start(), "missing durable must not be created")
	_, err = js.ConsumerInfo(streamName, accountDeletionDurable)
	require.Error(t, err)
	config := &nats.ConsumerConfig{Durable: accountDeletionDurable, FilterSubject: subjectAccountDeleted, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy}
	_, err = js.AddConsumer(streamName, config)
	require.NoError(t, err)
	require.NoError(t, c.Start())
	require.NotNil(t, c.sub)
	require.NoError(t, c.sub.Unsubscribe())
	require.NoError(t, js.DeleteConsumer(streamName, accountDeletionDurable))
	config.FilterSubject = "user.profile_deleted"
	_, err = js.AddConsumer(streamName, config)
	require.NoError(t, err)
	c.sub = nil
	require.Error(t, c.Start(), "neighboring subject must be denied")
}
