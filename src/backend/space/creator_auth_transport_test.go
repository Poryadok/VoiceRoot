package main

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatorAuthTransportRequiresSeparateCompleteMutualTLS(t *testing.T) {
	names := []string{"SPACE_AUTH_SESSION_FLOOR_GRPC_ADDR", "SPACE_AUTH_SESSION_FLOOR_TLS_CA_FILE", "SPACE_AUTH_SESSION_FLOOR_TLS_SERVER_NAME", "SPACE_AUTH_SESSION_FLOOR_CLIENT_CERT_FILE", "SPACE_AUTH_SESSION_FLOOR_CLIENT_KEY_FILE"}
	for _, name := range names {
		t.Setenv(name, "")
	}
	// Existing proof transport cannot silently enable or stand in for the floor.
	t.Setenv("AUTH_PRINCIPAL_GRPC_ADDR", "auth:9091")
	_, active, err := creatorAuthConfigFromEnv()
	require.NoError(t, err)
	require.False(t, active)
	cert, key := writeClientCertificate(t)
	values := []string{"auth:9092", cert, "auth", cert, key}
	for i, name := range names {
		t.Setenv(name, values[i])
	}
	config, active, err := creatorAuthConfigFromEnv()
	require.NoError(t, err)
	require.True(t, active)
	transport, err := config.tlsConfig()
	require.NoError(t, err)
	require.False(t, transport.InsecureSkipVerify)
	require.Equal(t, "auth", transport.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), transport.MinVersion)
	require.Len(t, transport.Certificates, 1)
	for i, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			_, active, err := creatorAuthConfigFromEnv()
			require.True(t, active)
			require.Error(t, err, "partial dedicated floor configuration must fail startup")
			t.Setenv(name, " "+values[i])
			_, _, err = creatorAuthConfigFromEnv()
			require.Error(t, err)
		})
	}
}
