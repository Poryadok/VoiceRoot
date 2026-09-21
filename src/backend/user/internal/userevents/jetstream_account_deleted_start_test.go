package userevents

import (
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/user/internal/store"
)

func TestAccountDeletionConsumer_StartFailsClosedWhenBrokerDeniesSubscription(t *testing.T) {
	consumer := &AccountDeletionConsumer{
		profiles: &store.ProfileStore{},
		bind: func() (*nats.Subscription, error) {
			return nil, errors.New("permission denied or stream missing")
		},
	}
	require.Error(t, consumer.Start())
	require.Nil(t, consumer.sub)
}
