package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestLifecycleRuntimeRequiresAllTenEndpointsAndExplicitActivation(t *testing.T) {
	env := map[string]string{}
	config, enabled, err := loadLifecycleRuntimeConfig(func(k string) string { return env[k] })
	require.NoError(t, err)
	require.False(t, enabled)
	env["SPACE_LIFECYCLE_ENABLED"] = "true"
	_, _, err = loadLifecycleRuntimeConfig(func(k string) string { return env[k] })
	require.Error(t, err)
	env["SPACE_LIFECYCLE_TLS_CA_FILE"] = "ca.pem"
	env["SPACE_LIFECYCLE_CLIENT_CERT_FILE"] = "client.pem"
	env["SPACE_LIFECYCLE_CLIENT_KEY_FILE"] = "client.key"
	env["SPACE_ROLE_CLIENT_CERT_FILE"] = "role.pem"
	env["SPACE_ROLE_CLIENT_KEY_FILE"] = "role.key"
	env["SPACE_LIFECYCLE_MODE"] = "development"
	env["SPACE_LIFECYCLE_DEV_TOMBSTONE_KEY_FILE"] = "private.json"
	for _, name := range []string{"ROLE", "CHAT", "MESSAGING", "FILE", "VOICE", "MATCHMAKING", "SEARCH", "SUBSCRIPTION", "BOT", "NOTIFICATION"} {
		env["SPACE_LIFECYCLE_"+name+"_GRPC_ADDR"] = "owner:9443"
	}
	config, enabled, err = loadLifecycleRuntimeConfig(func(k string) string { return env[k] })
	require.NoError(t, err)
	require.True(t, enabled)
	require.Len(t, config.Owners, 10)
	for key := range env {
		if len(key) > 10 && key[len(key)-10:] == "_GRPC_ADDR" {
			saved := env[key]
			delete(env, key)
			_, _, err = loadLifecycleRuntimeConfig(func(k string) string { return env[k] })
			require.Error(t, err, key)
			env[key] = saved
		}
	}
	env["SPACE_LIFECYCLE_ENABLED"] = "perhaps"
	_, _, err = loadLifecycleRuntimeConfig(func(k string) string { return env[k] })
	require.Error(t, err)
}
