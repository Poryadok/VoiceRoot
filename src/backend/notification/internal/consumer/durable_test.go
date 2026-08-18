package consumer_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
)

func TestSharedDurable_IsStableAcrossReplicas(t *testing.T) {
	require.Equal(t, "notif_msg_v2", consumer.SharedDurable("message"))
	require.Equal(t, consumer.SharedDurable("message"), consumer.SharedDurable("message"))
	require.Equal(t, "notif_subscription", consumer.SharedDurable("subscription"))
}
