package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestStoryInventoryRuntimeRequiresCompleteTrustGroup(t *testing.T) {
	for _, name := range storyInventorySettings {
		name := storyInventoryPrefix + name
		old, set := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if set {
				_ = os.Setenv(name, old)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	source, err := loadStoryInventorySource(nil)
	require.NoError(t, err)
	require.Nil(t, source)
	t.Setenv(storyInventoryPrefix+"GRPC_ADDR", "story:9093")
	source, err = loadStoryInventorySource(nil)
	require.Error(t, err)
	require.Nil(t, source)
	for _, name := range storyInventorySettings {
		t.Setenv(storyInventoryPrefix+name, "configured")
	}
	source, err = loadStoryInventorySource(nil)
	require.ErrorContains(t, err, "file principal runtime")
	require.Nil(t, source)
}

func TestMessagingInventoryRuntimeRequiresCompleteTrustGroup(t *testing.T) {
	for _, name := range storyInventorySettings {
		name := messagingInventoryPrefix + name
		old, set := os.LookupEnv(name)
		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() {
			if set {
				_ = os.Setenv(name, old)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	source, err := loadMessagingInventorySource(nil)
	require.NoError(t, err)
	require.Nil(t, source)
	t.Setenv(messagingInventoryPrefix+"GRPC_ADDR", "messaging:9093")
	source, err = loadMessagingInventorySource(nil)
	require.Error(t, err)
	require.Nil(t, source)
	for _, name := range storyInventorySettings {
		t.Setenv(messagingInventoryPrefix+name, "configured")
	}
	source, err = loadMessagingInventorySource(nil)
	require.ErrorContains(t, err, "file principal runtime")
	require.Nil(t, source)
}
