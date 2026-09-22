package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticProviderEventHMACKeysExposesCurrentKeyForVerification(t *testing.T) {
	source := NewStaticProviderEventHMACKeys("paddle-webhook-secret-v1", []byte("test-key"))

	version, key, err := source.CurrentProviderEventHMACKey(context.Background(), "paddle")
	require.NoError(t, err)
	require.Equal(t, "paddle-webhook-secret-v1", version)
	require.Equal(t, []byte("test-key"), key)

	keys, err := source.ProviderEventHMACVerificationKeys(context.Background(), "paddle")
	require.NoError(t, err)
	require.Equal(t, []ProviderEventHMACKey{{Version: "paddle-webhook-secret-v1", Key: []byte("test-key")}}, keys)
}
