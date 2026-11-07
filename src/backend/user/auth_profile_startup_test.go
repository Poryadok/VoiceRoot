package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthProfileStartupRejectsPartialDedicatedSettings(t *testing.T) {
	for _, field := range []string{"GRPC_LISTEN", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLIENT_CA_FILE", "REPLAY_REDIS_ADDR", "REPLAY_REDIS_PASSWORD", "JWKS_CA_FILE", "JWKS_CLIENT_CERT_FILE", "JWKS_CLIENT_KEY_FILE"} {
		t.Run(field, func(t *testing.T) {
			clearSpaceCaptureTestConfig(t)
			t.Setenv("USER_AUTH_PROFILE_PRINCIPAL_"+field, "")
			require.ErrorContains(t, runUser(t.Context()), "user Auth profile principal config")
		})
	}
}
