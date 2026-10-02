package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSpaceLifecyclePrincipalRuntimeRequiresCompleteConfiguration(t *testing.T) {
	keys := []string{
		"SPACE_PRINCIPAL_JWKS_URL",
		"SPACE_PRINCIPAL_JWKS_CA_FILE",
		"MESSAGING_SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE",
		"MESSAGING_SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE",
		"MESSAGING_PRINCIPAL_REPLAY_REDIS_URL",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	runtime, err := loadSpaceLifecyclePrincipalRuntime(context.Background())
	require.NoError(t, err)
	require.Nil(t, runtime, "the optional protected listener stays disabled without configuration")

	t.Setenv("SPACE_PRINCIPAL_JWKS_URL", "https://space:8443/.well-known/principal-jwks.json")
	runtime, err = loadSpaceLifecyclePrincipalRuntime(context.Background())
	require.Error(t, err, "partial principal verification must fail startup")
	require.Nil(t, runtime)
}
