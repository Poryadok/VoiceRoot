package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldDeliverPushToToken(t *testing.T) {
	require.True(t, shouldDeliverPushToToken("incoming_call", "voip_apns"))
	require.False(t, shouldDeliverPushToToken("incoming_call", "apns"))
	require.False(t, shouldDeliverPushToToken("incoming_call", "fcm"))
	require.False(t, shouldDeliverPushToToken("new_message", "voip_apns"))
	require.True(t, shouldDeliverPushToToken("new_message", "apns"))
	require.True(t, shouldDeliverPushToToken("match_found", "fcm"))
}
