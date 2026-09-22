package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountDeletionConsumerConfig_DefaultOffCreatesNoCredentialRequirement(t *testing.T) {
	config, err := accountDeletionConsumerConfigFromEnv(func(string) string { return "" })
	require.NoError(t, err)
	require.False(t, config.Enabled)
	require.Empty(t, config.CredentialsFile)
}

func TestAccountDeletionConsumerConfig_EnabledWithoutDedicatedCredentialFailsClosed(t *testing.T) {
	_, err := accountDeletionConsumerConfigFromEnv(func(key string) string {
		if key == "USER_ACCOUNT_DELETE_CONSUMER_ENABLED" {
			return "true"
		}
		return ""
	})
	require.Error(t, err)
}

func TestAccountDeletionConsumerConfig_EnabledWithoutNATSURLFailsClosed(t *testing.T) {
	_, err := accountDeletionConsumerConfigFromEnv(func(key string) string {
		switch key {
		case "USER_ACCOUNT_DELETE_CONSUMER_ENABLED":
			return "true"
		case "USER_ACCOUNT_DELETE_NATS_CREDS_FILE":
			return "C:/run/secrets/user-account-delete.creds"
		default:
			return ""
		}
	})
	require.Error(t, err)
}

func TestAccountDeletionConsumerConfig_EnabledUsesOnlyDedicatedCredential(t *testing.T) {
	config, err := accountDeletionConsumerConfigFromEnv(func(key string) string {
		switch key {
		case "USER_ACCOUNT_DELETE_CONSUMER_ENABLED":
			return "true"
		case "USER_ACCOUNT_DELETE_NATS_CREDS_FILE":
			return "C:/run/secrets/user-account-delete.creds"
		case "USER_ACCOUNT_DELETE_NATS_URL":
			return "nats://broker:4222"
		default:
			return ""
		}
	})
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, "C:/run/secrets/user-account-delete.creds", config.CredentialsFile)
	require.Equal(t, "nats://broker:4222", config.NATSURL)
}

func TestAccountDeletionConsumerConfig_EnabledRejectsSharedNATSURL(t *testing.T) {
	_, err := accountDeletionConsumerConfigFromEnv(func(key string) string {
		switch key {
		case "USER_ACCOUNT_DELETE_CONSUMER_ENABLED":
			return "true"
		case "USER_ACCOUNT_DELETE_NATS_CREDS_FILE":
			return "/run/secrets/user-account-delete.creds"
		case "NATS_URL":
			return "nats://shared-broker:4222"
		default:
			return ""
		}
	})
	require.Error(t, err)
}
