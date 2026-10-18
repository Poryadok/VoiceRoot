package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChatQuotaEntrypointActivation(t *testing.T) {
	for _, test := range []struct {
		env          map[string]string
		fail, active bool
	}{
		{map[string]string{}, false, false},
		{map[string]string{"NATS_URL": "nats://test"}, true, false},
		{map[string]string{"NATS_URL": "nats://test", "NOTIFICATION_REDIS_ADDR": "unavailable:6379"}, false, true},
		{map[string]string{"NATS_URL": "nats://test", "NOTIFICATION_REDIS_ADDR": "redis:6379", "NOTIFICATION_CHAT_PUSH_LIMIT": "0"}, true, false},
	} {
		q, err := newChatPushAdmission(func(k string) (string, bool) { v, ok := test.env[k]; return v, ok }, prometheus.NewRegistry())
		if test.fail {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, test.active, q != nil)
		if q != nil {
			require.NoError(t, q.Close())
		}
	}
}
